package store

import (
	"context"
	"database/sql"
	"fmt"
)

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
			model TEXT NOT NULL DEFAULT '',
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

	notebooksTable := `
		CREATE TABLE IF NOT EXISTS notebooks (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			description TEXT NOT NULL DEFAULT '',
			system_prompt TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_notebooks_user ON notebooks(user_id, created_at DESC);
	`

	notebookFilesTable := `
		CREATE TABLE IF NOT EXISTS notebook_files (
			id TEXT PRIMARY KEY,
			notebook_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			name TEXT NOT NULL,
			content_type TEXT NOT NULL DEFAULT '',
			size_bytes INTEGER NOT NULL DEFAULT 0,
			file_kind TEXT NOT NULL DEFAULT 'document',
			status TEXT NOT NULL DEFAULT 'pending',
			error_text TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			updated_at DATETIME NOT NULL,
			FOREIGN KEY(notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_notebook_files_notebook ON notebook_files(notebook_id, created_at);
	`

	notebookFileDataTable := `
		CREATE TABLE IF NOT EXISTS notebook_file_data (
			file_id TEXT PRIMARY KEY,
			data BLOB NOT NULL,
			FOREIGN KEY(file_id) REFERENCES notebook_files(id) ON DELETE CASCADE
		);
	`

	notebookJobsTable := `
		CREATE TABLE IF NOT EXISTS notebook_jobs (
			id TEXT PRIMARY KEY,
			notebook_id TEXT NOT NULL,
			file_id TEXT NOT NULL,
			user_id TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'pending',
			error_text TEXT NOT NULL DEFAULT '',
			created_at DATETIME NOT NULL,
			started_at DATETIME,
			finished_at DATETIME,
			FOREIGN KEY(notebook_id) REFERENCES notebooks(id) ON DELETE CASCADE,
			FOREIGN KEY(file_id) REFERENCES notebook_files(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_notebook_jobs_status ON notebook_jobs(status, created_at);
	`

	tables := []string{
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
		notebooksTable,
		notebookFilesTable,
		notebookFileDataTable,
		notebookJobsTable,
	}

	for _, table := range tables {
		if _, err := s.db.ExecContext(ctx, table); err != nil {
			return fmt.Errorf("create table %s: %w", table, err)
		}
	}

	if err := s.ensureColumn(ctx, "messages", "model", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "conversations", "notebook_id", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "notebooks", "skill_prompt", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "notebook_files", "page_count", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.ensureColumn(ctx, "notebook_files", "pages_indexed", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	return nil
}

func (s *Store) ensureColumn(ctx context.Context, table string, column string, definition string) error {
	rows, err := s.db.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("scan %s column info: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("read %s columns: %w", table, err)
	}

	if _, err := s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, definition)); err != nil {
		return fmt.Errorf("add %s.%s column: %w", table, column, err)
	}
	return nil
}
