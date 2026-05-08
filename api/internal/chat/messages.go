package chat

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/store"
)

func (s *Service) GetMessages(ctx context.Context, userID, conversationID string) ([]store.Message, error) {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return nil, err
	}
	return s.store.GetMessages(ctx, conversationID)
}

func (s *Service) GetFile(ctx context.Context, userID, fileID string) (store.File, error) {
	return s.store.GetFile(ctx, userID, fileID)
}

func (s *Service) AddUserMessageAndGenerate(ctx context.Context, userID, conversationID string, content string) (store.Message, error) {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return store.Message{}, err
	}
	return s.addUserMessageAndGenerate(ctx, userID, conversationID, content, content, nil, nil, nil)
}

func (s *Service) AddUserMessageAndGenerateWithFiles(ctx context.Context, userID, conversationID string, content string, files []attachments.UploadedFile) (store.Message, error) {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return store.Message{}, err
	}
	now := time.Now().UTC()
	userMessageID := uuid.NewString()
	prompt, err := attachments.BuildPrompt(content, files, s.attachmentOptions)
	if err != nil {
		return store.Message{}, err
	}
	storedFiles := make([]store.File, 0, len(files))
	links := make([]store.MessageFile, 0, len(files))
	for _, file := range files {
		fileID := uuid.NewString()
		storedFiles = append(storedFiles, store.File{
			ID:          fileID,
			Name:        file.Name,
			ContentType: file.ContentType,
			SizeBytes:   int64(len(file.Data)),
			Data:        file.Data,
			CreatedAt:   now,
		})
		links = append(links, store.MessageFile{ID: fileID, Name: file.Name})
	}
	return s.addUserMessageAndGenerate(ctx, userID, conversationID, content, prompt, &store.Message{
		ID:             userMessageID,
		ConversationID: conversationID,
		Role:           "user",
		Content:        content,
		UserContent:    content,
		LLMContent:     prompt,
		Attachments:    links,
		CreatedAt:      now,
	}, storedFiles, links)
}

func (s *Service) AddFailedUserMessage(
	ctx context.Context,
	userID string,
	conversationID string,
	content string,
	attachmentNames []string,
) (store.Message, error) {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return store.Message{}, err
	}
	now := time.Now().UTC()
	links := make([]store.MessageFile, 0, len(attachmentNames))
	for _, name := range attachmentNames {
		links = append(links, store.MessageFile{Name: name})
	}
	userMsg := store.Message{
		ID:             uuid.NewString(),
		ConversationID: conversationID,
		Role:           "user",
		Content:        content,
		UserContent:    content,
		LLMContent:     content,
		Attachments:    links,
		HasError:       true,
		CreatedAt:      now,
	}
	if err := s.store.AppendMessage(ctx, userMsg); err != nil {
		return store.Message{}, err
	}
	return userMsg, nil
}

func (s *Service) RequeueUserMessage(ctx context.Context, userID, conversationID string, messageID string) (store.Message, store.Message, error) {
	if err := s.authorizeConversation(ctx, userID, conversationID); err != nil {
		return store.Message{}, store.Message{}, err
	}
	original, err := s.store.GetMessage(ctx, conversationID, messageID)
	if err != nil {
		return store.Message{}, store.Message{}, err
	}
	if original.Role != "user" {
		return store.Message{}, store.Message{}, fmt.Errorf("only user messages can be requeued")
	}

	displayContent := strings.TrimSpace(original.UserContent)
	if displayContent == "" {
		displayContent = strings.TrimSpace(original.Content)
	}
	if displayContent == "" {
		return store.Message{}, store.Message{}, fmt.Errorf("message content is required")
	}
	llmContent := original.LLMContent
	if strings.TrimSpace(llmContent) == "" {
		llmContent = displayContent
	}

	now := time.Now().UTC()
	userMessageID := uuid.NewString()

	// Re-link the existing file rows to the new message rather than copying
	// blobs. The original user message keeps its links too, so the file row
	// has multiple referencing items — exactly the model the My Data view
	// will surface.
	links := make([]store.MessageFile, 0, len(original.Attachments))
	for _, attachment := range original.Attachments {
		if attachment.ID == "" {
			continue
		}
		links = append(links, attachment)
	}

	userMsg := store.Message{
		ID:             userMessageID,
		ConversationID: conversationID,
		Role:           "user",
		Content:        displayContent,
		UserContent:    displayContent,
		LLMContent:     llmContent,
		Attachments:    links,
		CreatedAt:      now,
	}
	assistantMsg, err := s.addUserMessageAndGenerate(ctx, userID, conversationID, displayContent, llmContent, &userMsg, nil, links)
	if err != nil {
		return store.Message{}, store.Message{}, err
	}
	return userMsg, assistantMsg, nil
}

func (s *Service) addUserMessageAndGenerate(
	ctx context.Context,
	userID string,
	conversationID string,
	displayContent string,
	llmContent string,
	preparedUserMessage *store.Message,
	preparedFiles []store.File,
	preparedLinks []store.MessageFile,
) (store.Message, error) {
	now := time.Now().UTC()
	userMsg := preparedOrNewUserMessage(preparedUserMessage, conversationID, displayContent, llmContent, now)

	if err := s.ensureConversationWithinTokenCap(ctx, conversationID); err != nil {
		return store.Message{}, err
	}

	if err := s.store.AppendMessageWithFiles(ctx, *userMsg, preparedFiles, preparedLinks); err != nil {
		return store.Message{}, err
	}
	if err := s.ensureConversationTitle(ctx, conversationID, displayContent); err != nil {
		s.logger.Warn("failed to auto-title conversation", "conversation_id", conversationID, "error", err)
	}

	assistantMsg := store.Message{
		ID:             uuid.NewString(),
		ConversationID: conversationID,
		Role:           "assistant",
		Content:        "",
		CreatedAt:      now.Add(time.Millisecond),
	}

	if err := s.store.AppendMessage(ctx, assistantMsg); err != nil {
		return store.Message{}, err
	}

	history, err := s.store.GetMessages(ctx, conversationID)
	if err != nil {
		return store.Message{}, err
	}

	settings := s.LoadRuntimeSettingsForConversation(ctx, userID, conversationID)

	// Inject notebook RAG context as a system prompt when the conversation
	// is linked to a notebook.
	var ragContext string
	if settings.NotebookID != "" && s.notebookSvc != nil {
		if rc, err := s.notebookSvc.RAGContext(ctx, userID, settings.NotebookID, displayContent); err == nil {
			ragContext = rc
		}
	}

	activePrompt := s.activeSystemPrompt(settings)
	llmMessages := toLLMMessages(history, activePrompt, ragContext, citeSourcesDirective)

	toolRuntime := s.notebookToolRuntime(userID, settings.NotebookID)
	go s.generateAssistantWithRuntime(userID, conversationID, assistantMsg.ID, llmMessages, settings, toolRuntime)
	return assistantMsg, nil
}

func preparedOrNewUserMessage(prepared *store.Message, conversationID string, displayContent string, llmContent string, now time.Time) *store.Message {
	if prepared != nil {
		return prepared
	}
	return &store.Message{
		ID:             uuid.NewString(),
		ConversationID: conversationID,
		Role:           "user",
		Content:        displayContent,
		UserContent:    displayContent,
		LLMContent:     llmContent,
		CreatedAt:      now,
	}
}

func (s *Service) ensureConversationWithinTokenCap(ctx context.Context, conversationID string) error {
	if s.maxTokenCount <= 0 {
		return nil
	}
	total, err := s.store.ConversationTokenTotal(ctx, conversationID)
	if err != nil {
		return err
	}
	if total >= s.maxTokenCount {
		return fmt.Errorf("%w (%d/%d)", ErrTokenCapReached, total, s.maxTokenCount)
	}
	return nil
}
