package attachments

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"mime"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/ledongthuc/pdf"

	"github.com/nexfortisme/relay/internal/prompts"
)

const (
	defaultMaxFileBytes  = 50 * 1024 * 1024    // 50MB
	defaultMaxImageBytes = 15 * 1024 * 1024    // 15MB
	inlineCharBudget     = 12000               // 12000 characters
	chunkSizeRunes       = 1200                // 1200 characters
	chunkOverlapRunes    = chunkSizeRunes / 10 // 120 characters
	maxReturnedChunks    = 20                  // 20 chunks
)

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
			imageData, imageContentType, err := prepareImageForPrompt(file.Data, contentType, options.MaxImageBytes)
			if err != nil {
				skippedImages = append(skippedImages, file.Name)
				continue
			}
			images = append(images, imageBlock(file.Name, imageContentType, imageData))
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
		imagesHeader, err := prompts.Load(prompts.AttachmentImagesHeader)
		if err != nil {
			return "", err
		}
		builder.WriteString(imagesHeader)
		builder.WriteString("\n\n")
		for _, block := range images {
			builder.WriteString(block)
			builder.WriteString("\n\n")
		}
	}
	if len(skippedImages) > 0 {
		skippedTpl, err := prompts.Load(prompts.AttachmentImagesSkipped)
		if err != nil {
			return "", err
		}
		builder.WriteString(fmt.Sprintf(skippedTpl+"\n\n", options.MaxImageBytes/1024, strings.Join(skippedImages, ", ")))
	}

	if len(documents) == 0 {
		return strings.TrimSpace(builder.String()), nil
	}

	if totalDocChars <= inlineCharBudget {
		docsInline, err := prompts.Load(prompts.AttachmentDocsInline)
		if err != nil {
			return "", err
		}
		builder.WriteString(docsInline)
		builder.WriteString("\n\n")
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

	docsRAG, err := prompts.Load(prompts.AttachmentDocsRAG)
	if err != nil {
		return "", err
	}
	builder.WriteString(docsRAG)
	builder.WriteString("\n\n")
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
	case ".gif":
		return "image/gif"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
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

func prepareImageForPrompt(raw []byte, contentType string, maxBytes int) ([]byte, string, error) {
	if maxBytes <= 0 || len(raw) <= maxBytes {
		return raw, contentType, nil
	}

	src, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, "", fmt.Errorf("decode image: %w", err)
	}

	bounds := src.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()
	if width <= 0 || height <= 0 {
		return nil, "", fmt.Errorf("decode image: empty image")
	}

	for scale := 1.0; scale >= 0.2; scale *= 0.85 {
		resized := resizeNearest(src, max(1, int(float64(width)*scale)), max(1, int(float64(height)*scale)))
		for quality := 85; quality >= 45; quality -= 10 {
			var out bytes.Buffer
			if err := jpeg.Encode(&out, resized, &jpeg.Options{Quality: quality}); err != nil {
				return nil, "", fmt.Errorf("encode image: %w", err)
			}
			if out.Len() <= maxBytes {
				return out.Bytes(), "image/jpeg", nil
			}
		}
	}

	return nil, "", fmt.Errorf("image exceeds max size of %d KB after compression", maxBytes/1024)
}

func resizeNearest(src image.Image, width int, height int) *image.RGBA {
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	for y := 0; y < height; y++ {
		srcY := srcBounds.Min.Y + y*srcHeight/height
		for x := 0; x < width; x++ {
			srcX := srcBounds.Min.X + x*srcWidth/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
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
