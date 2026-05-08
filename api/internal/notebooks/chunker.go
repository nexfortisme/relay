package notebooks

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"image/jpeg"
	"strings"
	"time"
	"unicode"

	"github.com/gen2brain/go-fitz"
	"github.com/google/uuid"
	"github.com/ledongthuc/pdf"
)

const (
	chunkSize    = 1200
	chunkOverlap = 120
	renderDPI    = 96.0
	jpegQuality  = 85
)

// ImageDescriber optionally describes a rendered page image for RAG indexing.
// Returning ("", nil) is treated as no description available.
type ImageDescriber func(ctx context.Context, jpegBytes []byte) (string, error)

// ProgressFunc is called during PDF processing with (pagesIndexed, pageCount).
// Called once with (0, N) when page count is known, then after each page completes.
type ProgressFunc func(pagesIndexed, pageCount int)

// ChunkAndIndex extracts text from a document and inserts pages + chunks + FTS
// into the per-notebook DB. For PDFs each page gets its own page row; for other
// document types a single page row with page_number=0 is created.
//
// describeImage and onProgress are optional; pass nil to skip either.
func ChunkAndIndex(ctx context.Context, db *sql.DB, fileID, name, contentType string, raw []byte, describeImage ImageDescriber, onProgress ProgressFunc) error {
	ext := strings.ToLower(name)
	if idx := strings.LastIndex(ext, "."); idx >= 0 {
		ext = ext[idx:]
	}

	switch {
	case contentType == "application/pdf" || ext == ".pdf":
		return indexPDF(ctx, db, fileID, raw, describeImage, onProgress)
	default:
		text := string(raw)
		return indexText(ctx, db, fileID, 0, text, nil, "")
	}
}

func indexPDF(ctx context.Context, db *sql.DB, fileID string, raw []byte, describeImage ImageDescriber, onProgress ProgressFunc) error {
	// Text extraction pass using ledongthuc/pdf.
	reader := bytes.NewReader(raw)
	pdfReader, err := pdf.NewReader(reader, int64(len(raw)))
	if err != nil {
		return fmt.Errorf("read pdf: %w", err)
	}
	totalPages := pdfReader.NumPage()
	if onProgress != nil {
		onProgress(0, totalPages)
	}

	pageTexts := make(map[int]string, totalPages)
	for i := 1; i <= totalPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		if strings.TrimSpace(text) != "" {
			pageTexts[i] = text
		}
	}

	// Image rendering pass using go-fitz (MuPDF).
	fitzDoc, fitzErr := fitz.NewFromMemory(raw)
	if fitzErr != nil {
		fitzDoc = nil
	}
	if fitzDoc != nil {
		defer fitzDoc.Close()
	}

	indexed := 0
	for i := 1; i <= totalPages; i++ {
		text := pageTexts[i]

		var imgData []byte
		var imgType string

		if fitzDoc != nil {
			img, err := fitzDoc.ImageDPI(i-1, renderDPI)
			if err == nil && img != nil {
				var buf bytes.Buffer
				if encErr := jpeg.Encode(&buf, img, &jpeg.Options{Quality: jpegQuality}); encErr == nil {
					imgData = buf.Bytes()
					imgType = "image/jpeg"
				}
			}
		}

		if imgData != nil && describeImage != nil && strings.TrimSpace(text) == "" {
			// Scanned page: description becomes the indexed content.
			desc, err := describeImage(ctx, imgData)
			if err == nil && strings.TrimSpace(desc) != "" {
				text = "[Page image description: " + desc + "]"
			}
		} else if imgData != nil && describeImage != nil {
			// Mixed page: append description to existing text.
			desc, err := describeImage(ctx, imgData)
			if err == nil && strings.TrimSpace(desc) != "" {
				text += "\n\n[Page image description: " + desc + "]"
			}
		}

		if strings.TrimSpace(text) == "" {
			if onProgress != nil {
				onProgress(indexed, totalPages)
			}
			continue
		}

		if err := indexText(ctx, db, fileID, i, text, imgData, imgType); err != nil {
			return err
		}
		indexed++
		if onProgress != nil {
			onProgress(indexed, totalPages)
		}
	}

	if indexed == 0 && totalPages > 0 {
		return fmt.Errorf("pdf has %d pages but no text could be extracted (scanned or image-only PDF?)", totalPages)
	}
	return nil
}

func indexText(ctx context.Context, db *sql.DB, fileID string, pageNum int, text string, imgData []byte, imgType string) error {
	now := time.Now().UTC()
	pageID := uuid.NewString()

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx,
		`INSERT OR IGNORE INTO pages(id, file_id, page_number, content, image_data, image_type, created_at) VALUES(?, ?, ?, ?, ?, ?, ?)`,
		pageID, fileID, pageNum, text, imgData, imgType, now,
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
