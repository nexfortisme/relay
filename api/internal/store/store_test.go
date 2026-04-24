package store

import (
	"context"
	"testing"
	"time"
)

func TestStoreConversationAndMessages(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	conversation, err := st.CreateConversation(ctx, "conv-1", "Test", time.Now())
	if err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	msg := Message{
		ID:             "msg-1",
		ConversationID: conversation.ID,
		Role:           "user",
		Content:        "hello",
		CreatedAt:      time.Now(),
	}
	if err := st.AppendMessage(ctx, msg); err != nil {
		t.Fatalf("append message: %v", err)
	}

	if err := st.SetMessageContent(ctx, msg.ID, "updated"); err != nil {
		t.Fatalf("set message content: %v", err)
	}

	messages, err := st.GetMessages(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "updated" {
		t.Fatalf("expected updated content, got %q", messages[0].Content)
	}
}
