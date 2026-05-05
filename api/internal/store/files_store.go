package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

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
