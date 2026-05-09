package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

func (s *Store) CreateConversation(
	ctx context.Context,
	id string,
	userID string,
	title string,
	now time.Time,
) (Conversation, error) {
	if title == "" {
		title = "New chat"
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO conversations(id, user_id, title, created_at, updated_at) VALUES(?, ?, ?, ?, ?)`,
		id, userID, title, now.UTC(), now.UTC(),
	)
	if err != nil {
		return Conversation{}, fmt.Errorf("create conversation: %w", err)
	}
	return Conversation{ID: id, Title: title, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (s *Store) ListConversations(ctx context.Context, userID string, includeArchived bool) ([]Conversation, error) {
	query := `
		SELECT id, title, archived_at, favorited_at, notebook_id, created_at, updated_at
		FROM conversations
		WHERE user_id = ?
	`
	if !includeArchived {
		query += "\nAND archived_at IS NULL"
	}
	query += "\nORDER BY updated_at DESC"

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]Conversation, 0)
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		conversations = append(conversations, c)
	}

	return conversations, rows.Err()
}

// ListNotebookConversations returns conversations linked to a notebook,
// newest first.
func (s *Store) ListNotebookConversations(ctx context.Context, userID, notebookID string, includeArchived bool) ([]Conversation, error) {
	query := `
		SELECT id, title, archived_at, favorited_at, notebook_id, created_at, updated_at
		FROM conversations
		WHERE user_id = ? AND notebook_id = ?
	`
	if !includeArchived {
		query += "\nAND archived_at IS NULL"
	}
	query += "\nORDER BY updated_at DESC"

	rows, err := s.db.QueryContext(ctx, query, userID, notebookID)
	if err != nil {
		return nil, fmt.Errorf("list notebook conversations: %w", err)
	}
	defer rows.Close()

	out := make([]Conversation, 0)
	for rows.Next() {
		c, err := scanConversation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetConversationOwner returns the user that owns the conversation. Used by
// service-layer ownership checks before performing any conversation-scoped
// action so that one user cannot read another's data via a guessed ID.
func (s *Store) GetConversationOwner(ctx context.Context, conversationID string) (string, error) {
	var userID string
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM conversations WHERE id = ?`, conversationID).Scan(&userID)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("conversation not found")
	}
	if err != nil {
		return "", fmt.Errorf("get conversation owner: %w", err)
	}
	return userID, nil
}

func (s *Store) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, title, archived_at, favorited_at, notebook_id, created_at, updated_at
		FROM conversations
		WHERE id = ?
	`, conversationID)

	c, err := scanConversation(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Conversation{}, fmt.Errorf("conversation not found")
		}
		return Conversation{}, fmt.Errorf("get conversation: %w", err)
	}
	return c, nil
}

func (s *Store) UpdateConversationTitle(ctx context.Context, conversationID string, title string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`,
		title,
		time.Now().UTC(),
		conversationID,
	)
	if err != nil {
		return fmt.Errorf("update conversation title: %w", err)
	}
	return nil
}

func (s *Store) ArchiveConversation(ctx context.Context, conversationID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE conversations SET archived_at = ?, updated_at = ? WHERE id = ?`, time.Now().UTC(), time.Now().UTC(), conversationID)
	if err != nil {
		return fmt.Errorf("archive conversation: %w", err)
	}
	return nil
}

func (s *Store) RestoreConversation(ctx context.Context, conversationID string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE conversations SET archived_at = NULL, updated_at = ? WHERE id = ?`, time.Now().UTC(), conversationID)
	if err != nil {
		return fmt.Errorf("restore conversation: %w", err)
	}
	return nil
}

func (s *Store) SetConversationFavorite(ctx context.Context, conversationID string, favorite bool) error {
	now := time.Now().UTC()
	var favoritedAt any
	if favorite {
		favoritedAt = now
	}
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE conversations SET favorited_at = ?, updated_at = ? WHERE id = ?`,
		favoritedAt,
		now,
		conversationID,
	)
	if err != nil {
		return fmt.Errorf("favorite conversation: %w", err)
	}
	return nil
}

func (s *Store) DeleteConversation(ctx context.Context, conversationID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}

type conversationScanner interface {
	Scan(dest ...any) error
}

func scanConversation(scanner conversationScanner) (Conversation, error) {
	var c Conversation
	var archivedAt sql.NullTime
	var favoritedAt sql.NullTime
	if err := scanner.Scan(
		&c.ID,
		&c.Title,
		&archivedAt,
		&favoritedAt,
		&c.NotebookID,
		&c.CreatedAt,
		&c.UpdatedAt,
	); err != nil {
		return Conversation{}, err
	}
	c.Archived = archivedAt.Valid
	c.Favorite = favoritedAt.Valid
	return c, nil
}
