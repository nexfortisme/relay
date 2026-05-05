package llm

import (
	"regexp"
	"strings"
)

func (m ChatMessage) ContentString() string {
	if s, ok := m.Content.(string); ok {
		return s
	}
	return ""
}

// imageDataURLRe matches markdown image syntax with data URLs: ![alt](data:...)
var imageDataURLRe = regexp.MustCompile(`!\[[^\]]*\]\((data:[^)]+)\)`)

// ParseContent converts a prompt string into either a plain string or a []ContentPart
// slice for multimodal LLM requests when image data URLs are present.
func ParseContent(content string) interface{} {
	matches := imageDataURLRe.FindAllStringSubmatchIndex(content, -1)
	if len(matches) == 0 {
		return content
	}
	var parts []ContentPart
	lastEnd := 0
	for _, match := range matches {
		if textBefore := content[lastEnd:match[0]]; strings.TrimSpace(textBefore) != "" {
			parts = append(parts, ContentPart{Type: "text", Text: textBefore})
		}
		parts = append(parts, ContentPart{
			Type:     "image_url",
			ImageURL: &ImageURLData{URL: content[match[2]:match[3]]},
		})
		lastEnd = match[1]
	}
	if lastEnd < len(content) {
		if remaining := strings.TrimSpace(content[lastEnd:]); remaining != "" {
			parts = append(parts, ContentPart{Type: "text", Text: remaining})
		}
	}
	return parts
}

func retryMessagesAfterRepetition(messages []ChatMessage, partialContent string) []ChatMessage {
	retryMessages := trimTrailingEmptyAssistant(messages)
	if strings.TrimSpace(partialContent) != "" {
		retryMessages = append(retryMessages, ChatMessage{
			Role:    "assistant",
			Content: partialContent,
		})
	}
	retryMessages = append(retryMessages, ChatMessage{
		Role:    "user",
		Content: repetitionRetryPrompt,
	})
	return retryMessages
}

func trimTrailingEmptyAssistant(messages []ChatMessage) []ChatMessage {
	trimmed := append([]ChatMessage(nil), messages...)
	for len(trimmed) > 0 {
		last := trimmed[len(trimmed)-1]
		if last.Role != "assistant" || strings.TrimSpace(last.ContentString()) != "" || len(last.ToolCalls) > 0 {
			break
		}
		trimmed = trimmed[:len(trimmed)-1]
	}
	return trimmed
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
