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
	ID              string        `json:"id"`
	ConversationID  string        `json:"conversationId"`
	Role            string        `json:"role"`
	Content         string        `json:"content"`
	UserContent     string        `json:"userContent,omitempty"`
	LLMContent      string        `json:"llmContent,omitempty"`
	Attachments     []MessageFile `json:"attachments,omitempty"`
	Thinking        string        `json:"thinking,omitempty"`
	HasError        bool          `json:"hasError,omitempty"`
	ElapsedMs       int64         `json:"elapsedMs,omitempty"`
	InputTokens     int           `json:"inputTokens,omitempty"`
	OutputTokens    int           `json:"outputTokens,omitempty"`
	ReasoningTokens int           `json:"reasoningTokens,omitempty"`
	TotalTokens     int           `json:"totalTokens,omitempty"`
	CreatedAt       time.Time     `json:"createdAt"`
}

// MessageFile is the lightweight projection of a file attached to a message.
// ID is empty when the file did not make it to the database (failed uploads).
type MessageFile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type File struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	ContentType string    `json:"contentType"`
	SizeBytes   int64     `json:"sizeBytes"`
	Data        []byte    `json:"-"`
	CreatedAt   time.Time `json:"createdAt"`
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
		elapsed_ms INTEGER NOT NULL DEFAULT 0,
		input_tokens INTEGER NOT NULL DEFAULT 0,
		output_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME NOT NULL,
		FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_messages_conversation_created
		ON messages(conversation_id, created_at);

		CREATE TABLE IF NOT EXISTS files (
		id TEXT PRIMARY KEY,
		name TEXT NOT NULL,
		content_type TEXT NOT NULL DEFAULT '',
		size_bytes INTEGER NOT NULL,
		data BLOB NOT NULL,
		created_at DATETIME NOT NULL
		);

		CREATE TABLE IF NOT EXISTS message_files (
		message_id TEXT NOT NULL,
		file_id TEXT NOT NULL,
		position INTEGER NOT NULL,
		PRIMARY KEY(message_id, file_id),
		FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE CASCADE,
		FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE
		);

		CREATE INDEX IF NOT EXISTS idx_message_files_message
		ON message_files(message_id, position);

		CREATE INDEX IF NOT EXISTS idx_message_files_file
		ON message_files(file_id);

		CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL
		);
	`

	if _, err := s.db.ExecContext(ctx, schema); err != nil {
		return fmt.Errorf("migrate sqlite schema: %w", err)
	}

	// Older databases may carry these columns/tables; the ALTERs below are
	// best-effort and idempotent so a fresh start always succeeds.
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE conversations ADD COLUMN archived_at DATETIME`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN thinking TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN user_content TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN llm_content TEXT NOT NULL DEFAULT ''`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN attachments_json TEXT NOT NULL DEFAULT '[]'`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN has_error INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN elapsed_ms INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN input_tokens INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN output_tokens INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN reasoning_tokens INTEGER NOT NULL DEFAULT 0`)
	_, _ = s.db.ExecContext(ctx, `ALTER TABLE messages ADD COLUMN total_tokens INTEGER NOT NULL DEFAULT 0`)

	// The old per-message attachments table has been replaced by the
	// dedicated files + message_files tables. Drop it on first run; any
	// blobs it held are intentionally discarded (callers were warned).
	if _, err := s.db.ExecContext(ctx, `DROP TABLE IF EXISTS message_attachments`); err != nil {
		return fmt.Errorf("drop legacy message_attachments: %w", err)
	}

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
		FROM conversations
	`
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
SELECT id, conversation_id, role, content, user_content, llm_content, attachments_json, thinking, has_error, elapsed_ms, input_tokens, output_tokens, reasoning_tokens, total_tokens, created_at
FROM messages
WHERE conversation_id = ?
ORDER BY created_at ASC`, conversationID)
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
			&m.ID, &m.ConversationID, &m.Role, &m.Content, &m.UserContent, &m.LLMContent, &fallbackJSON, &m.Thinking, &m.HasError, &m.ElapsedMs, &m.InputTokens, &m.OutputTokens, &m.ReasoningTokens, &m.TotalTokens, &m.CreatedAt,
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
SELECT id, conversation_id, role, content, user_content, llm_content, attachments_json, thinking, has_error, elapsed_ms, input_tokens, output_tokens, reasoning_tokens, total_tokens, created_at
FROM messages
WHERE conversation_id = ? AND id = ?
`, conversationID, messageID)

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
		`INSERT INTO messages(id, conversation_id, role, content, user_content, llm_content, attachments_json, thinking, has_error, elapsed_ms, input_tokens, output_tokens, reasoning_tokens, total_tokens, created_at) VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.ConversationID, m.Role, m.Content, userContent, llmContent, fallbackJSON, m.Thinking, m.HasError, m.ElapsedMs, m.InputTokens, m.OutputTokens, m.ReasoningTokens, m.TotalTokens, m.CreatedAt.UTC(),
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
			`INSERT INTO files(id, name, content_type, size_bytes, data, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
			f.ID, f.Name, f.ContentType, size, f.Data, createdAt.UTC(),
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

func (s *Store) GetFile(ctx context.Context, fileID string) (File, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, content_type, size_bytes, data, created_at
FROM files
WHERE id = ?
`, fileID)

	var f File
	if err := row.Scan(&f.ID, &f.Name, &f.ContentType, &f.SizeBytes, &f.Data, &f.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return File{}, fmt.Errorf("file not found")
		}
		return File{}, fmt.Errorf("get file: %w", err)
	}
	return f, nil
}

func (s *Store) GetMessageFileIDs(ctx context.Context, messageID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT file_id FROM message_files
WHERE message_id = ?
ORDER BY position ASC
`, messageID)
	if err != nil {
		return nil, fmt.Errorf("list message file ids: %w", err)
	}
	defer rows.Close()

	out := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan message file id: %w", err)
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) attachmentsForMessages(ctx context.Context, messageIDs []string) (map[string][]MessageFile, error) {
	out := make(map[string][]MessageFile)
	if len(messageIDs) == 0 {
		return out, nil
	}
	placeholders := strings.Repeat("?,", len(messageIDs))
	placeholders = placeholders[:len(placeholders)-1]
	args := make([]any, 0, len(messageIDs))
	for _, id := range messageIDs {
		args = append(args, id)
	}
	query := fmt.Sprintf(`
SELECT mf.message_id, f.id, f.name
FROM message_files mf
JOIN files f ON f.id = mf.file_id
WHERE mf.message_id IN (%s)
ORDER BY mf.message_id, mf.position ASC
`, placeholders)
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query message attachments: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var messageID string
		var file MessageFile
		if err := rows.Scan(&messageID, &file.ID, &file.Name); err != nil {
			return nil, fmt.Errorf("scan message attachment: %w", err)
		}
		out[messageID] = append(out[messageID], file)
	}
	return out, rows.Err()
}

func decodeFallbackAttachmentNames(raw string) []string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || trimmed == "[]" {
		return nil
	}
	var names []string
	if err := json.Unmarshal([]byte(trimmed), &names); err != nil {
		return nil
	}
	return names
}

func namesToMessageFiles(names []string) []MessageFile {
	out := make([]MessageFile, 0, len(names))
	for _, name := range names {
		out = append(out, MessageFile{Name: name})
	}
	return out
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

func (s *Store) GetSetting(ctx context.Context, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key = ?`, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting %s: %w", key, err)
	}
	return value, true, nil
}

func (s *Store) UpsertSetting(ctx context.Context, key, value string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings(key, value, updated_at) VALUES(?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, now,
	)
	if err != nil {
		return fmt.Errorf("upsert setting %s: %w", key, err)
	}
	return nil
}

func (s *Store) GetAllSettings(ctx context.Context) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings`)
	if err != nil {
		return nil, fmt.Errorf("get all settings: %w", err)
	}
	defer rows.Close()
	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		result[key] = value
	}
	return result, rows.Err()
}
