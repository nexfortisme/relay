package chat

import (
	"testing"

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
