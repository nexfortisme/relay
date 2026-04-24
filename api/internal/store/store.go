package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Archived  bool      `json:"archived"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Message struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversationId"`
	Role           string    `json:"role"`
	Content        string    `json:"content"`
	UserContent    string    `json:"userContent,omitempty"`
	LLMContent     string    `json:"llmContent,omitempty"`
	Attachments    []string  `json:"attachments,omitempty"`
	Thinking       string    `json:"thinking,omitempty"`
	HasError       bool      `json:"hasError,omitempty"`
	CreatedAt      time.Time `json:"createdAt"`
}

type Store struct {
	db *sql.DB
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", sqliteDSNWithPragmas(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}

	// Allow concurrent requests to use separate SQLite connections.
	// Combined with WAL + busy_timeout pragmas this helps avoid SQLITE_BUSY
	// during short-lived write contention.
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(12)
	db.SetConnMaxLifetime(0)
	db.SetConnMaxIdleTime(0)

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func sqliteDSNWithPragmas(path string) string {
	separator := "?"
	if strings.Contains(path, "?") {
		separator = "&"
	}

	return path + separator +
		"_pragma=journal_mode(WAL)" +
		"&_pragma=busy_timeout(5000)" +
		"&_pragma=foreign_keys(ON)" +
		"&_pragma=synchronous(NORMAL)"
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	const schema = `
CREATE TABLE IF NOT EXISTS conversations (
  id TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT 'New chat',
  archived_at DATETIME,
  created_at DATETIME NOT NULL,
  updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS messages (
  id TEXT PRIMARY KEY,
  conversation_id TEXT NOT NULL,
  role TEXT NOT NULL,
  content TEXT NOT NULL DEFAULT '',
  user_content TEXT NOT NULL DEFAULT '',
  llm_content TEXT NOT NULL DEFAULT '',
  attachments_json TEXT NOT NULL DEFAULT '[]',
  thinking TEXT NOT NULL DEFAULT '',
  has_error INTEGER NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL,
  FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation_created
  ON messages(conversation_id, created_at);
`
	_, err := s.db.ExecContext(ctx, schema)
	if err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN archived_at DATETIME`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN thinking TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN user_content TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN llm_content TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN attachments_json TEXT NOT NULL DEFAULT '[]'`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN has_error INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.ExecContext(ctx, `UPDATE messages SET user_content = content WHERE user_content = '' AND role = 'user'`)
	_, _ = s.db.ExecContext(ctx, `UPDATE messages SET llm_content = content WHERE llm_content = ''`)
	return nil
}

func (s *Store) CreateConversation(ctx context.Context, id string, title string, now time.Time) (Conversation, error) {
	if title == "" {
		title = "New chat"
	}

	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO conversations(id, title, created_at, updated_at) VALUES(?, ?, ?, ?)`,
		id, title, now.UTC(), now.UTC(),
	)
	if err != nil {
		return Conversation{}, fmt.Errorf("insert conversation: %w", err)
	}

	return Conversation{ID: id, Title: title, CreatedAt: now.UTC(), UpdatedAt: now.UTC()}, nil
}

func (s *Store) ListConversations(ctx context.Context, includeArchived bool) ([]Conversation, error) {
	query := `
SELECT id, title, archived_at, created_at, updated_at
FROM conversations`
	if !includeArchived {
		query += "\nWHERE archived_at IS NULL"
	}
	query += "\nORDER BY updated_at DESC"

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list conversations: %w", err)
	}
	defer rows.Close()

	conversations := make([]Conversation, 0)
	for rows.Next() {
		var c Conversation
		var archivedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.Title, &archivedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		c.Archived = archivedAt.Valid
		conversations = append(conversations, c)
	}
	return conversations, rows.Err()
}

func (s *Store) GetConversation(ctx context.Context, conversationID string) (Conversation, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, title, archived_at, created_at, updated_at
FROM conversations
WHERE id = ?`, conversationID)

	var c Conversation
	var archivedAt sql.NullTime
	if err := row.Scan(&c.ID, &c.Title, &archivedAt, &c.CreatedAt, &c.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Conversation{}, fmt.Errorf("conversation not found")
		}
		return Conversation{}, fmt.Errorf("get conversation: %w", err)
	}
	c.Archived = archivedAt.Valid
	return c, nil
}

func (s *Store) GetMessages(ctx context.Context, conversationID string) ([]Message, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, conversation_id, role, content, user_content, llm_content, attachments_json, thinking, has_error, created_at
FROM messages
WHERE conversation_id = ?
ORDER BY created_at ASC`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list messages: %w", err)
	}
	defer rows.Close()

	messages := make([]Message, 0)
	for rows.Next() {
		var m Message
		var attachmentsRaw string
		if err := rows.Scan(
			&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.UserContent, &m.LLMContent, &attachmentsRaw, &m.Thinking, &m.HasError, &m.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan message: %w", err)
		}
		if strings.TrimSpace(attachmentsRaw) != "" {
			if err := json.Unmarshal([]byte(attachmentsRaw), &m.Attachments); err != nil {
				return nil, fmt.Errorf("decode message attachments: %w", err)
			}
		}
		if m.Role == "user" && strings.TrimSpace(m.UserContent) != "" {
			m.Content = m.UserContent
		}
		if m.LLMContent == "" {
			m.LLMContent = m.Content
		}
		messages = append(messages, m)
	}

	return messages, rows.Err()
}

func (s *Store) GetMessage(ctx context.Context, conversationID string, messageID string) (Message, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, conversation_id, role, content, user_content, llm_content, attachments_json, thinking, has_error, created_at
FROM messages
WHERE conversation_id = ? AND id = ?
`, conversationID, messageID)

	var message Message
	var attachmentsRaw string
	if err := row.Scan(
		&message.ID,
		&message.ConversationID,
		&message.Role,
		&message.Content,
		&message.UserContent,
		&message.LLMContent,
		&attachmentsRaw,
		&message.Thinking,
		&message.HasError,
		&message.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Message{}, fmt.Errorf("message not found")
		}
		return Message{}, fmt.Errorf("get message: %w", err)
	}

	if strings.TrimSpace(attachmentsRaw) != "" {
		if err := json.Unmarshal([]byte(attachmentsRaw), &message.Attachments); err != nil {
			return Message{}, fmt.Errorf("decode message attachments: %w", err)
		}
	}

	return message, nil
}

func (s *Store) AppendMessage(ctx context.Context, m Message) error {
	attachmentsJSON := "[]"
	if len(m.Attachments) > 0 {
		encoded, err := json.Marshal(m.Attachments)
		if err != nil {
			return fmt.Errorf("marshal attachments: %w", err)
		}
		attachmentsJSON = string(encoded)
	}
	userContent := m.UserContent
	llmContent := m.LLMContent
	if userContent == "" && m.Role == "user" {
		userContent = m.Content
	}
	if llmContent == "" {
		llmContent = m.Content
	}
	_, err := s.db.ExecContext(
		ctx,
		`INSERT INTO messages(id, conversation_id, role, content, user_content, llm_content, attachments_json, thinking, has_error, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, m.Role, m.Content, userContent, llmContent, attachmentsJSON, m.Thinking, m.HasError, m.CreatedAt.UTC(),
	)
	if err != nil {
		return fmt.Errorf("insert message: %w", err)
	}

	_, err = s.db.ExecContext(
		ctx,
		`UPDATE conversations SET updated_at = ? WHERE id = ?`,
		time.Now().UTC(), m.ConversationID,
	)
	if err != nil {
		return fmt.Errorf("update conversation timestamp: %w", err)
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

func (s *Store) SetMessageThinking(ctx context.Context, messageID string, thinking string) error {
	_, err := s.db.ExecContext(ctx, `UPDATE messages SET thinking = ? WHERE id = ?`, thinking, messageID)
	if err != nil {
		return fmt.Errorf("update message thinking: %w", err)
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

func (s *Store) DeleteConversation(ctx context.Context, conversationID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM conversations WHERE id = ?`, conversationID)
	if err != nil {
		return fmt.Errorf("delete conversation: %w", err)
	}
	return nil
}
