package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

type Service struct {
	store    *store.Store
	llm      llm.Provider
	broker   *Broker
	tools    tools.Runtime
	logger   *slog.Logger
	timeout  time.Duration
	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
}

func NewService(st *store.Store, provider llm.Provider, toolRuntime tools.Runtime, logger *slog.Logger) *Service {
	return &Service{
		store:   st,
		llm:     provider,
		broker:  NewBroker(),
		tools:   toolRuntime,
		logger:  logger,
		timeout: 60 * time.Second,
		cancels: make(map[string]context.CancelFunc),
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
	now := time.Now().UTC()
	userMsg := store.Message{
		ID:             uuid.NewString(),
		ConversationID: conversationID,
		Role:           "user",
		Content:        content,
		CreatedAt:      now,
	}

	if err := s.store.AppendMessage(ctx, userMsg); err != nil {
		return store.Message{}, err
	}
	if err := s.ensureConversationTitle(ctx, conversationID, content); err != nil {
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
		out = append(out, llm.ChatMessage{
			Role:    m.Role,
			Content: m.Content,
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
