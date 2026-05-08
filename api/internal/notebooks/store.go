package notebooks

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// ChunkResult is a retrieved document chunk with source metadata.
type ChunkResult struct {
	FileID     string
	PageNumber int
	ChunkIndex int
	Content    string
}

// CSVTableMeta holds metadata about a CSV file indexed in the notebook.
type CSVTableMeta struct {
	FileID      string
	TableName   string
	FileName    string
	ColumnNames []string
	RowCount    int
	UpdatedAt   time.Time
}

// ImageMeta holds metadata about an image stored in the notebook.
type ImageMeta struct {
	FileID      string
	Name        string
	ContentType string
	SizeBytes   int64
}

// Filter represents a structured WHERE clause condition.
type Filter struct {
	Column string `json:"column"`
	Op     string `json:"op"`    // eq|neq|contains|gt|lt|gte|lte
	Value  string `json:"value"`
}

// NotebookStore provides typed operations on a per-notebook SQLite DB.
type NotebookStore struct {
	db *sql.DB
}

func NewNotebookStore(db *sql.DB) *NotebookStore {
	return &NotebookStore{db: db}
}

// SearchChunks runs an FTS5 query and returns up to max matching chunks.
func (s *NotebookStore) SearchChunks(ctx context.Context, query string, max int) ([]ChunkResult, error) {
	if max <= 0 {
		max = 10
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT c.file_id, c.page_number, c.chunk_index, c.content
		FROM chunks_fts fts
		JOIN chunks c ON c.rowid = fts.rowid
		WHERE chunks_fts MATCH ?
		ORDER BY fts.rank
		LIMIT ?
	`, query, max)
	if err != nil {
		return nil, fmt.Errorf("fts search: %w", err)
	}
	defer rows.Close()

	out := make([]ChunkResult, 0, max)
	for rows.Next() {
		var r ChunkResult
		if err := rows.Scan(&r.FileID, &r.PageNumber, &r.ChunkIndex, &r.Content); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// SampleChunks returns up to max chunks without scoring (fallback when FTS has no matches).
func (s *NotebookStore) SampleChunks(ctx context.Context, max int) ([]ChunkResult, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT file_id, page_number, chunk_index, content FROM chunks ORDER BY rowid LIMIT ?`, max)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]ChunkResult, 0, max)
	for rows.Next() {
		var r ChunkResult
		if err := rows.Scan(&r.FileID, &r.PageNumber, &r.ChunkIndex, &r.Content); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// ListCSVTables returns metadata for all imported CSV files.
func (s *NotebookStore) ListCSVTables(ctx context.Context) ([]CSVTableMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT file_id, table_name, file_name, column_names_json, row_count, updated_at FROM csv_tables ORDER BY file_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CSVTableMeta
	for rows.Next() {
		var m CSVTableMeta
		var colJSON string
		if err := rows.Scan(&m.FileID, &m.TableName, &m.FileName, &colJSON, &m.RowCount, &m.UpdatedAt); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(colJSON), &m.ColumnNames)
		out = append(out, m)
	}
	return out, rows.Err()
}

// GetCSVTableMeta returns metadata for a single CSV file, validating ownership.
func (s *NotebookStore) GetCSVTableMeta(ctx context.Context, fileID string) (CSVTableMeta, error) {
	var m CSVTableMeta
	var colJSON string
	err := s.db.QueryRowContext(ctx,
		`SELECT file_id, table_name, file_name, column_names_json, row_count, updated_at
		 FROM csv_tables WHERE file_id=?`, fileID,
	).Scan(&m.FileID, &m.TableName, &m.FileName, &colJSON, &m.RowCount, &m.UpdatedAt)
	if err != nil {
		return CSVTableMeta{}, fmt.Errorf("get csv table meta: %w", err)
	}
	_ = json.Unmarshal([]byte(colJSON), &m.ColumnNames)
	return m, nil
}

// ValidateTableName checks that tableName exists in csv_tables (prevents injection).
func (s *NotebookStore) ValidateTableName(ctx context.Context, tableName string) error {
	var count int
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM csv_tables WHERE table_name=?`, tableName).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("unknown table %q", tableName)
	}
	return nil
}

// QueryCSV runs a safe SELECT against a csv_ table with structured filters.
func (s *NotebookStore) QueryCSV(ctx context.Context, tableName string, filters []Filter, limit int) ([]map[string]string, error) {
	if err := s.ValidateTableName(ctx, tableName); err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	where, args := buildWhere(filters)
	query := fmt.Sprintf("SELECT * FROM %s%s LIMIT %d", quoteSQLiteIdent(tableName), where, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query csv: %w", err)
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var out []map[string]string
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		row := make(map[string]string, len(cols))
		for i, col := range cols {
			if vals[i] == nil {
				row[col] = ""
			} else {
				row[col] = fmt.Sprintf("%v", vals[i])
			}
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// InsertCSVRow inserts one row into a csv_ table.
func (s *NotebookStore) InsertCSVRow(ctx context.Context, tableName string, values map[string]string) (int64, error) {
	if err := s.ValidateTableName(ctx, tableName); err != nil {
		return 0, err
	}
	if len(values) == 0 {
		return 0, fmt.Errorf("no values provided")
	}

	cols := make([]string, 0, len(values))
	placeholders := make([]string, 0, len(values))
	args := make([]any, 0, len(values))
	for col, val := range values {
		cols = append(cols, quoteSQLiteIdent(col))
		placeholders = append(placeholders, "?")
		args = append(args, val)
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		quoteSQLiteIdent(tableName),
		strings.Join(cols, ", "),
		strings.Join(placeholders, ", "),
	)

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("insert csv row: %w", err)
	}

	// Update row_count
	_ = s.updateRowCount(ctx, tableName)
	return res.LastInsertId()
}

// UpdateCSVRows updates rows matching filters.
func (s *NotebookStore) UpdateCSVRows(ctx context.Context, tableName string, updates map[string]string, filters []Filter) (int64, error) {
	if err := s.ValidateTableName(ctx, tableName); err != nil {
		return 0, err
	}
	if len(updates) == 0 {
		return 0, fmt.Errorf("no updates provided")
	}

	setClauses := make([]string, 0, len(updates))
	args := make([]any, 0, len(updates)+len(filters))
	for col, val := range updates {
		setClauses = append(setClauses, quoteSQLiteIdent(col)+" = ?")
		args = append(args, val)
	}

	where, whereArgs := buildWhere(filters)
	args = append(args, whereArgs...)

	query := fmt.Sprintf("UPDATE %s SET %s%s",
		quoteSQLiteIdent(tableName),
		strings.Join(setClauses, ", "),
		where,
	)

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("update csv rows: %w", err)
	}
	return res.RowsAffected()
}

// DeleteCSVRows deletes rows matching filters.
func (s *NotebookStore) DeleteCSVRows(ctx context.Context, tableName string, filters []Filter) (int64, error) {
	if err := s.ValidateTableName(ctx, tableName); err != nil {
		return 0, err
	}

	where, args := buildWhere(filters)
	query := fmt.Sprintf("DELETE FROM %s%s", quoteSQLiteIdent(tableName), where)

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, fmt.Errorf("delete csv rows: %w", err)
	}

	_ = s.updateRowCount(ctx, tableName)
	n, _ := res.RowsAffected()
	return n, nil
}

// GetCSVTableData returns all columns and up to 1000 rows for the viewer.
func (s *NotebookStore) GetCSVTableData(ctx context.Context, tableName string) (columns []string, rows [][]string, err error) {
	if err := s.ValidateTableName(ctx, tableName); err != nil {
		return nil, nil, err
	}

	dbRows, err := s.db.QueryContext(ctx,
		fmt.Sprintf("SELECT * FROM %s LIMIT 1000", quoteSQLiteIdent(tableName)))
	if err != nil {
		return nil, nil, err
	}
	defer dbRows.Close()

	cols, err := dbRows.Columns()
	if err != nil {
		return nil, nil, err
	}

	var out [][]string
	for dbRows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := dbRows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make([]string, len(cols))
		for i := range cols {
			if vals[i] == nil {
				row[i] = ""
			} else {
				row[i] = fmt.Sprintf("%v", vals[i])
			}
		}
		out = append(out, row)
	}
	return cols, out, dbRows.Err()
}

// ListImageMeta returns metadata for all images in the notebook.
func (s *NotebookStore) ListImageMeta(ctx context.Context) ([]ImageMeta, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT file_id, name, content_type, size_bytes FROM image_meta ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ImageMeta
	for rows.Next() {
		var m ImageMeta
		if err := rows.Scan(&m.FileID, &m.Name, &m.ContentType, &m.SizeBytes); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// InsertImageMeta records image metadata in the notebook DB.
func (s *NotebookStore) InsertImageMeta(ctx context.Context, fileID, name, contentType string, sizeBytes int64) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO image_meta(file_id, name, content_type, size_bytes, created_at)
		 VALUES(?, ?, ?, ?, ?)`,
		fileID, name, contentType, sizeBytes, time.Now().UTC(),
	)
	return err
}

// buildWhere converts a Filter slice into a safe WHERE clause + args.
// Returns ("", nil) when filters is empty.
func buildWhere(filters []Filter) (string, []any) {
	if len(filters) == 0 {
		return "", nil
	}
	clauses := make([]string, 0, len(filters))
	args := make([]any, 0, len(filters))
	for _, f := range filters {
		col := quoteSQLiteIdent(f.Column)
		switch f.Op {
		case "eq", "":
			clauses = append(clauses, col+" = ?")
			args = append(args, f.Value)
		case "neq":
			clauses = append(clauses, col+" != ?")
			args = append(args, f.Value)
		case "contains":
			clauses = append(clauses, col+" LIKE ?")
			args = append(args, "%"+f.Value+"%")
		case "gt":
			clauses = append(clauses, col+" > ?")
			args = append(args, f.Value)
		case "lt":
			clauses = append(clauses, col+" < ?")
			args = append(args, f.Value)
		case "gte":
			clauses = append(clauses, col+" >= ?")
			args = append(args, f.Value)
		case "lte":
			clauses = append(clauses, col+" <= ?")
			args = append(args, f.Value)
		}
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (s *NotebookStore) updateRowCount(ctx context.Context, tableName string) error {
	var count int
	err := s.db.QueryRowContext(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM %s", quoteSQLiteIdent(tableName))).Scan(&count)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`UPDATE csv_tables SET row_count=?, updated_at=? WHERE table_name=?`,
		count, time.Now().UTC(), tableName)
	return err
}
