package chat

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nexfortisme/relay/internal/store"
)

func (s *Service) CreateConversation(ctx context.Context, userID string) (store.Conversation, error) {
	now := time.Now().UTC()
	return s.store.CreateConversation(ctx, uuid.NewString(), userID, "New chat", now)
}

func (s *Service) ListConversations(ctx context.Context, userID string, includeArchived bool) ([]store.Conversation, error) {
	return s.store.ListConversations(ctx, userID, includeArchived)
}

func (s *Service) RenameConversation(ctx context.Context, userID, conversationID string, title string) error {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return err
	}
	trimmed := clampConversationTitle(title)
	if trimmed == "" {
		return fmt.Errorf("title cannot be empty")
	}
	return s.store.UpdateConversationTitle(ctx, conversationID, trimmed)
}

func (s *Service) ArchiveConversation(ctx context.Context, userID, conversationID string) error {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return err
	}
	return s.store.ArchiveConversation(ctx, conversationID)
}

func (s *Service) RestoreConversation(ctx context.Context, userID, conversationID string) error {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return err
	}
	return s.store.RestoreConversation(ctx, conversationID)
}

func (s *Service) SetConversationFavorite(ctx context.Context, userID, conversationID string, favorite bool) error {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return err
	}
	return s.store.SetConversationFavorite(ctx, conversationID, favorite)
}

func (s *Service) DeleteConversation(ctx context.Context, userID, conversationID string) error {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return err
	}
	return s.store.DeleteConversation(ctx, conversationID)
}

// authorizeConversation returns nil when the conversation exists and is owned
// by userID. Returns ErrForbidden otherwise so callers can render a 404/403.
func (s *Service) authorizeConversation(ctx context.Context, userID, conversationID string) error {
	owner, err := s.store.GetConversationOwner(ctx, conversationID)
	if err != nil {
		return err
	}
	if owner != userID {
		return ErrForbidden
	}
	return nil
}

// AuthorizeConversation exposes the same ownership check used by the chat
// service to HTTP handlers so they can guard the WebSocket stream and other
// non-DB-touching paths (stop generation) without bypassing auth.
func (s *Service) AuthorizeConversation(ctx context.Context, userID, conversationID string) error {
	return s.authorizeConversation(ctx, userID, conversationID)
}

// SetConversationNotebookID links a conversation to a notebook.
func (s *Service) SetConversationNotebookID(ctx context.Context, conversationID, notebookID string) error {
	return s.store.SetConversationNotebookID(ctx, conversationID, notebookID)
}

// ListNotebookConversations returns conversations linked to a notebook.
func (s *Service) ListNotebookConversations(ctx context.Context, userID, notebookID string, includeArchived bool) ([]store.Conversation, error) {
	return s.store.ListNotebookConversations(ctx, userID, notebookID, includeArchived)
}
