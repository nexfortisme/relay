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

type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Session struct {
	ID               string     `json:"id"`
	UserID           string     `json:"userId"`
	RefreshTokenHash string     `json:"-"`
	ExpiresAt        time.Time  `json:"expiresAt"`
	RememberMe       bool       `json:"rememberMe"`
	CreatedAt        time.Time  `json:"createdAt"`
	RevokedAt        *time.Time `json:"revokedAt,omitempty"`
}

var ErrNotFound = errors.New("not found")

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

	usersTable := `
		CREATE TABLE IF NOT EXISTS users (
			id TEXT PRIMARY KEY,
			username TEXT NOT NULL UNIQUE,
			password_hash TEXT NOT NULL,
			created_at DATETIME NOT NULL
		);
	`
	
	sessionsTable := `
		CREATE TABLE IF NOT EXISTS sessions (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			refresh_token_hash TEXT NOT NULL UNIQUE,
			expires_at DATETIME NOT NULL,
			remember_me INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			revoked_at DATETIME,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);
	`

	sessionsIndcies := `
		CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id);
		CREATE INDEX IF NOT EXISTS idx_sessions_refresh ON sessions(refresh_token_hash);
	`

	conversationsTable := `
		CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			title TEXT NOT NULL DEFAULT 'New chat',
			archived_at DATETIME,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL
		);
	`

	messagesTable := `
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
	`

	messageIndcies := `
		CREATE INDEX IF NOT EXISTS idx_messages_conversation_created
		ON messages(conversation_id, created_at);
	`

	filesTable := `
		CREATE TABLE IF NOT EXISTS files (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL DEFAULT '',
			name TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL,
			data BLOB NOT NULL,
			created_at DATETIME NOT NULL
		);
	`

	messageFilesTable := `
		CREATE TABLE IF NOT EXISTS message_files (
			message_id TEXT NOT NULL,
			file_id TEXT NOT NULL,
			position INTEGER NOT NULL,
			PRIMARY KEY(message_id, file_id),
			FOREIGN KEY(message_id) REFERENCES messages(id) ON DELETE CASCADE,
			FOREIGN KEY(file_id) REFERENCES files(id) ON DELETE CASCADE
		);
	`

	messageFilesIndcies := `
		CREATE INDEX IF NOT EXISTS idx_message_files_message
		ON message_files(message_id, position);

		CREATE INDEX IF NOT EXISTS idx_message_files_file
		ON message_files(file_id);
	`

	settingsTable := `
		CREATE TABLE IF NOT EXISTS settings (
		user_id TEXT NOT NULL DEFAULT '',
		key TEXT NOT NULL,
		value TEXT NOT NULL,
		updated_at DATETIME NOT NULL,
		PRIMARY KEY(user_id, key)
		);
	`

	feedsTable := `
		CREATE TABLE IF NOT EXISTS feeds (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			url TEXT NOT NULL,
			title TEXT NOT NULL,
			site_url TEXT NOT NULL DEFAULT '',
			description TEXT NOT NULL DEFAULT '',
			polling_interval_minutes INTEGER NOT NULL DEFAULT 30,
			auto_summarize INTEGER NOT NULL DEFAULT 0,
			auto_add_to_notebook INTEGER NOT NULL DEFAULT 0,
			notebook_id TEXT NOT NULL DEFAULT '',
			last_checked_at DATETIME,
			next_check_at DATETIME NOT NULL,
			last_error TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			UNIQUE(user_id, url)
		);
	`

	feedsIndcies := `
		CREATE INDEX IF NOT EXISTS idx_feeds_user
		ON feeds(user_id, title);

		CREATE INDEX IF NOT EXISTS idx_feeds_due
		ON feeds(next_check_at);
	`

	feedItemsTable := `
		CREATE TABLE IF NOT EXISTS feed_items (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			feed_id TEXT NOT NULL,
			external_id TEXT NOT NULL,
			title TEXT NOT NULL,
			url TEXT NOT NULL DEFAULT '',
			author TEXT NOT NULL DEFAULT '',
			published_at DATETIME,
			preview TEXT NOT NULL DEFAULT '',
			content TEXT NOT NULL DEFAULT '',
			media_type TEXT NOT NULL DEFAULT '',
			media_url TEXT NOT NULL DEFAULT '',
			summary TEXT NOT NULL DEFAULT '',
			summary_status TEXT NOT NULL DEFAULT '',
			summary_error TEXT NOT NULL DEFAULT '',
			read INTEGER NOT NULL DEFAULT 0,
			starred INTEGER NOT NULL DEFAULT 0,
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY(feed_id) REFERENCES feeds(id) ON DELETE CASCADE,
			UNIQUE(feed_id, external_id)
		);
	`

	feedItemsIndcies := `
		CREATE INDEX IF NOT EXISTS idx_feed_items_user_read
		ON feed_items(user_id, read, published_at DESC, created_at DESC);

		CREATE INDEX IF NOT EXISTS idx_feed_items_user_starred
		ON feed_items(user_id, starred, published_at DESC, created_at DESC);

		CREATE INDEX IF NOT EXISTS idx_feed_items_feed
		ON feed_items(feed_id, published_at DESC, created_at DESC);
	`

	tables := []string {
		usersTable,
		sessionsTable,
		sessionsIndcies,
		conversationsTable,
		messagesTable,
		messageIndcies,
		filesTable,
		messageFilesTable,
		messageFilesIndcies,
		settingsTable,
		feedsTable,
		feedsIndcies,
		feedItemsTable,
		feedItemsIndcies,
	}

	for _, table := range tables {
		if _, err := s.db.ExecContext(ctx, table); err != nil {
			return fmt.Errorf("create table %s: %w", table, err)
		}
	}

	return nil
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

func (s *Store) GetFile(ctx context.Context, userID string, fileID string) (File, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT id, name, content_type, size_bytes, data, created_at
FROM files
WHERE id = ? AND user_id = ?
`, fileID, userID)

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

func (s *Store) GetSetting(ctx context.Context, userID, key string) (string, bool, error) {
	var value string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE user_id = ? AND key = ?`, userID, key).Scan(&value)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("get setting %s: %w", key, err)
	}
	return value, true, nil
}

func (s *Store) UpsertSetting(ctx context.Context, userID, key, value string) error {
	now := time.Now().UTC()
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO settings(user_id, key, value, updated_at) VALUES(?, ?, ?, ?)
		 ON CONFLICT(user_id, key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		userID, key, value, now,
	)
	if err != nil {
		return fmt.Errorf("upsert setting %s: %w", key, err)
	}
	return nil
}

func (s *Store) GetAllSettings(ctx context.Context, userID string) (map[string]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key, value FROM settings WHERE user_id = ?`, userID)
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

// CopyDefaultSettings seeds a per-user copy of the default global settings on
// registration, so each new user starts with a private settings row they can
// edit independently.
func (s *Store) CopyDefaultSettings(ctx context.Context, userID string, defaults map[string]string) error {
	now := time.Now().UTC()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin seed settings: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	for k, v := range defaults {
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO settings(user_id, key, value, updated_at) VALUES(?, ?, ?, ?)
			 ON CONFLICT(user_id, key) DO NOTHING`,
			userID, k, v, now,
		); err != nil {
			return fmt.Errorf("Error Seeding Setting %s: %w", k, err)
		}
	}
	return tx.Commit()
}
