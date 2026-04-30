package chat

import (
	"strings"
	"testing"
	"time"

	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/store"
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
