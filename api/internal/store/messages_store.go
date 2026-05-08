package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *Store) AppendMessage(ctx context.Context, m Message) error {
	return s.AppendMessageWithFiles(ctx, m, nil, nil)
}

// AppendMessageWithFiles persists a message together with file blobs and
// message↔file links. fileLinks[i] points to files[i]; the position field on
// fileLinks is set to its slice index when zero. Pass nil for either slice
// when the message has no real files (e.g. failed uploads — set
// m.Attachments to record the names instead).
func (s *Store) AppendMessageWithFiles(ctx context.Context, m Message, files []File, fileLinks []MessageFile) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin append message: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Files inherit the conversation's owner so per-user file listings and
	// authorization stay in sync without callers needing to thread user IDs.
	var conversationUserID string
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM conversations WHERE id = ?`, m.ConversationID).Scan(&conversationUserID); err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("lookup conversation owner: %w", err)
		}
	}

	userContent := m.UserContent
	llmContent := m.LLMContent
	if userContent == "" && m.Role == "user" {
		userContent = m.Content
	}
	if llmContent == "" {
		llmContent = m.Content
	}

	fallbackJSON := "[]"
	if len(fileLinks) == 0 && len(m.Attachments) > 0 {
		// Failed message path: no files in the DB, but we still want the
		// names to render after a reload.
		names := make([]string, 0, len(m.Attachments))
		for _, a := range m.Attachments {
			names = append(names, a.Name)
		}
		encoded, err := json.Marshal(names)
		if err != nil {
			return fmt.Errorf("marshal failed-message attachment names: %w", err)
		}
		fallbackJSON = string(encoded)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO messages(id, conversation_id, role, content, user_content, llm_content, attachments_json, thinking, model, has_error, elapsed_ms, input_tokens, output_tokens, reasoning_tokens, total_tokens, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, m.Role, m.Content, userContent, llmContent, fallbackJSON, m.Thinking, m.Model, m.HasError, m.ElapsedMs, m.InputTokens, m.OutputTokens, m.ReasoningTokens, m.TotalTokens, m.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	for _, f := range files {
		createdAt := f.CreatedAt
		if createdAt.IsZero() {
			createdAt = m.CreatedAt
		}
		size := f.SizeBytes
		if size == 0 {
			size = int64(len(f.Data))
		}
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO files(id, user_id, name, content_type, size_bytes, data, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
			f.ID, conversationUserID, f.Name, f.ContentType, size, f.Data, createdAt.UTC(),
		)
		if err != nil {
			return fmt.Errorf("insert file: %w", err)
		}
	}

	for idx, link := range fileLinks {
		position := idx
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO message_files(message_id, file_id, position) VALUES(?, ?, ?)`,
			m.ID, link.ID, position,
		)
		if err != nil {
			return fmt.Errorf("insert message_files link: %w", err)
		}
	}

	_, err = tx.ExecContext(
		ctx,
		`UPDATE conversations SET updated_at = ? WHERE id = ?`,
		time.Now().UTC(), m.ConversationID,
	)
	if err != nil {
		return fmt.Errorf("update conversation timestamp: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit append message: %w", err)
	}
	return nil
}

// LinkExistingFilesToMessage attaches existing file rows to a message. Used
// by the requeue path so the new message shares blobs with the original.
func (s *Store) LinkExistingFilesToMessage(ctx context.Context, messageID string, fileIDs []string) error {
	if len(fileIDs) == 0 {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin link files: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()
	for idx, fileID := range fileIDs {
		_, err = tx.ExecContext(
			ctx,
			`INSERT INTO message_files(message_id, file_id, position) VALUES(?, ?, ?)`,
			messageID, fileID, idx,
		)
		if err != nil {
			return fmt.Errorf("insert message_files link: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit link files: %w", err)
	}
	return nil
}

func (s *Store) SetMessageContent(ctx context.Context, messageID string, content string) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE messages SET content = ?, user_content = ?, llm_content = ? WHERE id = ?`,
		content, content, content, messageID,
	)
	if err != nil {
		return fmt.Errorf("update message content: %w", err)
	}
	return nil
}

func (s *Store) SetMessageElapsedMs(ctx context.Context, messageID string, elapsedMs int64) error {
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET elapsed_ms = ? WHERE id = ?`, elapsedMs, messageID)
	if err != nil {
		return fmt.Errorf("update message elapsed: %w", err)
	}
	return nil
}

func (s *Store) SetMessageTokenUsage(ctx context.Context, messageID string, inputTokens int, outputTokens int, reasoningTokens int, totalTokens int) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE messages SET input_tokens = ?, output_tokens = ?, reasoning_tokens = ?, total_tokens = ? WHERE id = ?`,
		inputTokens,
		outputTokens,
		reasoningTokens,
		totalTokens,
		messageID,
	)
	if err != nil {
		return fmt.Errorf("update message token usage: %w", err)
	}
	return nil
}

func (s *Store) ConversationTokenTotal(ctx context.Context, conversationID string) (int, error) {
	var total int
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(total_tokens), 0) FROM messages WHERE conversation_id = ?`, conversationID).Scan(&total)
	if err != nil {
		return 0, fmt.Errorf("get conversation token total: %w", err)
	}
	return total, nil
}

func (s *Store) SetMessageThinking(ctx context.Context, messageID string, thinking string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET thinking = ? WHERE id = ?`, thinking, messageID)
	if err != nil {
		return fmt.Errorf("update message thinking: %w", err)
	}
	return nil
}

func (s *Store) SetMessageModel(ctx context.Context, messageID string, model string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET model = ? WHERE id = ?`, model, messageID)
	if err != nil {
		return fmt.Errorf("update message model: %w", err)
	}
	return nil
}

func (s *Store) SetLatestUserMessageError(ctx context.Context, conversationID string, hasError bool) error {
	_, err := s.db.ExecContext(
		ctx,
		`UPDATE messages
			SET has_error = ?
			WHERE id = (
			SELECT id FROM messages
			WHERE conversation_id = ? AND role = 'user'
			ORDER BY created_at DESC
			LIMIT 1
		)`,
		hasError,
		conversationID,
	)
	if err != nil {
		return fmt.Errorf("set latest user message error: %w", err)
	}
	return nil
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
				model,
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

	messages := make([]Message, 0)
	messageIDs := make([]string, 0)
	fallbackNames := make(map[string][]string)

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
			&m.Model,
			&m.HasError,
			&m.ElapsedMs,
			&m.InputTokens,
			&m.OutputTokens,
			&m.ReasoningTokens,
			&m.TotalTokens,
			&m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}

		fallbackNames[m.ID] = decodeFallbackAttachmentNames(fallbackJSON)
		if m.Role == "user" && strings.TrimSpace(m.UserContent) != "" {
			m.Content = m.UserContent
		}
		if m.LLMContent == "" {
			m.LLMContent = m.Content
		}

		messageIDs = append(messageIDs, m.ID)
		messages = append(messages, m)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	attached, err := s.attachmentsForMessages(ctx, messageIDs)
	if err != nil {
		return nil, err
	}

	for i := range messages {
		if files, ok := attached[messages[i].ID]; ok && len(files) > 0 {
			messages[i].Attachments = files
			continue
		}
		if names := fallbackNames[messages[i].ID]; len(names) > 0 {
			messages[i].Attachments = namesToMessageFiles(names)
		}
	}

	return messages, nil
}

func (s *Store) GetMessage(ctx context.Context, conversationID string, messageID string) (Message, error) {
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
			model,
			has_error,
			elapsed_ms,
			input_tokens,
			output_tokens,
			reasoning_tokens,
			total_tokens,
			created_at
		FROM messages
		WHERE conversation_id = ? AND id = ?
	`, conversationID, messageID,
	)

	var message Message
	var fallbackJSON string

	if err := row.Scan(
		&message.ID,
		&message.ConversationID,
		&message.Role,
		&message.Content,
		&message.UserContent,
		&message.LLMContent,
		&fallbackJSON,
		&message.Thinking,
		&message.Model,
		&message.HasError,
		&message.ElapsedMs,
		&message.InputTokens,
		&message.OutputTokens,
		&message.ReasoningTokens,
		&message.TotalTokens,
		&message.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Message{}, fmt.Errorf("message not found")
		}
		return Message{}, fmt.Errorf("get message: %w", err)
	}

	attached, err := s.attachmentsForMessages(ctx, []string{message.ID})
	if err != nil {
		return Message{}, err
	}
	if files, ok := attached[message.ID]; ok && len(files) > 0 {
		message.Attachments = files
	} else if names := decodeFallbackAttachmentNames(fallbackJSON); len(names) > 0 {
		message.Attachments = namesToMessageFiles(names)
	}

	return message, nil
}
