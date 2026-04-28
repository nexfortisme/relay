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

	if err := st.SetMessageTokenUsage(ctx, msg.ID, 10, 5, 3, 15); err != nil {
		t.Fatalf("set message token usage: %v", err)
	}
	total, err := st.ConversationTokenTotal(ctx, conversation.ID)
	if err != nil {
		t.Fatalf("get token total: %v", err)
	}
	if total != 15 {
		t.Fatalf("expected token total 15, got %d", total)
	}
}

func TestStoreMessageAttachmentBlob(t *testing.T) {
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
		Content:        "see attached",
		Attachments:    []string{"report.txt"},
		CreatedAt:      time.Now(),
	}
	want := []byte("stored in sqlite")
	if err := st.AppendMessageWithAttachments(ctx, msg, []MessageAttachment{
		{
			Index:       0,
			Name:        "report.txt",
			ContentType: "text/plain",
			Data:        want,
			CreatedAt:   msg.CreatedAt,
		},
	}); err != nil {
		t.Fatalf("append message with attachment: %v", err)
	}

	got, err := st.GetMessageAttachment(ctx, conversation.ID, msg.ID, 0)
	if err != nil {
		t.Fatalf("get message attachment: %v", err)
	}
	if got.Name != "report.txt" {
		t.Fatalf("expected report.txt, got %q", got.Name)
	}
	if got.ContentType != "text/plain" {
		t.Fatalf("expected text/plain, got %q", got.ContentType)
	}
	if string(got.Data) != string(want) {
		t.Fatalf("expected attachment data %q, got %q", string(want), string(got.Data))
	}
	if got.SizeBytes != int64(len(want)) {
		t.Fatalf("expected size %d, got %d", len(want), got.SizeBytes)
	}
}
