package prompts

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Key identifies a prompt resource file under resources/api/.
type Key string

const (
	CiteSources             Key = "cite_sources"
	SuggestTitle            Key = "suggest_title"
	FallbackResponse        Key = "fallback_response"
	UnableToFind            Key = "unable_to_find"
	AttachmentImagesHeader  Key = "attachment_images_header"
	AttachmentImagesSkipped Key = "attachment_images_skipped"
	AttachmentDocsInline    Key = "attachment_docs_inline"
	AttachmentDocsRAG       Key = "attachment_docs_rag"
)

// Load reads and returns the prompt text for the given key from resources/api/.
// Leading markdown headings (lines starting with #) and surrounding blank lines
// are stripped so files can have human-readable titles without affecting the
// content sent to the LLM.
func Load(key Key) (string, error) {
	path := filepath.Join(resourcesDir(), string(key)+".md")
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("prompts: load %q: %w", key, err)
	}
	return strings.TrimSpace(stripHeadings(string(data))), nil
}

// stripHeadings removes leading markdown heading lines (starting with #) and
// any blank lines that immediately follow them.
func stripHeadings(s string) string {
	lines := strings.Split(s, "\n")
	start := 0
	for start < len(lines) {
		trimmed := strings.TrimSpace(lines[start])
		if strings.HasPrefix(trimmed, "#") || trimmed == "" {
			start++
			continue
		}
		break
	}
	return strings.Join(lines[start:], "\n")
}

// MustLoad loads a prompt and panics if the file cannot be read.
// Intended for use at service initialization time.
func MustLoad(key Key) string {
	s, err := Load(key)
	if err != nil {
		panic(err)
	}
	return s
}

// resourcesDir returns the path to the resources/api directory. It checks an
// env var override first, then probes paths relative to the working directory
// to support both running from the project root and from api/ (via air).
func resourcesDir() string {
	if dir := os.Getenv("RESOURCES_PATH"); dir != "" {
		return dir
	}
	for _, candidate := range []string{
		filepath.Join("resources", "api"),
		filepath.Join("..", "resources", "api"),
	} {
		if fi, err := os.Stat(candidate); err == nil && fi.IsDir() {
			return candidate
		}
	}
	return filepath.Join("..", "resources", "api")
}
