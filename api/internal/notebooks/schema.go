package notebooks

const notebookDBSchema = `
CREATE TABLE IF NOT EXISTS pages (
	id TEXT PRIMARY KEY,
	file_id TEXT NOT NULL,
	page_number INTEGER NOT NULL DEFAULT 0,
	content TEXT NOT NULL,
	image_data BLOB,
	image_type TEXT,
	created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_pages_file ON pages(file_id, page_number);

CREATE TABLE IF NOT EXISTS chunks (
	id TEXT PRIMARY KEY,
	file_id TEXT NOT NULL,
	page_number INTEGER NOT NULL DEFAULT 0,
	chunk_index INTEGER NOT NULL,
	content TEXT NOT NULL,
	created_at DATETIME NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_chunks_file ON chunks(file_id, chunk_index);

CREATE VIRTUAL TABLE IF NOT EXISTS chunks_fts USING fts5(
	content,
	content='chunks',
	content_rowid='rowid',
	tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS chunks_ai AFTER INSERT ON chunks BEGIN
	INSERT INTO chunks_fts(rowid, content) VALUES (new.rowid, new.content);
END;

CREATE TRIGGER IF NOT EXISTS chunks_ad AFTER DELETE ON chunks BEGIN
	INSERT INTO chunks_fts(chunks_fts, rowid, content) VALUES('delete', old.rowid, old.content);
END;

CREATE TABLE IF NOT EXISTS csv_tables (
	file_id TEXT PRIMARY KEY,
	table_name TEXT NOT NULL UNIQUE,
	file_name TEXT NOT NULL DEFAULT '',
	column_names_json TEXT NOT NULL DEFAULT '[]',
	row_count INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL,
	updated_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS image_meta (
	file_id TEXT PRIMARY KEY,
	name TEXT NOT NULL,
	content_type TEXT NOT NULL,
	size_bytes INTEGER NOT NULL DEFAULT 0,
	created_at DATETIME NOT NULL
);
`
