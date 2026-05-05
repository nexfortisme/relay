package httpapi

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/chat"
	"github.com/nexfortisme/relay/internal/feeds"
	"github.com/nexfortisme/relay/internal/store"
)

const maxSingleFileBytes = 50 << 20

type Handlers struct {
	chat                     *chat.Service
	feeds                    *feeds.Service
	logger                   *slog.Logger
	maxMultipartPayloadBytes int64
	maxMultipartPayloadLabel string
}

var streamUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewHandlers(chatService *chat.Service, feedsService *feeds.Service, logger *slog.Logger, maxMultipartPayloadBytes int64) *Handlers {
	if maxMultipartPayloadBytes <= 0 {
		maxMultipartPayloadBytes = 30 << 20 // 30MB
	}
	return &Handlers{
		chat:                     chatService,
		feeds:                    feedsService,
		logger:                   logger,
		maxMultipartPayloadBytes: maxMultipartPayloadBytes,
		maxMultipartPayloadLabel: bytesLabel(maxMultipartPayloadBytes),
	}
}

func (h *Handlers) CreateConversation(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	conversation, err := h.chat.CreateConversation(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, conversation)
}

func (h *Handlers) GetSettings(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	settings, err := h.chat.GetSettings(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

type updateSettingsRequest struct {
	LLMUrl       *string `json:"llm_url"`
	LLMModel     *string `json:"llm_model"`
	LLMAPIKey    *string `json:"llm_api_key"`
	SystemPrompt *string `json:"system_prompt"`
}

func (h *Handlers) UpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	updates := make(map[string]string)
	if req.LLMUrl != nil {
		updates["llm_url"] = *req.LLMUrl
	}
	if req.LLMModel != nil {
		updates["llm_model"] = *req.LLMModel
	}
	if req.LLMAPIKey != nil {
		updates["llm_api_key"] = *req.LLMAPIKey
	}
	if req.SystemPrompt != nil {
		updates["system_prompt"] = *req.SystemPrompt
	}
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err := h.chat.UpdateSettings(c.Request.Context(), userID, updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	settings, err := h.chat.GetSettings(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *Handlers) ListConversations(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	includeArchived := c.Query("includeArchived") == "1" || c.Query("includeArchived") == "true"
	conversations, err := h.chat.ListConversations(c.Request.Context(), userID, includeArchived)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": conversations})
}

func (h *Handlers) ListMessages(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	conversationID := c.Param("id")
	messages, err := h.chat.GetMessages(c.Request.Context(), userID, conversationID)
	if err != nil {
		writeChatError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": messages})
}

type createMessageRequest struct {
	Content string `json:"content"`
}

type createFailedMessageRequest struct {
	Content     string   `json:"content"`
	Attachments []string `json:"attachments"`
}

type requeueMessageResponse struct {
	UserMessage        store.Message `json:"userMessage"`
	AssistantMessageID string        `json:"assistantMessageId"`
}

type renameConversationRequest struct {
	Title string `json:"title"`
}

type messagePayload struct {
	Content string
	Files   []attachments.UploadedFile
}

type requestError struct {
	status  int
	message string
}

func (h *Handlers) CreateMessage(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	conversationID := c.Param("id")

	payload, reqErr := h.parseCreateMessagePayload(c)
	if reqErr != nil {
		c.JSON(reqErr.status, gin.H{"error": reqErr.message})
		return
	}
	if payload.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	assistantMessageID, err := h.queueAssistantResponse(c.Request.Context(), userID, conversationID, payload)
	if err != nil {
		if errors.Is(err, chat.ErrTokenCapReached) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if len(payload.Files) > 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"assistantMessageId": assistantMessageID,
	})
}

func (h *Handlers) parseCreateMessagePayload(c *gin.Context) (messagePayload, *requestError) {
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		return h.parseMultipartMessagePayload(c)
	}

	var req createMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return messagePayload{}, &requestError{status: http.StatusBadRequest, message: "invalid payload"}
	}
	return messagePayload{Content: strings.TrimSpace(req.Content)}, nil
}

func (h *Handlers) parseMultipartMessagePayload(c *gin.Context) (messagePayload, *requestError) {
	if err := c.Request.ParseMultipartForm(h.maxMultipartPayloadBytes); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
			return messagePayload{}, &requestError{
				status:  http.StatusRequestEntityTooLarge,
				message: fmt.Sprintf("total file upload size exceeds %s", h.maxMultipartPayloadLabel),
			}
		}
		return messagePayload{}, &requestError{status: http.StatusBadRequest, message: "invalid multipart payload"}
	}

	files, err := h.parseUploadedFiles(c.Request.MultipartForm.File["files"])
	if err != nil {
		return messagePayload{}, &requestError{status: http.StatusBadRequest, message: err.Error()}
	}
	return messagePayload{
		Content: strings.TrimSpace(c.PostForm("content")),
		Files:   files,
	}, nil
}

func (h *Handlers) queueAssistantResponse(ctx context.Context, userID, conversationID string, payload messagePayload) (string, error) {
	if len(payload.Files) > 0 {
		msg, err := h.chat.AddUserMessageAndGenerateWithFiles(ctx, userID, conversationID, payload.Content, payload.Files)
		return msg.ID, err
	}
	msg, err := h.chat.AddUserMessageAndGenerate(ctx, userID, conversationID, payload.Content)
	return msg.ID, err
}

func (h *Handlers) CreateFailedMessage(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	conversationID := c.Param("id")
	var req createFailedMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	content := strings.TrimSpace(req.Content)
	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	msg, err := h.chat.AddFailedUserMessage(c.Request.Context(), userID, conversationID, content, req.Attachments)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, msg)
}

func (h *Handlers) RequeueMessage(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	userMessage, assistantMessage, err := h.chat.RequeueUserMessage(
		c.Request.Context(),
		userID,
		c.Param("id"),
		c.Param("messageId"),
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, requeueMessageResponse{
		UserMessage:        userMessage,
		AssistantMessageID: assistantMessage.ID,
	})
}

func bytesLabel(bytes int64) string {
	if bytes <= 0 {
		return "0B"
	}
	const (
		kb = int64(1024)
		mb = kb * 1024
	)
	if bytes >= mb {
		whole := float64(bytes) / float64(mb)
		if math.Mod(whole, 1) == 0 {
			return fmt.Sprintf("%.0fMB", whole)
		}
		return fmt.Sprintf("%.1fMB", whole)
	}
	if bytes >= kb {
		whole := float64(bytes) / float64(kb)
		if math.Mod(whole, 1) == 0 {
			return fmt.Sprintf("%.0fKB", whole)
		}
		return fmt.Sprintf("%.1fKB", whole)
	}
	return fmt.Sprintf("%dB", bytes)
}

func (h *Handlers) parseUploadedFiles(formFiles []*multipart.FileHeader) ([]attachments.UploadedFile, error) {
	files := make([]attachments.UploadedFile, 0, len(formFiles))
	for _, fileHeader := range formFiles {
		if fileHeader.Size > maxSingleFileBytes {
			return nil, fmt.Errorf("%s exceeds max size of %s", fileHeader.Filename, bytesLabel(maxSingleFileBytes))
		}

		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", fileHeader.Filename, err)
		}

		raw, readErr := io.ReadAll(io.LimitReader(file, maxSingleFileBytes+1))
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", fileHeader.Filename, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", fileHeader.Filename, closeErr)
		}
		if int64(len(raw)) > maxSingleFileBytes {
			return nil, fmt.Errorf("%s exceeds max size of %s", fileHeader.Filename, bytesLabel(maxSingleFileBytes))
		}

		files = append(files, attachments.UploadedFile{
			Name:        fileHeader.Filename,
			ContentType: fileHeader.Header.Get("Content-Type"),
			Data:        raw,
		})
	}
	return files, nil
}

func (h *Handlers) RenameConversation(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	conversationID := c.Param("id")
	var req renameConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}

	if err := h.chat.RenameConversation(c.Request.Context(), userID, conversationID, req.Title); err != nil {
		writeChatError(c, err)
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handlers) SuggestConversationTitle(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	conversationID := c.Param("id")
	title, err := h.chat.SuggestConversationTitle(c.Request.Context(), userID, conversationID)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"title": title})
}

func (h *Handlers) ArchiveConversation(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err := h.chat.ArchiveConversation(c.Request.Context(), userID, c.Param("id")); err != nil {
		writeChatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) RestoreConversation(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err := h.chat.RestoreConversation(c.Request.Context(), userID, c.Param("id")); err != nil {
		writeChatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) DeleteConversation(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err := h.chat.DeleteConversation(c.Request.Context(), userID, c.Param("id")); err != nil {
		writeChatError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) StopConversationGeneration(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err := h.chat.AuthorizeConversation(c.Request.Context(), userID, c.Param("id")); err != nil {
		writeChatError(c, err)
		return
	}
	if stopped := h.chat.StopGeneration(c.Param("id")); !stopped {
		c.JSON(http.StatusConflict, gin.H{"error": "no generation in progress"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) DownloadFile(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	fileID := c.Param("id")
	if strings.TrimSpace(fileID) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file id is required"})
		return
	}

	file, err := h.chat.GetFile(c.Request.Context(), userID, fileID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	contentType := file.ContentType
	if strings.TrimSpace(contentType) == "" {
		contentType = http.DetectContentType(file.Data)
	}
	disposition := mime.FormatMediaType("attachment", map[string]string{"filename": file.Name})
	c.Header("Content-Disposition", disposition)
	c.DataFromReader(http.StatusOK, file.SizeBytes, contentType, bytes.NewReader(file.Data), nil)
}

func (h *Handlers) StreamConversation(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	conversationID := c.Param("id")
	if err := h.chat.AuthorizeConversation(c.Request.Context(), userID, conversationID); err != nil {
		writeChatError(c, err)
		return
	}
	sub, unsubscribe := h.chat.Subscribe(conversationID)
	defer unsubscribe()

	conn, err := streamUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Warn("failed to upgrade websocket", "error", err)
		return
	}
	defer conn.Close()

	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-c.Request.Context().Done():
			return
		case <-ticker.C:
			if err := conn.WriteJSON(chat.Event{
				Type: "ping",
			}); err != nil {
				return
			}
		case event := <-sub:
			if err := conn.WriteJSON(event); err != nil {
				return
			}
		}
	}
}

// writeChatError maps service-layer errors to HTTP status codes. Forbidden and
// not-found are folded into 404 so we don't leak existence of conversations
// owned by other users.
func writeChatError(c *gin.Context, err error) {
	if errors.Is(err, chat.ErrForbidden) || errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}

func RequestLogger(logger *slog.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		duration := time.Since(start)
		logger.Info("http_request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", duration.Milliseconds(),
			"request_id", c.GetString("requestID"),
		)
	}
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		reqID := c.GetHeader("X-Request-ID")
		if reqID == "" {
			reqID = fmt.Sprintf("req-%d", time.Now().UnixNano())
		}
		c.Set("requestID", reqID)
		c.Writer.Header().Set("X-Request-ID", reqID)
		c.Next()
	}
}
