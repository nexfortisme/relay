package attachments

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

func PersistUploadedFiles(baseDir string, conversationID string, messageID string, files []UploadedFile) ([]string, error) {
	if len(files) == 0 {
		return nil, nil
	}
	targetDir := filepath.Join(baseDir, "files", conversationID, messageID)
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return nil, fmt.Errorf("create upload directory: %w", err)
	}

	names := make([]string, 0, len(files))
	for idx, file := range files {
		safeName := sanitizeFilename(file.Name)
		target := filepath.Join(targetDir, fmt.Sprintf("%02d-%s", idx+1, safeName))
		if err := os.WriteFile(target, file.Data, 0o644); err != nil {
			return nil, fmt.Errorf("persist %s: %w", file.Name, err)
		}
		names = append(names, file.Name)
	}
	return names, nil
}

func sanitizeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	if name == "" || name == "." || name == ".." {
		return "upload.bin"
	}
	var b strings.Builder
	for _, r := range name {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' {
			b.WriteRune(r)
			continue
		}
		b.WriteRune('_')
	}
	out := strings.Trim(b.String(), "._")
	if out == "" {
		return "upload.bin"
	}
	return out
}

func ResolveUploadedFilePath(baseDir string, conversationID string, messageID string, index int, originalName string) (string, string, error) {
	if index < 0 {
		return "", "", fmt.Errorf("invalid attachment index")
	}
	safeName := sanitizeFilename(originalName)
	storedName := fmt.Sprintf("%02d-%s", index+1, safeName)
	target := filepath.Join(baseDir, "files", conversationID, messageID, storedName)

	cleanBase := filepath.Clean(baseDir)
	cleanTarget := filepath.Clean(target)
	rel, err := filepath.Rel(cleanBase, cleanTarget)
	if err != nil {
		return "", "", fmt.Errorf("resolve attachment path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", "", fmt.Errorf("invalid attachment path")
	}
	return cleanTarget, storedName, nil
}

func ParseAttachmentIndex(raw string) (int, error) {
	index, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid attachment index")
	}
	if index < 0 {
		return 0, fmt.Errorf("invalid attachment index")
	}
	return index, nil
}
