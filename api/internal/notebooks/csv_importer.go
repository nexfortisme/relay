package notebooks

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

var nonAlphanumRE = regexp.MustCompile(`[^a-z0-9]+`)

// sanitizeColumnName converts an arbitrary CSV header to a safe SQL identifier.
func sanitizeColumnName(s string) string {
	lower := strings.ToLower(strings.TrimSpace(s))
	clean := nonAlphanumRE.ReplaceAllString(lower, "_")
	clean = strings.Trim(clean, "_")
	if clean == "" {
		clean = "col"
	}
	// Prefix digit-starting names
	if len(clean) > 0 && clean[0] >= '0' && clean[0] <= '9' {
		clean = "col_" + clean
	}
	return clean
}

// deduplicateColumns ensures column names are unique by appending _2, _3, ...
func deduplicateColumns(names []string) []string {
	seen := make(map[string]int)
	out := make([]string, len(names))
	for i, name := range names {
		seen[name]++
		if seen[name] == 1 {
			out[i] = name
		} else {
			out[i] = fmt.Sprintf("%s_%d", name, seen[name])
		}
	}
	return out
}

// quoteSQLiteIdent wraps an identifier in double quotes, escaping inner quotes.
func quoteSQLiteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// CSVTableName derives the dynamic table name from a fileID.
// Uses first 8 characters of the UUID.
func CSVTableName(fileID string) string {
	safe := strings.ReplaceAll(fileID, "-", "")
	if len(safe) > 8 {
		safe = safe[:8]
	}
	return "csv_" + safe
}

// ImportCSV parses raw CSV bytes and imports them into a dynamic table in db.
// tableName should be the result of CSVTableName(fileID).
// Returns the sanitized column names and row count.
func ImportCSV(ctx context.Context, db *sql.DB, fileID, tableName, fileName string, raw []byte) (columnNames []string, rowCount int, err error) {
	r := csv.NewReader(bytes.NewReader(raw))
	r.LazyQuotes = true
	r.TrimLeadingSpace = true

	records, err := r.ReadAll()
	if err != nil {
		return nil, 0, fmt.Errorf("parse csv: %w", err)
	}
	if len(records) == 0 {
		return nil, 0, fmt.Errorf("csv is empty")
	}

	// First row = headers
	rawHeaders := records[0]
	sanitized := make([]string, len(rawHeaders))
	for i, h := range rawHeaders {
		sanitized[i] = sanitizeColumnName(h)
		if sanitized[i] == "" {
			sanitized[i] = fmt.Sprintf("col_%d", i+1)
		}
	}
	columns := deduplicateColumns(sanitized)

	// Build CREATE TABLE statement
	colDefs := make([]string, len(columns))
	for i, c := range columns {
		colDefs[i] = quoteSQLiteIdent(c) + " TEXT NOT NULL DEFAULT ''"
	}
	createSQL := fmt.Sprintf(
		"CREATE TABLE IF NOT EXISTS %s (rowid INTEGER PRIMARY KEY AUTOINCREMENT, %s)",
		quoteSQLiteIdent(tableName),
		strings.Join(colDefs, ", "),
	)

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, createSQL); err != nil {
		return nil, 0, fmt.Errorf("create csv table: %w", err)
	}

	// Prepare insert
	placeholders := make([]string, len(columns))
	quotedCols := make([]string, len(columns))
	for i, c := range columns {
		placeholders[i] = "?"
		quotedCols[i] = quoteSQLiteIdent(c)
	}
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) VALUES (%s)",
		quoteSQLiteIdent(tableName),
		strings.Join(quotedCols, ", "),
		strings.Join(placeholders, ", "),
	)
	stmt, err := tx.PrepareContext(ctx, insertSQL)
	if err != nil {
		return nil, 0, fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	dataRows := records[1:]
	for _, row := range dataRows {
		args := make([]any, len(columns))
		for i := range columns {
			if i < len(row) {
				args[i] = row[i]
			} else {
				args[i] = ""
			}
		}
		if _, err := stmt.ExecContext(ctx, args...); err != nil {
			return nil, 0, fmt.Errorf("insert csv row: %w", err)
		}
	}

	// Upsert csv_tables metadata
	now := time.Now().UTC()
	colJSON, _ := json.Marshal(columns)
	_, err = tx.ExecContext(ctx,
		`INSERT INTO csv_tables(file_id, table_name, file_name, column_names_json, row_count, created_at, updated_at)
		 VALUES(?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(file_id) DO UPDATE SET
		   table_name=excluded.table_name,
		   file_name=excluded.file_name,
		   column_names_json=excluded.column_names_json,
		   row_count=excluded.row_count,
		   updated_at=excluded.updated_at`,
		fileID, tableName, fileName, string(colJSON), len(dataRows), now, now,
	)
	if err != nil {
		return nil, 0, fmt.Errorf("upsert csv_tables: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, 0, fmt.Errorf("commit: %w", err)
	}

	return columns, len(dataRows), nil
}
