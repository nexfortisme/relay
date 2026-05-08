package chat

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/notebooks"
	"github.com/nexfortisme/relay/internal/prompts"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

type Service struct {
	store             *store.Store
	defaultLLMURL     string
	defaultLLMModel   string
	broker            *Broker
	tools             tools.Runtime
	logger            *slog.Logger
	attachmentOptions attachments.PromptOptions
	responseTimeout   time.Duration
	maxTokenCount     int
	cancelMu          sync.Mutex
	cancels           map[string]context.CancelFunc
	notebookSvc       *notebooks.Service
	notebookRegistry  *notebooks.Registry
}

const maxConversationTitleLength = 40

var ErrTokenCapReached = errors.New("conversation token cap reached")
var ErrForbidden = errors.New("forbidden")

// citeSourcesDirective is injected as a system message on every assistant
// generation. It is intentionally scoped to "when your answer draws on" so
// that plain conversational turns aren't forced to fabricate citations.
// External sources are required to be markdown links because the frontend
// renders them through marked + DOMPurify (see web/src/components/MessageList.vue).
var citeSourcesDirective = prompts.MustLoad(prompts.CiteSources)

func NewService(
	st *store.Store,
	defaultLLMURL string,
	defaultLLMModel string,
	toolRuntime tools.Runtime,
	logger *slog.Logger,
	attachmentOptions attachments.PromptOptions,
	maxTokenCount int,
) *Service {
	return &Service{
		store:             st,
		defaultLLMURL:     defaultLLMURL,
		defaultLLMModel:   defaultLLMModel,
		broker:            NewBroker(),
		tools:             toolRuntime,
		logger:            logger,
		attachmentOptions: attachmentOptions,
		responseTimeout:   5 * time.Minute,
		maxTokenCount:     maxTokenCount,
		cancels:           make(map[string]context.CancelFunc),
	}
}

func (s *Service) settingOrDefault(ctx context.Context, userID, key, defaultVal string) string {
	val, ok, err := s.store.GetSetting(ctx, userID, key)
	if err != nil || !ok || val == "" {
		return defaultVal
	}
	return val
}

type RuntimeSettings struct {
	LLMURL         string
	LLMModel       string
	LLMAPIKey      string
	SystemPrompt   string
	NotebookID     string
	NotebookPrompt string // non-empty: replaces SystemPrompt for notebook chats
	NotebookSkill  string // injected between system prompt and RAG context
}

func (s *Service) LoadRuntimeSettings(ctx context.Context, userID string) RuntimeSettings {
	return RuntimeSettings{
		LLMURL:       s.settingOrDefault(ctx, userID, "llm_url", s.defaultLLMURL),
		LLMModel:     s.settingOrDefault(ctx, userID, "llm_model", s.defaultLLMModel),
		LLMAPIKey:    s.settingOrDefault(ctx, userID, "llm_api_key", ""),
		SystemPrompt: s.settingOrDefault(ctx, userID, "system_prompt", ""),
	}
}

// LoadRuntimeSettingsForConversation extends LoadRuntimeSettings with notebook context.
func (s *Service) LoadRuntimeSettingsForConversation(ctx context.Context, userID, conversationID string) RuntimeSettings {
	settings := s.LoadRuntimeSettings(ctx, userID)
	if s.notebookSvc == nil {
		return settings
	}
	notebookID, err := s.store.GetConversationNotebookID(ctx, conversationID)
	if err != nil || notebookID == "" {
		return settings
	}
	nb, err := s.notebookSvc.GetNotebook(ctx, userID, notebookID)
	if err != nil {
		return settings
	}
	settings.NotebookID = notebookID
	settings.NotebookPrompt = nb.SystemPrompt
	settings.NotebookSkill = nb.SkillPrompt
	return settings
}

// WithNotebooks injects the notebook service and registry after construction
// to avoid a circular dependency between chat and notebooks packages.
func (s *Service) WithNotebooks(svc *notebooks.Service, reg *notebooks.Registry) {
	s.notebookSvc = svc
	s.notebookRegistry = reg
}

// DefaultSettings returns the values a freshly-registered user gets seeded
// with, so their settings are independent of the deployment defaults from
// that point on.
func (s *Service) DefaultSettings() map[string]string {
	return map[string]string{
		"llm_url":       s.defaultLLMURL,
		"llm_model":     s.defaultLLMModel,
		"llm_api_key":   "",
		"system_prompt": "",
	}
}

func (s *Service) GetSettings(ctx context.Context, userID string) (map[string]string, error) {
	dbSettings, err := s.store.GetAllSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	result := s.DefaultSettings()
	for k, v := range dbSettings {
		result[k] = v
	}
	return result, nil
}

func (s *Service) UpdateSettings(ctx context.Context, userID string, settings map[string]string) error {
	allowed := map[string]bool{"llm_url": true, "llm_model": true, "llm_api_key": true, "system_prompt": true}
	for k, v := range settings {
		if !allowed[k] {
			continue
		}
		if err := s.store.UpsertSetting(ctx, userID, k, v); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) Subscribe(conversationID string) (<-chan Event, func()) {
	return s.broker.Subscribe(conversationID)
}

func (s *Service) StopGeneration(conversationID string) bool {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	cancel, ok := s.cancels[conversationID]
	if !ok {
		return false
	}
	cancel()
	return true
}

// toLLMMessages converts stored history into the wire format expected by the
// LLM, optionally prepending one or more system prompts. Empty prompts are
// skipped so callers can safely pass stored settings that may be unset.
func toLLMMessages(messages []store.Message, systemPrompts ...string) []llm.ChatMessage {
	out := make([]llm.ChatMessage, 0, len(messages)+len(systemPrompts))
	for _, prompt := range systemPrompts {
		if strings.TrimSpace(prompt) == "" {
			continue
		}
		out = append(out, llm.ChatMessage{Role: "system", Content: prompt})
	}
	for _, m := range messages {
		if m.Role == "system" {
			continue
		}
		llmContent := m.LLMContent
		if llmContent == "" {
			llmContent = m.Content
		}
		out = append(out, llm.ChatMessage{
			Role:    m.Role,
			Content: llm.ParseContent(llmContent),
		})
	}
	return out
}

// activeSystemPrompt returns the effective system prompt for a request,
// using the notebook prompt when set (replaces the global prompt).
func (s *Service) activeSystemPrompt(settings RuntimeSettings) string {
	if strings.TrimSpace(settings.NotebookPrompt) != "" {
		return settings.NotebookPrompt
	}
	return settings.SystemPrompt
}

// notebookToolRuntime returns a composite runtime extended with notebook tools
// when the conversation is linked to a notebook.
func (s *Service) notebookToolRuntime(userID, notebookID string) tools.Runtime {
	if notebookID == "" || s.notebookSvc == nil || s.notebookRegistry == nil {
		return s.tools
	}
	nbRuntime := notebooks.NewToolRuntime(s.notebookSvc, s.notebookRegistry, userID, notebookID)
	// Notebook tools first so the LLM prefers uploaded-document search over web search.
	return tools.NewCompositeRuntime(nbRuntime, s.tools)
}

func errorsIsContextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
