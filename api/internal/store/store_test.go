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
	conversation, err := st.CreateConversation(ctx, "conv-1", "user-1", "Test", time.Now())
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

func TestStoreFilesAndMessageLinks(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	if _, err := st.CreateConversation(ctx, "conv-1", "user-1", "Test", time.Now()); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	want := []byte("stored in sqlite")
	fileID := "file-1"
	msg := Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		Role:           "user",
		Content:        "see attached",
		CreatedAt:      time.Now(),
	}
	files := []File{
		{
			ID:          fileID,
			Name:        "report.txt",
			ContentType: "text/plain",
			Data:        want,
			CreatedAt:   msg.CreatedAt,
		},
	}
	links := []MessageFile{{ID: fileID, Name: "report.txt"}}
	if err := st.AppendMessageWithFiles(ctx, msg, files, links); err != nil {
		t.Fatalf("append message with files: %v", err)
	}

	messages, err := st.GetMessages(ctx, "conv-1")
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	if len(messages[0].Attachments) != 1 || messages[0].Attachments[0].ID != fileID || messages[0].Attachments[0].Name != "report.txt" {
		t.Fatalf("unexpected attachments: %#v", messages[0].Attachments)
	}

	got, err := st.GetFile(ctx, "user-1", fileID)
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if got.Name != "report.txt" {
		t.Fatalf("expected report.txt, got %q", got.Name)
	}
	if string(got.Data) != string(want) {
		t.Fatalf("expected %q, got %q", string(want), string(got.Data))
	}
	if got.SizeBytes != int64(len(want)) {
		t.Fatalf("expected size %d, got %d", len(want), got.SizeBytes)
	}

	ids, err := st.GetMessageFileIDs(ctx, "msg-1")
	if err != nil {
		t.Fatalf("get message file ids: %v", err)
	}
	if len(ids) != 1 || ids[0] != fileID {
		t.Fatalf("expected [%s], got %v", fileID, ids)
	}
}

func TestStoreFailedMessageRetainsAttachmentNames(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	if _, err := st.CreateConversation(ctx, "conv-1", "user-1", "Test", time.Now()); err != nil {
		t.Fatalf("create conversation: %v", err)
	}

	msg := Message{
		ID:             "msg-1",
		ConversationID: "conv-1",
		Role:           "user",
		Content:        "see attached",
		HasError:       true,
		Attachments:    []MessageFile{{Name: "lost.txt"}, {Name: "alsolost.png"}},
		CreatedAt:      time.Now(),
	}
	if err := st.AppendMessage(ctx, msg); err != nil {
		t.Fatalf("append failed message: %v", err)
	}

	messages, err := st.GetMessages(ctx, "conv-1")
	if err != nil {
		t.Fatalf("get messages: %v", err)
	}
	if len(messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(messages))
	}
	got := messages[0].Attachments
	if len(got) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(got))
	}
	if got[0].Name != "lost.txt" || got[1].Name != "alsolost.png" {
		t.Fatalf("unexpected attachment names: %#v", got)
	}
	if got[0].ID != "" || got[1].ID != "" {
		t.Fatalf("expected empty IDs for failed-message attachments, got %#v", got)
	}
}
