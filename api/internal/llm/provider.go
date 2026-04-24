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
	"sort"
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
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
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

type ToolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
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

		if err := p.generateWithTools(ctx, messages, runtime, ch); err != nil {
			ch <- TokenEvent{Err: err}
			return
		}

		ch <- TokenEvent{Done: true}
	}()

	return ch
}

type chatRequest struct {
	Model      string        `json:"model"`
	Messages   []ChatMessage `json:"messages"`
	Stream     bool          `json:"stream"`
	Tools      []openAITool  `json:"tools,omitempty"`
	ToolChoice string        `json:"tool_choice,omitempty"`
}

type openAITool struct {
	Type     string             `json:"type"`
	Function openAIToolFunction `json:"function"`
}

type openAIToolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

const maxSearchFetchFailures = 3

func (p *HTTPProvider) generateWithTools(ctx context.Context, messages []ChatMessage, runtime tools.Runtime, out chan<- TokenEvent) error {
	defs, err := runtime.Definitions(ctx)
	if err != nil {
		return err
	}
	requestTools := openAITools(defs)
	currentMessages := append([]ChatMessage(nil), messages...)
	searchFetchFailures := 0

	for round := 0; round < 8; round++ {
		response, err := p.generateStream(ctx, currentMessages, requestTools)
		if err != nil {
			return err
		}

		if len(response.ToolCalls) == 0 {
			if response.Thinking != "" {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- TokenEvent{Thinking: response.Thinking}:
				}
			}
			if response.Content != "" {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case out <- TokenEvent{Token: response.Content}:
				}
			}
			return nil
		}

		preToolThinking := response.Thinking + response.Content
		if strings.TrimSpace(preToolThinking) != "" {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case out <- TokenEvent{Thinking: preToolThinking}:
			}
		}

		currentMessages = append(currentMessages, ChatMessage{
			Role:      "assistant",
			Content:   response.Content,
			ToolCalls: response.ToolCalls,
		})

		for _, toolCall := range response.ToolCalls {
			toolResult, err := executeToolCall(ctx, runtime, toolCall)
			if err != nil {
				if isSearchOrFetchTool(toolCall.Function.Name) {
					searchFetchFailures++
					if searchFetchFailures >= maxSearchFetchFailures {
						return sendUnableToFind(ctx, out)
					}
					toolResult = tools.Result{
						Name:    toolCall.Function.Name,
						Output:  err.Error(),
						IsError: true,
					}
				} else {
					return err
				}
			}
			if isSearchOrFetchTool(toolCall.Function.Name) && toolResultFailed(toolResult) {
				searchFetchFailures++
				if searchFetchFailures >= maxSearchFetchFailures {
					return sendUnableToFind(ctx, out)
				}
			}
			if isSearchOrFetchTool(toolCall.Function.Name) && !toolResultFailed(toolResult) {
				searchFetchFailures = 0
			}
			toolOutput, err := toolResultContent(toolResult)
			if err != nil {
				return err
			}
			currentMessages = append(currentMessages, ChatMessage{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: toolCall.ID,
			})
		}
	}

	return fmt.Errorf("too many tool call rounds")
}

type llmResponse struct {
	Content   string
	Thinking  string
	ToolCalls []ToolCall
}

func (p *HTTPProvider) generateStream(ctx context.Context, messages []ChatMessage, requestTools []openAITool) (llmResponse, error) {
	payload, err := json.Marshal(chatRequest{
		Model:      p.model,
		Messages:   messages,
		Stream:     true,
		Tools:      requestTools,
		ToolChoice: toolChoice(requestTools),
	})
	if err != nil {
		return llmResponse{}, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return llmResponse{}, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	res, err := p.client.Do(req)
	if err != nil {
		return llmResponse{}, fmt.Errorf("call llm endpoint: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return llmResponse{}, fmt.Errorf("llm request failed status=%d body=%s", res.StatusCode, string(body))
	}

	// Try SSE/line streaming first. If the provider returns standard JSON,
	// fall back to one-shot decoding.
	if response, streamed, err := p.consumeSSE(ctx, res.Body); err != nil {
		return llmResponse{}, err
	} else if streamed {
		return response, nil
	}

	return p.consumeSingleJSON(ctx, messages, requestTools)
}

func (p *HTTPProvider) consumeSSE(ctx context.Context, body io.Reader) (llmResponse, bool, error) {
	scanner := bufio.NewScanner(body)
	sawStream := false
	var content strings.Builder
	var thinkingBuilder strings.Builder
	toolCalls := map[int]*ToolCall{}
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
			return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), ToolCalls: orderedToolCalls(toolCalls)}, true, nil
		}

		token, thinking, calls, ok := extractChunk(raw)
		if ok && token != "" {
			content.WriteString(token)
		}
		if ok && thinking != "" {
			thinkingBuilder.WriteString(thinking)
		}
		for _, call := range calls {
			accumulateToolCall(toolCalls, call)
		}
		select {
		case <-ctx.Done():
			return llmResponse{}, true, ctx.Err()
		default:
		}
	}

	if err := scanner.Err(); err != nil {
		return llmResponse{}, sawStream, fmt.Errorf("read llm stream: %w", err)
	}
	return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), ToolCalls: orderedToolCalls(toolCalls)}, sawStream, nil
}

func (p *HTTPProvider) consumeSingleJSON(ctx context.Context, messages []ChatMessage, requestTools []openAITool) (llmResponse, error) {
	payload, err := json.Marshal(chatRequest{
		Model:      p.model,
		Messages:   messages,
		Stream:     false,
		Tools:      requestTools,
		ToolChoice: toolChoice(requestTools),
	})
	if err != nil {
		return llmResponse{}, fmt.Errorf("marshal fallback request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return llmResponse{}, fmt.Errorf("create fallback request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := p.client.Do(req)
	if err != nil {
		return llmResponse{}, fmt.Errorf("fallback llm call failed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return llmResponse{}, fmt.Errorf("fallback llm request failed status=%d body=%s", res.StatusCode, string(body))
	}

	var decoded struct {
		Choices []struct {
			Message struct {
				Content          string     `json:"content"`
				Reasoning        string     `json:"reasoning"`
				ReasoningContent string     `json:"reasoning_content"`
				ToolCalls        []ToolCall `json:"tool_calls"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&decoded); err != nil {
		return llmResponse{}, fmt.Errorf("decode fallback llm response: %w", err)
	}
	if len(decoded.Choices) == 0 {
		return llmResponse{}, fmt.Errorf("llm response did not include any choices")
	}

	message := decoded.Choices[0].Message
	return llmResponse{
		Content:   message.Content,
		Thinking:  firstNonEmpty(message.ReasoningContent, message.Reasoning),
		ToolCalls: message.ToolCalls,
	}, nil
}

func extractChunk(raw string) (string, string, []toolCallDelta, bool) {
	var openAIChunk struct {
		Choices []struct {
			Delta struct {
				Content          string `json:"content"`
				Reasoning        string `json:"reasoning"`
				ReasoningContent string `json:"reasoning_content"`
				ToolCalls        []struct {
					Index    int    `json:"index"`
					ID       string `json:"id"`
					Type     string `json:"type"`
					Function struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					} `json:"function"`
				} `json:"tool_calls"`
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
			return openAIChunk.Choices[0].Delta.Content, "", nil, true
		}
		calls := make([]toolCallDelta, 0, len(openAIChunk.Choices[0].Delta.ToolCalls))
		for _, call := range openAIChunk.Choices[0].Delta.ToolCalls {
			calls = append(calls, toolCallDelta{
				Index:     call.Index,
				ID:        call.ID,
				Type:      call.Type,
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			})
		}
		if len(calls) > 0 {
			return "", "", calls, true
		}
		reasoning := firstNonEmpty(
			openAIChunk.Choices[0].Delta.ReasoningContent,
			openAIChunk.Choices[0].Delta.Reasoning,
			openAIChunk.Choices[0].Message.ReasoningContent,
			openAIChunk.Choices[0].Message.Reasoning,
		)
		if reasoning != "" {
			return "", reasoning, nil, true
		}
		if openAIChunk.Choices[0].Message.Content != "" {
			return openAIChunk.Choices[0].Message.Content, "", nil, true
		}
	}

	var ollamaChunk struct {
		Message struct {
			Content  string `json:"content"`
			Thinking string `json:"thinking"`
		} `json:"message"`
		Response string `json:"response"`
		Thinking string `json:"thinking"`
	}
	if err := json.Unmarshal([]byte(raw), &ollamaChunk); err == nil {
		if ollamaChunk.Thinking != "" {
			return "", ollamaChunk.Thinking, nil, true
		}
		if ollamaChunk.Message.Thinking != "" {
			return "", ollamaChunk.Message.Thinking, nil, true
		}
		if ollamaChunk.Message.Content != "" {
			return ollamaChunk.Message.Content, "", nil, true
		}
		if ollamaChunk.Response != "" {
			return ollamaChunk.Response, "", nil, true
		}
	}

	return "", "", nil, false
}

type toolCallDelta struct {
	Index     int
	ID        string
	Type      string
	Name      string
	Arguments string
}

func openAITools(defs []tools.Definition) []openAITool {
	if len(defs) == 0 {
		return nil
	}
	out := make([]openAITool, 0, len(defs))
	for _, def := range defs {
		parameters := normalizeToolParameters(def.InputSchema)
		out = append(out, openAITool{
			Type: "function",
			Function: openAIToolFunction{
				Name:        def.Name,
				Description: def.Description,
				Parameters:  parameters,
			},
		})
	}
	return out
}

func normalizeToolParameters(schema map[string]any) map[string]any {
	if schema == nil {
		schema = map[string]any{}
	}
	normalized := make(map[string]any, len(schema)+2)
	for key, value := range schema {
		normalized[key] = value
	}
	normalized["type"] = "object"
	if _, ok := normalized["properties"].(map[string]any); !ok {
		normalized["properties"] = map[string]any{}
	}
	if _, ok := normalized["required"].([]any); !ok {
		if _, hasRequired := normalized["required"]; !hasRequired {
			normalized["required"] = []any{}
		}
	}
	return normalized
}

func toolChoice(requestTools []openAITool) string {
	if len(requestTools) == 0 {
		return ""
	}
	return "auto"
}

func sendUnableToFind(ctx context.Context, out chan<- TokenEvent) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- TokenEvent{Token: "I can't find that out right now."}:
		return nil
	}
}

func isSearchOrFetchTool(name string) bool {
	return name == "web_search" || name == "fetch_url" || name == "fetch_urls"
}

func toolResultFailed(result tools.Result) bool {
	if result.IsError {
		return true
	}
	if text, ok := result.Output.(string); ok {
		return strings.TrimSpace(text) == ""
	}
	return result.Output == nil
}

func executeToolCall(ctx context.Context, runtime tools.Runtime, toolCall ToolCall) (tools.Result, error) {
	args := map[string]any{}
	if strings.TrimSpace(toolCall.Function.Arguments) != "" {
		if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
			return tools.Result{}, fmt.Errorf("parse tool arguments for %s: %w", toolCall.Function.Name, err)
		}
	}
	result, err := runtime.Execute(ctx, tools.Call{
		Name:      toolCall.Function.Name,
		Arguments: args,
	})
	if err != nil {
		return tools.Result{}, err
	}
	return result, nil
}

func toolResultContent(result tools.Result) (string, error) {
	if _, ok := result.Output.(string); ok {
		return result.Output.(string), nil
	}
	encoded, err := json.Marshal(result.Output)
	if err != nil {
		return "", fmt.Errorf("marshal tool result for %s: %w", result.Name, err)
	}
	return string(encoded), nil
}

func accumulateToolCall(calls map[int]*ToolCall, delta toolCallDelta) {
	call, ok := calls[delta.Index]
	if !ok {
		call = &ToolCall{Type: "function"}
		calls[delta.Index] = call
	}
	if delta.ID != "" {
		call.ID = delta.ID
	}
	if delta.Type != "" {
		call.Type = delta.Type
	}
	if delta.Name != "" {
		call.Function.Name += delta.Name
	}
	if delta.Arguments != "" {
		call.Function.Arguments += delta.Arguments
	}
}

func orderedToolCalls(calls map[int]*ToolCall) []ToolCall {
	if len(calls) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(calls))
	for index := range calls {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	out := make([]ToolCall, 0, len(indexes))
	for _, index := range indexes {
		call := *calls[index]
		if call.ID == "" {
			call.ID = fmt.Sprintf("tool-call-%d", index)
		}
		if call.Type == "" {
			call.Type = "function"
		}
		out = append(out, call)
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
