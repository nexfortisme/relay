package llm

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/nexfortisme/relay/internal/prompts"
	"github.com/nexfortisme/relay/internal/tools"
)

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

type llmResponse struct {
	Content            string
	Thinking           string
	ToolCalls          []ToolCall
	Usage              TokenUsage
	RepetitionDetected bool
}

const (
	// maxToolCallRounds bounds how many request → tool-call → request cycles a
	// single generation may perform before we give up, preventing a model that
	// keeps requesting tools from looping forever.
	maxToolCallRounds = 8
	// maxSearchFetchFailures is how many consecutive failed web_search /
	// fetch_url calls are tolerated before responding with a canned
	// "unable to find" message instead of erroring the whole generation.
	maxSearchFetchFailures = 3
	maxRepetitionRetries   = 2
)

var repetitionRetryPrompt = prompts.MustLoad(prompts.RepetitionRetry)

// NewHTTPProvider builds a client for any OpenAI-compatible /chat/completions
// endpoint. apiKey may be empty for unauthenticated local servers.
func NewHTTPProvider(baseURL string, model string, apiKey string, responseTimeout time.Duration) *HTTPProvider {
	return NewHTTPProviderWithReasoningEffort(baseURL, model, apiKey, responseTimeout, "")
}

// NewHTTPProviderWithReasoningEffort is NewHTTPProvider with an explicit
// reasoning_effort request field — internal generations (titles, feed
// summaries) pass "none" to skip extended thinking on models that support it.

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

// GenerateStream runs one assistant generation — including any intermediate
// tool-call rounds — and emits TokenEvents on the returned channel. The channel
// is closed after a terminal event: either Done (with cumulative usage) or Err.
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

// generateWithTools drives the request loop: send the conversation, stream the
// reply, and — when the model requests tool calls — execute them, append the
// results as "tool" messages, and ask again. Repetition (a looping model) and
// repeated search failures are handled with bounded retries.
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

	for round := 0; round < maxToolCallRounds; round++ {
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
			isSearchFetch := isSearchOrFetchTool(toolCall.Function.Name)
			toolResult, err := executeToolCall(ctx, runtime, toolCall)
			if err != nil {
				if !isSearchFetch {
					return totalUsage, err
				}
				// Feed the failure back to the model as an error result so it
				// can retry with a different query/URL instead of aborting.
				toolResult = tools.Result{
					Name:    toolCall.Function.Name,
					Output:  err.Error(),
					IsError: true,
				}
			}
			if isSearchFetch {
				if toolResultFailed(toolResult) {
					searchFetchFailures++
					if searchFetchFailures >= maxSearchFetchFailures {
						return totalUsage, sendUnableToFind(ctx, out)
					}
				} else {
					searchFetchFailures = 0
				}
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

// CollectText drains a GenerateStream channel and returns the concatenated
// response tokens, discarding thinking output and usage. It is the shared
// helper for one-shot internal generations (conversation titles, feed names,
// feed summaries) where token-by-token streaming isn't needed.
func CollectText(stream <-chan TokenEvent) (string, error) {
	var builder strings.Builder
	for event := range stream {
		if event.Err != nil {
			return "", event.Err
		}
		builder.WriteString(event.Token)
	}
	return builder.String(), nil
}
