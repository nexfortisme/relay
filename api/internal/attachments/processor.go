package attachments

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"mime"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"
)

type UploadedFile struct {
	Name        string
	ContentType string
	Data        []byte
}

type chunk struct {
	Source string
	Index  int
	Text   string
	Score  int
}

const (
	defaultMaxFileBytes  = 3 * 1024 * 1024
	defaultMaxImageBytes = 15 * 1024 * 1024
	inlineCharBudget     = 12000
	chunkSizeRunes       = 1200
	chunkOverlapRunes    = 180
	maxReturnedChunks    = 8
)

type PromptOptions struct {
	MaxFileBytes  int
	MaxImageBytes int
}

func (o PromptOptions) withDefaults() PromptOptions {
	if o.MaxFileBytes <= 0 {
		o.MaxFileBytes = defaultMaxFileBytes
	}
	if o.MaxImageBytes <= 0 {
		o.MaxImageBytes = defaultMaxImageBytes
	}
	return o
}

func BuildPrompt(userPrompt string, files []UploadedFile, opts PromptOptions) (string, error) {
	options := opts.withDefaults()
	trimmedPrompt := strings.TrimSpace(userPrompt)
	if len(files) == 0 {
		return trimmedPrompt, nil
	}

	images := make([]string, 0, len(files))
	skippedImages := make([]string, 0)
	documents := make([]chunk, 0, len(files)*2)
	totalDocChars := 0

	for _, file := range files {
		if len(file.Data) == 0 {
			continue
		}

		contentType := normalizedContentType(file.ContentType, file.Name)
		if strings.HasPrefix(contentType, "image/") {
			if len(file.Data) > options.MaxImageBytes {
				skippedImages = append(skippedImages, file.Name)
				continue
			}
			images = append(images, imageBlock(file.Name, contentType, file.Data))
			continue
		}

		if len(file.Data) > options.MaxFileBytes {
			return "", fmt.Errorf("%s exceeds max size of %d MB", file.Name, options.MaxFileBytes/(1024*1024))
		}

		text, err := extractDocumentText(file.Name, contentType, file.Data)
		if err != nil {
			return "", err
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		totalDocChars += len(text)
		for idx, part := range splitText(text, chunkSizeRunes, chunkOverlapRunes) {
			documents = append(documents, chunk{
				Source: file.Name,
				Index:  idx + 1,
				Text:   part,
			})
		}
	}

	var builder strings.Builder
	builder.WriteString(trimmedPrompt)
	builder.WriteString("\n\n---\n")

	if len(images) > 0 {
		builder.WriteString("Attached images are included below as data URLs.\n")
		builder.WriteString("Use these for visual reasoning when your model supports vision.\n\n")
		for _, block := range images {
			builder.WriteString(block)
			builder.WriteString("\n\n")
		}
	}
	if len(skippedImages) > 0 {
		builder.WriteString(fmt.Sprintf(
			"Ignored oversized images (max %d KB each): %s\n\n",
			options.MaxImageBytes/1024,
			strings.Join(skippedImages, ", "),
		))
	}

	if len(documents) == 0 {
		return strings.TrimSpace(builder.String()), nil
	}

	if totalDocChars <= inlineCharBudget && len(files) <= 2 {
		builder.WriteString("Decision: include documents in full context.\n\n")
		for _, c := range documents {
			builder.WriteString(fmt.Sprintf("[Document %s part %d]\n%s\n\n", c.Source, c.Index, c.Text))
		}
		return strings.TrimSpace(builder.String()), nil
	}

	scored := scoreChunks(trimmedPrompt, documents)
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score == scored[j].Score {
			if scored[i].Source == scored[j].Source {
				return scored[i].Index < scored[j].Index
			}
			return scored[i].Source < scored[j].Source
		}
		return scored[i].Score > scored[j].Score
	})
	if len(scored) > maxReturnedChunks {
		scored = scored[:maxReturnedChunks]
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Source == scored[j].Source {
			return scored[i].Index < scored[j].Index
		}
		return scored[i].Source < scored[j].Source
	})

	builder.WriteString("Decision: documents were chunked for retrieval-style context.\n")
	builder.WriteString("The following chunks were selected for relevance to the request.\n\n")
	for _, c := range scored {
		builder.WriteString(fmt.Sprintf("[RAG %s#%d]\n%s\n\n", c.Source, c.Index, c.Text))
	}
	return strings.TrimSpace(builder.String()), nil
}

func normalizedContentType(contentType string, filename string) string {
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err == nil && mediaType != "" {
		return strings.ToLower(mediaType)
	}
	ext := strings.ToLower(filepath.Ext(filename))
	switch ext {
	case ".md":
		return "text/markdown"
	case ".txt":
		return "text/plain"
	case ".pdf":
		return "application/pdf"
	case ".json":
		return "application/json"
	case ".csv":
		return "text/csv"
	default:
		return contentType
	}
}

func imageBlock(name string, contentType string, raw []byte) string {
	return fmt.Sprintf(
		"[Image %s]\n![%s](data:%s;base64,%s)",
		name,
		name,
		contentType,
		base64.StdEncoding.EncodeToString(raw),
	)
}

func extractDocumentText(name string, contentType string, raw []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(name))
	switch {
	case contentType == "application/pdf" || ext == ".pdf":
		return extractPDFText(raw)
	case strings.HasPrefix(contentType, "text/"),
		contentType == "application/json",
		contentType == "application/xml",
		contentType == "application/yaml",
		contentType == "application/x-yaml",
		ext == ".json",
		ext == ".md",
		ext == ".txt",
		ext == ".csv",
		ext == ".xml",
		ext == ".yaml",
		ext == ".yml":
		return string(raw), nil
	default:
		return "", fmt.Errorf("unsupported file type for %s", name)
	}
}

func extractPDFText(raw []byte) (string, error) {
	reader := bytes.NewReader(raw)
	pdfReader, err := pdf.NewReader(reader, int64(len(raw)))
	if err != nil {
		return "", fmt.Errorf("read pdf: %w", err)
	}
	var out strings.Builder
	totalPages := pdfReader.NumPage()
	for i := 1; i <= totalPages; i++ {
		page := pdfReader.Page(i)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			return "", fmt.Errorf("extract pdf page %d: %w", i, err)
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		out.WriteString(fmt.Sprintf("[Page %d]\n%s\n\n", i, text))
	}
	return out.String(), nil
}

func splitText(text string, size int, overlap int) []string {
	runes := []rune(text)
	if len(runes) <= size {
		return []string{text}
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

func scoreChunks(prompt string, chunks []chunk) []chunk {
	terms := tokenSet(prompt)
	if len(terms) == 0 {
		return chunks
	}
	out := make([]chunk, len(chunks))
	copy(out, chunks)
	for i := range out {
		chunkTerms := tokenSet(out[i].Text)
		score := 0
		for term := range terms {
			if _, ok := chunkTerms[term]; ok {
				score++
			}
		}
		out[i].Score = score
	}
	return out
}

func tokenSet(s string) map[string]struct{} {
	out := make(map[string]struct{})
	var token strings.Builder
	flush := func() {
		if token.Len() < 3 {
			token.Reset()
			return
		}
		out[strings.ToLower(token.String())] = struct{}{}
		token.Reset()
	}
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			token.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
