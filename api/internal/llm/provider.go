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

	"github.com/nexfortisme/relay/internal/prompts"
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
	Usage    *TokenUsage
	Done     bool
	Err      error
}

type TokenUsage struct {
	InputTokens     int `json:"inputTokens,omitempty"`
	OutputTokens    int `json:"outputTokens,omitempty"`
	ReasoningTokens int `json:"reasoningTokens,omitempty"`
	TotalTokens     int `json:"totalTokens,omitempty"`
}

func (u TokenUsage) IsZero() bool {
	return u.InputTokens == 0 && u.OutputTokens == 0 && u.ReasoningTokens == 0 && u.TotalTokens == 0
}

func (u *TokenUsage) Add(other TokenUsage) {
	if other.IsZero() {
		return
	}
	u.InputTokens += other.InputTokens
	u.OutputTokens += other.OutputTokens
	u.ReasoningTokens += other.ReasoningTokens
	u.TotalTokens += other.TotalTokens
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
	baseURL         string
	model           string
	apiKey          string
	reasoningEffort string
	client          *http.Client
	responseTimeout time.Duration
}

func NewHTTPProvider(baseURL string, model string, apiKey string, responseTimeout time.Duration) *HTTPProvider {
	return NewHTTPProviderWithReasoningEffort(baseURL, model, apiKey, responseTimeout, "")
}

func NewHTTPProviderWithReasoningEffort(baseURL string, model string, apiKey string, responseTimeout time.Duration, reasoningEffort string) *HTTPProvider {
	return &HTTPProvider{
		baseURL:         strings.TrimSuffix(baseURL, "/"),
		model:           model,
		apiKey:          strings.TrimSpace(apiKey),
		reasoningEffort: strings.TrimSpace(reasoningEffort),
		responseTimeout: responseTimeout,
		client:          &http.Client{},
	}
}

func (p *HTTPProvider) GenerateStream(ctx context.Context, messages []ChatMessage, runtime tools.Runtime) <-chan TokenEvent {
	ch := make(chan TokenEvent)

	go func() {
		defer close(ch)

		usage, err := p.generateWithTools(ctx, messages, runtime, ch)
		if err != nil {
			ch <- TokenEvent{Err: err}
			return
		}

		ch <- TokenEvent{Done: true, Usage: &usage}
	}()

	return ch
}

type chatRequest struct {
	Model           string         `json:"model"`
	Messages        []ChatMessage  `json:"messages"`
	Stream          bool           `json:"stream"`
	StreamOptions   *streamOptions `json:"stream_options,omitempty"`
	Tools           []openAITool   `json:"tools,omitempty"`
	ToolChoice      string         `json:"tool_choice,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
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

const (
	maxSearchFetchFailures = 3
	maxRepetitionRetries   = 2
)

const repetitionRetryPrompt = "Your previous response was stopped because it began repeating itself. Continue from exactly where the assistant message left off, without restating or repeating any text that has already been provided."

func (p *HTTPProvider) generateWithTools(ctx context.Context, messages []ChatMessage, runtime tools.Runtime, out chan<- TokenEvent) (TokenUsage, error) {
	defs, err := runtime.Definitions(ctx)
	if err != nil {
		return TokenUsage{}, err
	}
	requestTools := openAITools(defs)
	currentMessages := append([]ChatMessage(nil), messages...)
	searchFetchFailures := 0
	repetitionRetries := 0
	totalUsage := TokenUsage{}

	for round := 0; round < 8; round++ {
		respCtx, respCancel := context.WithTimeout(ctx, p.responseTimeout)
		response, didStream, err := p.generateStream(ctx, respCtx, currentMessages, requestTools, out)
		respCancel()
		if err != nil {
			return totalUsage, err
		}
		totalUsage.Add(response.Usage)

		if response.RepetitionDetected {
			if repetitionRetries >= maxRepetitionRetries {
				if strings.TrimSpace(response.Content) != "" {
					return totalUsage, nil
				}
				return totalUsage, fmt.Errorf("llm response repeated before producing usable content")
			}
			repetitionRetries++
			currentMessages = retryMessagesAfterRepetition(currentMessages, response.Content)
			continue
		}

		if len(response.ToolCalls) == 0 {
			if !didStream {
				// SSE not available — send accumulated response manually.
				if response.Thinking != "" {
					select {
					case <-ctx.Done():
						return totalUsage, ctx.Err()
					case out <- TokenEvent{Thinking: response.Thinking}:
					}
				}
				if response.Content != "" {
					select {
					case <-ctx.Done():
						return totalUsage, ctx.Err()
					case out <- TokenEvent{Token: response.Content}:
					}
				}
			}
			// When didStream, tokens were already forwarded in consumeSSE.
			return totalUsage, nil
		}

		if !didStream {
			// Only send pre-tool content as thinking when it wasn't already streamed.
			preToolThinking := response.Thinking + response.Content
			if strings.TrimSpace(preToolThinking) != "" {
				select {
				case <-ctx.Done():
					return totalUsage, ctx.Err()
				case out <- TokenEvent{Thinking: preToolThinking}:
				}
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
						return totalUsage, sendUnableToFind(ctx, out)
					}
					toolResult = tools.Result{
						Name:    toolCall.Function.Name,
						Output:  err.Error(),
						IsError: true,
					}
				} else {
					return totalUsage, err
				}
			}
			if isSearchOrFetchTool(toolCall.Function.Name) && toolResultFailed(toolResult) {
				searchFetchFailures++
				if searchFetchFailures >= maxSearchFetchFailures {
					return totalUsage, sendUnableToFind(ctx, out)
				}
			}
			if isSearchOrFetchTool(toolCall.Function.Name) && !toolResultFailed(toolResult) {
				searchFetchFailures = 0
			}
			toolOutput, err := toolResultContent(toolResult)
			if err != nil {
				return totalUsage, err
			}
			currentMessages = append(currentMessages, ChatMessage{
				Role:       "tool",
				Content:    toolOutput,
				ToolCallID: toolCall.ID,
			})
		}
	}

	return totalUsage, fmt.Errorf("too many tool call rounds")
}

type llmResponse struct {
	Content            string
	Thinking           string
	ToolCalls          []ToolCall
	Usage              TokenUsage
	RepetitionDetected bool
}

func (p *HTTPProvider) generateStream(parentCtx, respCtx context.Context, messages []ChatMessage, requestTools []openAITool, out chan<- TokenEvent) (llmResponse, bool, error) {
	payload, err := json.Marshal(chatRequest{
		Model:           p.model,
		Messages:        messages,
		Stream:          true,
		StreamOptions:   &streamOptions{IncludeUsage: true},
		Tools:           requestTools,
		ToolChoice:      toolChoice(requestTools),
		ReasoningEffort: p.reasoningEffort,
	})
	if err != nil {
		return llmResponse{}, false, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		respCtx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return llmResponse{}, false, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	p.applyAuth(req)

	res, err := p.client.Do(req)
	if err != nil {
		return llmResponse{}, false, fmt.Errorf("call llm endpoint: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode >= 400 {
		body, _ := io.ReadAll(res.Body)
		return llmResponse{}, false, fmt.Errorf("llm request failed status=%d body=%s", res.StatusCode, string(body))
	}

	// Try SSE/line streaming first. If the provider returns standard JSON,
	// fall back to one-shot decoding.
	if response, streamed, err := p.consumeSSE(parentCtx, respCtx, res.Body, out); err != nil {
		return llmResponse{}, true, err
	} else if streamed {
		return response, true, nil
	}

	response, err := p.consumeSingleJSON(respCtx, messages, requestTools)
	return response, false, err
}

func (p *HTTPProvider) consumeSSE(parentCtx, respCtx context.Context, body io.Reader, out chan<- TokenEvent) (llmResponse, bool, error) {
	scanner := bufio.NewScanner(body)
	sawStream := false
	var content strings.Builder
	var thinkingBuilder strings.Builder
	contentRepetition := repetitionDetector{}
	thinkingRepetition := repetitionDetector{}
	totalUsage := TokenUsage{}
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
			return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), ToolCalls: orderedToolCalls(toolCalls), Usage: totalUsage}, true, nil
		}

		token, thinking, calls, usage, ok := extractChunk(raw)
		if usage != nil {
			totalUsage.Add(*usage)
		}
		if ok && token != "" {
			if !contentRepetition.Accept(token) {
				return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), Usage: totalUsage, RepetitionDetected: true}, true, nil
			}
			content.WriteString(token)
			select {
			case <-parentCtx.Done():
				return llmResponse{}, true, parentCtx.Err()
			case <-respCtx.Done():
				select {
				case out <- TokenEvent{Token: token}:
				default:
				}
				return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), Usage: totalUsage}, true, nil
			case out <- TokenEvent{Token: token}:
			}
		}
		if ok && thinking != "" {
			if !thinkingRepetition.Accept(thinking) {
				return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), Usage: totalUsage, RepetitionDetected: true}, true, nil
			}
			thinkingBuilder.WriteString(thinking)
			select {
			case <-parentCtx.Done():
				return llmResponse{}, true, parentCtx.Err()
			case <-respCtx.Done():
				select {
				case out <- TokenEvent{Thinking: thinking}:
				default:
				}
				return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), Usage: totalUsage}, true, nil
			case out <- TokenEvent{Thinking: thinking}:
			}
		}
		for _, call := range calls {
			accumulateToolCall(toolCalls, call)
		}
		select {
		case <-parentCtx.Done():
			return llmResponse{}, true, parentCtx.Err()
		case <-respCtx.Done():
			return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), Usage: totalUsage}, true, nil
		default:
		}
	}

	if err := scanner.Err(); err != nil {
		if parentCtx.Err() != nil {
			return llmResponse{}, sawStream, parentCtx.Err()
		}
		if respCtx.Err() != nil {
			return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), Usage: totalUsage}, sawStream, nil
		}
		return llmResponse{}, sawStream, fmt.Errorf("read llm stream: %w", err)
	}
	return llmResponse{Content: content.String(), Thinking: thinkingBuilder.String(), ToolCalls: orderedToolCalls(toolCalls), Usage: totalUsage}, sawStream, nil
}

func (p *HTTPProvider) consumeSingleJSON(ctx context.Context, messages []ChatMessage, requestTools []openAITool) (llmResponse, error) {
	payload, err := json.Marshal(chatRequest{
		Model:           p.model,
		Messages:        messages,
		Stream:          false,
		Tools:           requestTools,
		ToolChoice:      toolChoice(requestTools),
		ReasoningEffort: p.reasoningEffort,
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
	p.applyAuth(req)

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
		Usage apiUsage `json:"usage"`
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
		Usage:     decoded.Usage.tokenUsage(),
	}, nil
}

func (p *HTTPProvider) applyAuth(req *http.Request) {
	if p.apiKey == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
}

func extractChunk(raw string) (string, string, []toolCallDelta, *TokenUsage, bool) {
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
		Usage apiUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(raw), &openAIChunk); err == nil && len(openAIChunk.Choices) > 0 {
		usage := openAIChunk.Usage.tokenUsagePtr()
		if openAIChunk.Choices[0].Delta.Content != "" {
			return openAIChunk.Choices[0].Delta.Content, "", nil, usage, true
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
			return "", "", calls, usage, true
		}
		reasoning := firstNonEmpty(
			openAIChunk.Choices[0].Delta.ReasoningContent,
			openAIChunk.Choices[0].Delta.Reasoning,
			openAIChunk.Choices[0].Message.ReasoningContent,
			openAIChunk.Choices[0].Message.Reasoning,
		)
		if reasoning != "" {
			return "", reasoning, nil, usage, true
		}
		if openAIChunk.Choices[0].Message.Content != "" {
			return openAIChunk.Choices[0].Message.Content, "", nil, usage, true
		}
		if usage != nil {
			return "", "", nil, usage, true
		}
	}

	var openAIUsageOnlyChunk struct {
		Usage apiUsage `json:"usage"`
	}
	if err := json.Unmarshal([]byte(raw), &openAIUsageOnlyChunk); err == nil {
		if usage := openAIUsageOnlyChunk.Usage.tokenUsagePtr(); usage != nil {
			return "", "", nil, usage, true
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
			return "", ollamaChunk.Thinking, nil, nil, true
		}
		if ollamaChunk.Message.Thinking != "" {
			return "", ollamaChunk.Message.Thinking, nil, nil, true
		}
		if ollamaChunk.Message.Content != "" {
			return ollamaChunk.Message.Content, "", nil, nil, true
		}
		if ollamaChunk.Response != "" {
			return ollamaChunk.Response, "", nil, nil, true
		}
	}

	return "", "", nil, nil, false
}

type apiUsage struct {
	PromptTokens            int `json:"prompt_tokens"`
	CompletionTokens        int `json:"completion_tokens"`
	TotalTokens             int `json:"total_tokens"`
	CompletionTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

func (u apiUsage) tokenUsagePtr() *TokenUsage {
	usage := u.tokenUsage()
	if usage.IsZero() {
		return nil
	}
	return &usage
}

func (u apiUsage) tokenUsage() TokenUsage {
	reasoningTokens := max(0, u.CompletionTokensDetails.ReasoningTokens)
	outputTokens := max(0, u.CompletionTokens-reasoningTokens)
	totalTokens := u.PromptTokens + outputTokens
	if totalTokens == 0 && u.TotalTokens > 0 {
		totalTokens = max(0, u.TotalTokens-reasoningTokens)
	}
	return TokenUsage{
		InputTokens:     max(0, u.PromptTokens),
		OutputTokens:    outputTokens,
		ReasoningTokens: reasoningTokens,
		TotalTokens:     totalTokens,
	}
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

type repetitionDetector struct {
	tail string
}

const (
	repetitionWindowRunes       = 12000
	repetitionWindowWords       = 900
	repetitionShortMinUnitWords = 6
	repetitionShortMaxUnitWords = 80
	repetitionShortRepeatCount  = 3
	repetitionShortMinUnitChars = 30
	repetitionLongMinUnitWords  = 40
	repetitionLongMaxUnitWords  = 300
	repetitionLongRepeatCount   = 2
	repetitionLongMinUnitChars  = 240
)

func (d *repetitionDetector) Accept(next string) bool {
	if next == "" {
		return true
	}
	candidate := d.tail + next
	if hasRepeatedSuffix(candidate) {
		return false
	}
	d.tail = tailRunes(candidate, repetitionWindowRunes)
	return true
}

func hasRepeatedSuffix(text string) bool {
	words := repetitionWords(text)
	if len(words) > repetitionWindowWords {
		words = words[len(words)-repetitionWindowWords:]
	}

	return hasRepeatedWordSuffix(words, repetitionShortMinUnitWords, repetitionShortMaxUnitWords, repetitionShortRepeatCount, repetitionShortMinUnitChars) ||
		hasRepeatedWordSuffix(words, repetitionLongMinUnitWords, repetitionLongMaxUnitWords, repetitionLongRepeatCount, repetitionLongMinUnitChars)
}

func hasRepeatedWordSuffix(words []string, minUnitWords int, maxUnitWords int, repeatCount int, minUnitChars int) bool {
	if len(words) < minUnitWords*repeatCount {
		return false
	}

	maxUnitWords = min(maxUnitWords, len(words)/repeatCount)
	for unitWords := minUnitWords; unitWords <= maxUnitWords; unitWords++ {
		if !repeatedWordSuffix(words, unitWords, repeatCount) {
			continue
		}
		unitText := strings.Join(words[len(words)-unitWords:], " ")
		if len(unitText) >= minUnitChars {
			return true
		}
	}
	return false
}

func repetitionWords(text string) []string {
	return strings.Fields(strings.ToLower(text))
}

func repeatedWordSuffix(words []string, unitWords int, repeatCount int) bool {
	end := len(words)
	baseStart := end - unitWords
	for repeat := 2; repeat <= repeatCount; repeat++ {
		start := end - unitWords*repeat
		for offset := 0; offset < unitWords; offset++ {
			if words[start+offset] != words[baseStart+offset] {
				return false
			}
		}
	}
	return true
}

func tailRunes(text string, maxRunes int) string {
	runes := []rune(text)
	if len(runes) <= maxRunes {
		return text
	}
	return string(runes[len(runes)-maxRunes:])
}

func sendUnableToFind(ctx context.Context, out chan<- TokenEvent) error {
	msg, _ := prompts.Load(prompts.UnableToFind)
	if msg == "" {
		msg = "I can't find that out right now."
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case out <- TokenEvent{Token: msg}:
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
