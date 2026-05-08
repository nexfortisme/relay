package notebooks

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

const (
	chunkSize    = 1200
	chunkOverlap = 120
)

// ChunkAndIndex extracts text from a document and inserts pages + chunks + FTS
// into the per-notebook DB. For PDFs each page gets its own page row; for other
// document types a single page row with page_number=0 is created.
func ChunkAndIndex(ctx context.Context, db *sql.DB, fileID, name, contentType string, raw []byte) error {
	ext := strings.ToLower(name)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx:]
	}

	switch {
	case contentType == "application/pdf" || ext == ".pdf":
		return indexPDF(ctx, db, fileID, raw)
	default:
		text := string(raw)
		return indexText(ctx, db, fileID, 0, text)
	}
}

func indexPDF(ctx context.Context, db *sql.DB, fileID string, raw []byte) error {
	reader := bytes.NewReader(raw)
	pdfReader, err := pdf.NewReader(reader, int64(len(raw)))
	if err != nil {
		return fmt.Errorf("read pdf: %w", err)
	}

	totalPages := pdfReader.NumPage()
	indexed := 0
	for i := 1; i <= totalPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			// Some PDF structures (inline images, malformed streams) cause parse
			// errors on individual pages. Skip rather than aborting the whole file.
			continue
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		if err := indexText(ctx, db, fileID, i, text); err != nil {
			return err
		}
		indexed++
	}
	if indexed == 0 && totalPages > 0 {
		return fmt.Errorf("pdf has %d pages but no text could be extracted (scanned or image-only PDF?)", totalPages)
	}
	return nil
}

func indexText(ctx context.Context, db *sql.DB, fileID string, pageNum int, text string) error {
	now := time.Now().UTC()
	pageID := uuid.NewString()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO pages(id, file_id, page_number, content, created_at) VALUES(?, ?, ?, ?, ?)`,
		pageID, fileID, pageNum, text, now,
	)
	if err != nil {
		return fmt.Errorf("insert page: %w", err)
	}

	parts := splitText(text, chunkSize, chunkOverlap)
	for idx, part := range parts {
		chunkID := uuid.NewString()
		_, err = tx.ExecContext(ctx,
			`INSERT INTO chunks(id, file_id, page_number, chunk_index, content, created_at) VALUES(?, ?, ?, ?, ?, ?)`,
			chunkID, fileID, pageNum, idx+1, part, now,
		)
		if err != nil {
			return fmt.Errorf("insert chunk: %w", err)
		}
	}

	return tx.Commit()
}

func splitText(text string, size, overlap int) []string {
	runes := []rune(text)
	if len(runes) <= size {
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return nil
		}
		return []string{trimmed}
	}
	step := size - overlap
	if step <= 0 {
		step = size
	}
	out := make([]string, 0, len(runes)/step+1)
	for start := 0; start < len(runes); start += step {
		end := start + size
		if end > len(runes) {
			end = len(runes)
		}
		part := strings.TrimSpace(string(runes[start:end]))
		if part != "" {
			out = append(out, part)
		}
		if end == len(runes) {
			break
		}
	}
	return out
}

// tokenSet builds a set of lowercase tokens (3+ chars) from s for scoring.
func tokenSet(s string) map[string]struct{} {
	out := make(map[string]struct{})
	var tok strings.Builder
	flush := func() {
		if tok.Len() >= 3 {
			out[strings.ToLower(tok.String())] = struct{}{}
		}
		tok.Reset()
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			tok.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
