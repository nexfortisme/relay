package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/nexfortisme/relay/internal/tools"
)

type ContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *ImageURLData `json:"image_url,omitempty"`
}

type ImageURLData struct {
	URL string `json:"url"`
}

type ChatMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

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

type TokenEvent struct {
	Token    string
	Thinking string
	Done     bool
	Err      error
}

type Provider interface {
	GenerateStream(ctx context.Context, messages []ChatMessage, runtime tools.Runtime) <-chan TokenEvent
}

type HTTPProvider struct {
	baseURL string
	model   string
	client  *http.Client
}

func NewHTTPProvider(baseURL string, model string) *HTTPProvider {
	return &HTTPProvider{
		baseURL: strings.TrimSuffix(baseURL, "/"),
		model:   model,
		client: &http.Client{
			Timeout: 45 * time.Second,
		},
	}
}

func (p *HTTPProvider) GenerateStream(ctx context.Context, messages []ChatMessage, runtime tools.Runtime) <-chan TokenEvent {
	ch := make(chan TokenEvent)

	go func() {
		defer close(ch)

		_, _ = runtime.Definitions(ctx)

		if err := p.generateStream(ctx, messages, ch); err != nil {
			ch <- TokenEvent{Err: err}
			return
		}

		ch <- TokenEvent{Done: true}
	}()

	return ch
}

type chatRequest struct {
	Model    string        `json:"model"`
	Messages []ChatMessage `json:"messages"`
	Stream   bool          `json:"stream"`
}

func (p *HTTPProvider) generateStream(ctx context.Context, messages []ChatMessage, out chan<- TokenEvent) error {
	payload, err := json.Marshal(chatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	res, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("call llm endpoint: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("llm request failed status=%d body=%s", res.StatusCode, string(body))
	}

	// Try SSE/line streaming first. If the provider returns standard JSON,
	// fall back to one-shot decoding.
	if streamed, err := p.consumeSSE(ctx, res.Body, out); err != nil {
		return err
	} else if streamed {
		return nil
	}

	return p.consumeSingleJSON(ctx, messages, out)
}

func (p *HTTPProvider) consumeSSE(ctx context.Context, body io.Reader, out chan<- TokenEvent) (bool, error) {
	scanner := bufio.NewScanner(body)
	sawStream := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "event:") {
			sawStream = true
			continue
		}

		raw := line
		if strings.HasPrefix(raw, "data:") {
			sawStream = true
			raw = strings.TrimSpace(strings.TrimPrefix(raw, "data:"))
		}

		if raw == "[DONE]" {
			return true, nil
		}

		token, thinking, ok := extractToken(raw)
		if ok && token != "" {
			select {
			case <-ctx.Done():
				return true, ctx.Err()
			case out <- TokenEvent{Token: token}:
			}
		}
		if ok && thinking != "" {
			select {
			case <-ctx.Done():
				return true, ctx.Err()
			case out <- TokenEvent{Thinking: thinking}:
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return sawStream, fmt.Errorf("read llm stream: %w", err)
	}
	return sawStream, nil
}

func (p *HTTPProvider) consumeSingleJSON(ctx context.Context, messages []ChatMessage, out chan<- TokenEvent) error {
	payload, err := json.Marshal(chatRequest{
		Model:    p.model,
		Messages: messages,
		Stream:   false,
	})
	if err != nil {
		return fmt.Errorf("marshal fallback request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return fmt.Errorf("create fallback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("fallback llm call failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return fmt.Errorf("fallback llm request failed status=%d body=%s", res.StatusCode, string(body))
	}

	var decoded struct {
		Choices []struct {
			Message ChatMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		return fmt.Errorf("decode fallback llm response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return fmt.Errorf("llm response did not include any choices")
	}

	out <- TokenEvent{Token: decoded.Choices[0].Message.ContentString()}
	return nil
}

func extractToken(raw string) (string, string, bool) {
	var openAIChunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				Reasoning        string `json:"reasoning"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"delta"`
			Message struct {
				Content          string `json:"content"`
				Reasoning        string `json:"reasoning"`
				ReasoningContent string `json:"reasoning_content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(raw), &openAIChunk); err == nil && len(openAIChunk.Choices) > 0 {
		if openAIChunk.Choices[0].Delta.Content != "" {
			return openAIChunk.Choices[0].Delta.Content, "", true
		}
		reasoning := firstNonEmpty(
			openAIChunk.Choices[0].Delta.ReasoningContent,
			openAIChunk.Choices[0].Delta.Reasoning,
			openAIChunk.Choices[0].Message.ReasoningContent,
			openAIChunk.Choices[0].Message.Reasoning,
		)
		if reasoning != "" {
			return "", reasoning, true
		}
		if openAIChunk.Choices[0].Message.Content != "" {
			return openAIChunk.Choices[0].Message.Content, "", true
		}
	}

	var ollamaChunk struct {
		Message struct {
			Content string `json:"content"`
			Thinking string `json:"thinking"`
		} `json:"message"`
		Response string `json:"response"`
		Thinking string `json:"thinking"`
	}
	if err := json.Unmarshal([]byte(raw), &ollamaChunk); err == nil {
		if ollamaChunk.Thinking != "" {
			return "", ollamaChunk.Thinking, true
		}
		if ollamaChunk.Message.Thinking != "" {
			return "", ollamaChunk.Message.Thinking, true
		}
		if ollamaChunk.Message.Content != "" {
			return ollamaChunk.Message.Content, "", true
		}
		if ollamaChunk.Response != "" {
			return ollamaChunk.Response, "", true
		}
	}

	return "", "", false
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
