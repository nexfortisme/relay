package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

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
