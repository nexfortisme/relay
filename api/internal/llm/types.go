package llm

import (
	"context"
	"net/http"
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

type TokenEvent struct {
	Token    string
	Thinking string
	Usage    *TokenUsage
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
	baseURL         string
	model           string
	apiKey          string
	reasoningEffort string
	client          *http.Client
	responseTimeout time.Duration
}
