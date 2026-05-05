package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
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

	// Creating the conversation
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO conversations(id, user_id, title, created_at, updated_at) VALUES(?, ?, ?, ?, ?)`,
		id, userID, title, now.UTC(), now.UTC(),
	)
	if err != nil {
		return Conversation{}, fmt.Errorf("Error Creating Conversation: %w", err)
	}

	// No errors, return an object mathing the created conversation
	return Conversation{ID: id, Title: title, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (s *Store) ListConversations(ctx context.Context, userID string, includeArchived bool) ([]Conversation, error) {

	// Query to fetch the conversations
	query := `
		SELECT id, title, archived_at, created_at, updated_at
		FROM conversations
		WHERE user_id = ?
	`
	if !includeArchived {
		query += "\nAND archived_at IS NULL"
	}
	query += "\nORDER BY updated_at DESC"

	rows, err := s.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("Error Fetching Conversations: %w", err)
	}
	defer rows.Close()

	// Conversations list for returning
	conversations := make([]Conversation, 0)

	// Iterating over the rows and appending the conversations to the list
	for rows.Next() {
		var c Conversation
		var archivedAt sql.NullTime

		// Scanning the rows and parsing it into the conversation object
		if err := rows.Scan(&c.ID, &c.Title, &archivedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("Error Parsing Conversation: %w", err)
		}

		// Boolean Check to see if archivedAt is valid
		c.Archived = archivedAt.Valid

		// Appending the conversation to the list
		conversations = append(conversations, c)
	}

	// Return the conversations list and any errors
	return conversations, rows.Err()
}

// GetConversationOwner returns the user that owns the conversation. Used by
// service-layer ownership checks before performing any conversation-scoped
// action so that one user cannot read another's data via a guessed ID.
func (s *Store) GetConversationOwner(ctx context.Context, conversationID string) (string, error) {
	var userID string

	// Query to fetch the user that owns the conversation
	err := s.db.QueryRowContext(ctx, `SELECT user_id FROM conversations WHERE id = ?`, conversationID).Scan(&userID)

	// Boolean Check to see if the conversation does not exist
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("Conversation Not Found")
	}
	if err != nil {
		return "", fmt.Errorf("Error Fetching Conversation Owner: %w", err)
	}

	// Return the user ID and any errors
	return userID, nil
}

func (s *Store) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {

	// Query to fetch the conversation
	row := s.db.QueryRowContext(
		ctx, 
		`
			SELECT id, title, archived_at, created_at, updated_at
			FROM conversations
			WHERE id = ?
		`, conversationID,
	)

	var c Conversation
	var archivedAt sql.NullTime

	// Scanning the rows and parsing it into the conversation object
	if err := row.Scan(&c.ID, &c.Title, &archivedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Conversation{}, fmt.Errorf("Conversation Not Found")
		}
		return Conversation{}, fmt.Errorf("Error Fetching Conversation: %w", err)
	}

	// Boolean Check to see if the conversation is archived
	c.Archived = archivedAt.Valid

	// Return the conversation object and any errors
	return c, nil
}

func (s *Store) GetMessages(ctx context.Context, conversationID string) ([]Message, error) {
	rows, err := s.db.QueryContext(
		ctx, `
			SELECT 
				id, 
				conversation_id, 
				role, content, 
				user_content, 
				llm_content, 
				attachments_json, 
				thinking, 
				has_error, 
				elapsed_ms, 
				input_tokens, 
				output_tokens, 
				reasoning_tokens, 
				total_tokens, 
				created_at
			FROM messages
			WHERE conversation_id = ?
			ORDER BY created_at ASC
		`, conversationID,
	)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	// Messages list for returning
	messages := make([]Message, 0)
	messageIDs := make([]string, 0)
	fallbackNames := make(map[string][]string)

	// Iterating over the rows, parsing and appending the messages to the list
	for rows.Next() {
		var m Message
		var fallbackJSON string
		if err := rows.Scan(
			&m.ID, 
			&m.ConversationID, 
			&m.Role, 
			&m.Content, 
			&m.UserContent, 
			&m.LLMContent, 
			&fallbackJSON, 
			&m.Thinking, 
			&m.HasError, 
			&m.ElapsedMs, 
			&m.InputTokens, 
			&m.OutputTokens, 
			&m.ReasoningTokens, 
			&m.TotalTokens, 
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("Error Parsing Message: %w", err)
		}

		// Decoding the fallback attachment names
		fallbackNames[m.ID] = decodeFallbackAttachmentNames(fallbackJSON)
		if m.Role == "user" && strings.TrimSpace(m.UserContent) != "" {
			m.Content = m.UserContent
		}
		if m.LLMContent == "" {
			m.LLMContent = m.Content
		}

		// Appending the message ID to the list
		messageIDs = append(messageIDs, m.ID)

		// Appending the message to the list
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Fetching the attachments for the messages
	attached, err := s.attachmentsForMessages(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	// Iterating over the messages, fetching the attachments and appending them to the list
	for i := range messages {
		if files, ok := attached[messages[i].ID]; ok && len(files) > 0 {
			messages[i].Attachments = files
			continue
		}
		if names := fallbackNames[messages[i].ID]; len(names) > 0 {
			messages[i].Attachments = namesToMessageFiles(names)
		}
	}

	// Return the messages list and any errors
	return messages, nil
}

// GetMessage fetches a message by conversation ID and message ID
// Used by the requeue path so the new message shares blobs with the original.
func (s *Store) GetMessage(ctx context.Context, conversationID string, messageID string) (Message, error) {

	// Query to fetch the message
	row := s.db.QueryRowContext(ctx, `
		SELECT 
			id, 
			conversation_id, 
			role, 
			content, 
			user_content, 
			llm_content, 
			attachments_json, 
			thinking, 
			has_error, 
			elapsed_ms, 
			input_tokens, 
			output_tokens, 
			reasoning_tokens, 
			total_tokens, 
			created_at,
		FROM messages
		WHERE conversation_id = ? AND id = ?
	`, conversationID, messageID,
	)

	// Message object for returning
	var message Message

	// Fallback JSON string for attachments
	var fallbackJSON string

	// Scanning the rows and parsing it into the message object
	if err := row.Scan(
		&message.ID,
		&message.ConversationID,
		&message.Role,
		&message.Content,
		&message.UserContent,
		&message.LLMContent,
		&fallbackJSON,
		&message.Thinking,
		&message.HasError,
		&message.ElapsedMs,
		&message.InputTokens,
		&message.OutputTokens,
		&message.ReasoningTokens,
		&message.TotalTokens,
		&message.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Message{}, fmt.Errorf("Message Not Found")
		}
		return Message{}, fmt.Errorf("Error Fetching Message: %w", err)
	}

	// Fetching the attachments for the message
	attached, err := s.attachmentsForMessages(ctx, []string{message.ID})
	if err != nil {
		return Message{}, err
	}
	// Iterating over the message, fetching the attachments and appending them to the list
	if files, ok := attached[message.ID]; ok && len(files) > 0 {
		message.Attachments = files
	} else if names := decodeFallbackAttachmentNames(fallbackJSON); len(names) > 0 {
		message.Attachments = namesToMessageFiles(names)
	}

	// Return the message object and any errors
	return message, nil
}