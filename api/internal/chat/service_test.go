package chat

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/auth"
	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

func TestToLLMMessagesDoesNotIncludeStoredThinking(t *testing.T) {
	messages := []store.Message{
		{
			ID:         "msg-1",
			Role:       "assistant",
			Content:    "visible answer",
			LLMContent: "visible answer",
			Thinking:   "private reasoning that must not be resent",
		},
	}

	got := toLLMMessages(messages)
	if len(got) != 1 {
		t.Fatalf("expected 1 LLM message, got %d", len(got))
	}
	if got[0].ContentString() != "visible answer" {
		t.Fatalf("expected visible answer only, got %q", got[0].ContentString())
	}
}

func TestConversationTitleHelpersNormalizeAndClamp(t *testing.T) {
	if got := deriveTitle("  \n  "); got != "New chat" {
		t.Fatalf("expected fallback title, got %q", got)
	}

	longTitle := strings.Repeat("a", maxConversationTitleLength+1)
	if got := deriveTitle(longTitle); got != strings.Repeat("a", maxConversationTitleLength)+"..." {
		t.Fatalf("expected ellipsized title, got %q", got)
	}
	if got := clampConversationTitle("  Hello\nworld  "); got != "Hello world" {
		t.Fatalf("expected normalized title, got %q", got)
	}
	if got := clampConversationTitle(longTitle); got != strings.Repeat("a", maxConversationTitleLength) {
		t.Fatalf("expected clamped title, got %q", got)
	}
}

func TestPublishAssistantFinalEventIncludesFinalContentAndThinking(t *testing.T) {
	service := &Service{broker: NewBroker()}
	events, unsubscribe := service.Subscribe("conv-1")
	defer unsubscribe()

	service.publishAssistantFinalEvent("conv-1", "msg-1", "done", assistantFinalState{
		content:   "final answer",
		thinking:  "final reasoning",
		elapsedMs: 123,
		usage: llm.TokenUsage{
			InputTokens:     10,
			OutputTokens:    5,
			ReasoningTokens: 3,
			TotalTokens:     15,
		},
	})

	select {
	case event := <-events:
		if event.Type != "done" || event.MessageID != "msg-1" {
			t.Fatalf("unexpected event identity: %#v", event)
		}
		if event.Content != "final answer" {
			t.Fatalf("expected final content, got %q", event.Content)
		}
		if event.Thinking != "final reasoning" {
			t.Fatalf("expected final thinking, got %q", event.Thinking)
		}
		if event.ElapsedMs != 123 || event.InputTokens != 10 || event.OutputTokens != 5 || event.ReasoningTokens != 3 || event.TotalTokens != 15 {
			t.Fatalf("unexpected terminal metadata: %#v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for final event")
	}
}

func TestGenerateAssistantPassesUserContextToToolRuntime(t *testing.T) {
	var requests atomic.Int32
	llmServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		switch requests.Add(1) {
		case 1:
			_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"tool_calls":[{"index":0,"id":"call-1","type":"function","function":{"name":"capture_user","arguments":"{}"}}]}}]}`+"\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		default:
			_, _ = io.WriteString(w, `data: {"choices":[{"delta":{"content":"done"}}]}`+"\n\n")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		}
	}))
	defer llmServer.Close()

	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	now := time.Now().UTC()
	if _, err := st.CreateConversation(context.Background(), "conv-1", "user-1", "Test", now); err != nil {
		t.Fatalf("create conversation: %v", err)
	}
	if err := st.AppendMessage(context.Background(), store.Message{
		ID:             "assistant-1",
		ConversationID: "conv-1",
		Role:           "assistant",
		CreatedAt:      now,
	}); err != nil {
		t.Fatalf("append assistant message: %v", err)
	}

	runtime := &captureUserRuntime{}
	service := NewService(
		st,
		llmServer.URL,
		"test-model",
		runtime,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
		attachments.PromptOptions{},
		0,
	)
	service.responseTimeout = time.Second

	service.generateAssistant("user-1", "conv-1", "assistant-1", []llm.ChatMessage{
		{Role: "user", Content: "use the tool"},
	}, RuntimeSettings{LLMURL: llmServer.URL, LLMModel: "test-model"})

	if runtime.userID != "user-1" {
		t.Fatalf("expected tool runtime to receive user-1 context, got %q", runtime.userID)
	}
	messages, err := st.GetMessages(context.Background(), "conv-1")
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "done" {
		t.Fatalf("expected final assistant content to persist, got %#v", messages)
	}
}

type captureUserRuntime struct {
	userID string
}

func (r *captureUserRuntime) Definitions(context.Context) ([]tools.Definition, error) {
	return []tools.Definition{
		{
			Name:        "capture_user",
			Description: "Capture the authenticated user from context.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []any{},
			},
		},
	}, nil
}

func (r *captureUserRuntime) Execute(ctx context.Context, call tools.Call) (tools.Result, error) {
	userID, _ := auth.UserIDFromContext(ctx)
	r.userID = userID
	return tools.Result{
		Name:   call.Name,
		Output: map[string]string{"userId": userID},
	}, nil
}
