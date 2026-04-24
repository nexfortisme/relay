package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

type Service struct {
	store             *store.Store
	llm               llm.Provider
	broker            *Broker
	tools             tools.Runtime
	logger            *slog.Logger
	relayDir          string
	attachmentOptions attachments.PromptOptions
	timeout           time.Duration
	cancelMu          sync.Mutex
	cancels           map[string]context.CancelFunc
}

func NewService(
	st *store.Store,
	provider llm.Provider,
	toolRuntime tools.Runtime,
	logger *slog.Logger,
	relayDir string,
	attachmentOptions attachments.PromptOptions,
) *Service {
	return &Service{
		store:             st,
		llm:               provider,
		broker:            NewBroker(),
		tools:             toolRuntime,
		logger:            logger,
		relayDir:          relayDir,
		attachmentOptions: attachmentOptions,
		timeout:           60 * time.Second,
		cancels:           make(map[string]context.CancelFunc),
	}
}

func (s *Service) CreateConversation(ctx context.Context) (store.Conversation, error) {
	now := time.Now().UTC()
	return s.store.CreateConversation(ctx, uuid.NewString(), "New chat", now)
}

func (s *Service) ListConversations(ctx context.Context, includeArchived bool) ([]store.Conversation, error) {
	return s.store.ListConversations(ctx, includeArchived)
}

func (s *Service) GetMessages(ctx context.Context, conversationID string) ([]store.Message, error) {
	return s.store.GetMessages(ctx, conversationID)
}

func (s *Service) Subscribe(conversationID string) (<-chan Event, func()) {
	return s.broker.Subscribe(conversationID)
}

func (s *Service) AddUserMessageAndGenerate(ctx context.Context, conversationID string, content string) (store.Message, error) {
	return s.addUserMessageAndGenerate(ctx, conversationID, content, content, nil)
}

func (s *Service) AddUserMessageAndGenerateWithFiles(ctx context.Context, conversationID string, content string, files []attachments.UploadedFile) (store.Message, error) {
	now := time.Now().UTC()
	userMessageID := uuid.NewString()
	persistedNames, err := attachments.PersistUploadedFiles(s.relayDir, conversationID, userMessageID, files)
	if err != nil {
		return store.Message{}, err
	}
	prompt, err := attachments.BuildPrompt(content, files, s.attachmentOptions)
	if err != nil {
		return store.Message{}, err
	}
	return s.addUserMessageAndGenerate(ctx, conversationID, content, prompt, &store.Message{
		ID:             userMessageID,
		ConversationID: conversationID,
		Role:           "user",
		Content:        content,
		UserContent:    content,
		LLMContent:     prompt,
		Attachments:    persistedNames,
		CreatedAt:      now,
	})
}

func (s *Service) AddFailedUserMessage(
	ctx context.Context,
	conversationID string,
	content string,
	attachments []string,
) (store.Message, error) {
	now := time.Now().UTC()
	userMsg := store.Message{
		ID:             uuid.NewString(),
		ConversationID: conversationID,
		Role:           "user",
		Content:        content,
		UserContent:    content,
		LLMContent:     content,
		Attachments:    attachments,
		HasError:       true,
		CreatedAt:      now,
	}
	if err := s.store.AppendMessage(ctx, userMsg); err != nil {
		return store.Message{}, err
	}
	return userMsg, nil
}

func (s *Service) addUserMessageAndGenerate(
	ctx context.Context,
	conversationID string,
	displayContent string,
	llmContent string,
	preparedUserMessage *store.Message,
) (store.Message, error) {
	now := time.Now().UTC()
	userMsg := preparedUserMessage
	if userMsg == nil {
		userMsg = &store.Message{
			ID:             uuid.NewString(),
			ConversationID: conversationID,
			Role:           "user",
			Content:        displayContent,
			UserContent:    displayContent,
			LLMContent:     llmContent,
			CreatedAt:      now,
		}
	}

	if err := s.store.AppendMessage(ctx, *userMsg); err != nil {
		return store.Message{}, err
	}
	if err := s.ensureConversationTitle(ctx, conversationID, displayContent); err != nil {
		s.logger.Warn("failed to auto-title conversation", "conversation_id", conversationID, "error", err)
	}

	assistantMsg := store.Message{
		ID:             uuid.NewString(),
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        "",
		CreatedAt:      now.Add(time.Millisecond),
	}

	if err := s.store.AppendMessage(ctx, assistantMsg); err != nil {
		return store.Message{}, err
	}

	history, err := s.store.GetMessages(ctx, conversationID)
	if err != nil {
		return store.Message{}, err
	}

	go s.generateAssistant(conversationID, assistantMsg.ID, toLLMMessages(history))
	return assistantMsg, nil
}

func (s *Service) RenameConversation(ctx context.Context, conversationID string, title string) error {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" {
		return fmt.Errorf("title cannot be empty")
	}
	return s.store.UpdateConversationTitle(ctx, conversationID, trimmed)
}

func (s *Service) ArchiveConversation(ctx context.Context, conversationID string) error {
	return s.store.ArchiveConversation(ctx, conversationID)
}

func (s *Service) RestoreConversation(ctx context.Context, conversationID string) error {
	return s.store.RestoreConversation(ctx, conversationID)
}

func (s *Service) DeleteConversation(ctx context.Context, conversationID string) error {
	return s.store.DeleteConversation(ctx, conversationID)
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

func (s *Service) GetMessageAttachment(ctx context.Context, conversationID string, messageID string, attachmentIndex int) (string, string, error) {
	message, err := s.store.GetMessage(ctx, conversationID, messageID)
	if err != nil {
		return "", "", err
	}
	if attachmentIndex < 0 || attachmentIndex >= len(message.Attachments) {
		return "", "", fmt.Errorf("attachment not found")
	}

	path, storedName, err := attachments.ResolveUploadedFilePath(
		s.relayDir,
		conversationID,
		messageID,
		attachmentIndex,
		message.Attachments[attachmentIndex],
	)
	if err != nil {
		return "", "", err
	}
	if _, err := os.Stat(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", "", fmt.Errorf("attachment not found")
		}
		return "", "", err
	}
	return path, storedName, nil
}

func (s *Service) generateAssistant(conversationID string, assistantMessageID string, messages []llm.ChatMessage) {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	s.registerCancel(conversationID, cancel)
	defer s.unregisterCancel(conversationID)

	stream := s.llm.GenerateStream(ctx, messages, s.tools)
	var contentBuilder strings.Builder
	var thinkingBuilder strings.Builder

	for event := range stream {
		if event.Err != nil {
			if errorsIsContextDone(event.Err) {
				finalContent := strings.TrimSpace(contentBuilder.String())
				finalThinking := strings.TrimSpace(thinkingBuilder.String())
				if err := s.store.SetMessageContent(context.Background(), assistantMessageID, finalContent); err != nil {
					s.logger.Error("failed to persist stopped assistant message", "message_id", assistantMessageID, "error", err)
				}
				if err := s.store.SetMessageThinking(context.Background(), assistantMessageID, finalThinking); err != nil {
					s.logger.Error("failed to persist stopped assistant thinking", "message_id", assistantMessageID, "error", err)
				}
				s.broker.Publish(conversationID, Event{
					Type:      "stopped",
					MessageID: assistantMessageID,
				})
				return
			}
			s.logger.Error("generation failed", "conversation_id", conversationID, "error", event.Err)
			if err := s.store.SetLatestUserMessageError(context.Background(), conversationID, true); err != nil {
				s.logger.Error("failed to persist user message error state", "conversation_id", conversationID, "error", err)
			}
			s.broker.Publish(conversationID, Event{
				Type:      "error",
				MessageID: assistantMessageID,
				Error:     event.Err.Error(),
			})
			return
		}

		if event.Token != "" {
			contentBuilder.WriteString(event.Token)
			s.broker.Publish(conversationID, Event{
				Type:      "token",
				MessageID: assistantMessageID,
				Token:     event.Token,
			})
		}
		if event.Thinking != "" {
			thinkingBuilder.WriteString(event.Thinking)
			s.broker.Publish(conversationID, Event{
				Type:      "thinking",
				MessageID: assistantMessageID,
				Thinking:  event.Thinking,
			})
		}

		if event.Done {
			finalContent := strings.TrimSpace(contentBuilder.String())
			finalThinking := strings.TrimSpace(thinkingBuilder.String())
			if err := s.store.SetMessageContent(ctx, assistantMessageID, finalContent); err != nil {
				s.logger.Error("failed to persist final assistant message", "message_id", assistantMessageID, "error", err)
				s.broker.Publish(conversationID, Event{
					Type:      "error",
					MessageID: assistantMessageID,
					Error:     fmt.Sprintf("failed to persist message: %v", err),
				})
				return
			}
			if err := s.store.SetMessageThinking(ctx, assistantMessageID, finalThinking); err != nil {
				s.logger.Error("failed to persist assistant thinking", "message_id", assistantMessageID, "error", err)
			}
			s.broker.Publish(conversationID, Event{
				Type:      "done",
				MessageID: assistantMessageID,
				Thinking:  finalThinking,
			})
			return
		}
	}
}

func (s *Service) registerCancel(conversationID string, cancel context.CancelFunc) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	s.cancels[conversationID] = cancel
}

func (s *Service) unregisterCancel(conversationID string) {
	s.cancelMu.Lock()
	defer s.cancelMu.Unlock()
	delete(s.cancels, conversationID)
}

func errorsIsContextDone(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}

func toLLMMessages(messages []store.Message) []llm.ChatMessage {
	out := make([]llm.ChatMessage, 0, len(messages))
	for _, m := range messages {
		llmContent := m.LLMContent
		if llmContent == "" {
			llmContent = m.Content
		}
		out = append(out, llm.ChatMessage{
			Role:    m.Role,
			Content: llmContent,
		})
	}
	return out
}

func (s *Service) ensureConversationTitle(ctx context.Context, conversationID string, firstMessage string) error {
	conversation, err := s.store.GetConversation(ctx, conversationID)
	if err != nil {
		return err
	}
	if conversation.Title != "New chat" {
		return nil
	}

	autoTitle := deriveTitle(firstMessage)
	return s.store.UpdateConversationTitle(ctx, conversationID, autoTitle)
}

func deriveTitle(message string) string {
	trimmed := strings.TrimSpace(strings.ReplaceAll(message, "\n", " "))
	if trimmed == "" {
		return "New chat"
	}
	if len(trimmed) <= 40 {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:40]) + "..."
}
