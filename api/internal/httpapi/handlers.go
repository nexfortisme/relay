package httpapi

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/chat"
)

type Handlers struct {
	chat                     *chat.Service
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

func NewHandlers(chatService *chat.Service, logger *slog.Logger, maxMultipartPayloadBytes int64) *Handlers {
	if maxMultipartPayloadBytes <= 0 {
		maxMultipartPayloadBytes = 30 << 20
	}
	return &Handlers{
		chat:                     chatService,
		logger:                   logger,
		maxMultipartPayloadBytes: maxMultipartPayloadBytes,
		maxMultipartPayloadLabel: bytesLabel(maxMultipartPayloadBytes),
	}
}

func (h *Handlers) CreateConversation(c *gin.Context) {
	conversation, err := h.chat.CreateConversation(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, conversation)
}

func (h *Handlers) GetSettings(c *gin.Context) {
	settings, err := h.chat.GetSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

type updateSettingsRequest struct {
	LLMUrl       *string `json:"llm_url"`
	LLMModel     *string `json:"llm_model"`
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
	if req.SystemPrompt != nil {
		updates["system_prompt"] = *req.SystemPrompt
	}
	if err := h.chat.UpdateSettings(c.Request.Context(), updates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	settings, err := h.chat.GetSettings(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, settings)
}

func (h *Handlers) ListConversations(c *gin.Context) {
	includeArchived := c.Query("includeArchived") == "1" || c.Query("includeArchived") == "true"
	conversations, err := h.chat.ListConversations(c.Request.Context(), includeArchived)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": conversations})
}

func (h *Handlers) ListMessages(c *gin.Context) {
	conversationID := c.Param("id")
	messages, err := h.chat.GetMessages(c.Request.Context(), conversationID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
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

type renameConversationRequest struct {
	Title string `json:"title"`
}

func (h *Handlers) CreateMessage(c *gin.Context) {
	conversationID := c.Param("id")

	contentType := c.ContentType()
	var content string
	files := make([]attachments.UploadedFile, 0)

	if strings.HasPrefix(contentType, "multipart/form-data") {
		if err := c.Request.ParseMultipartForm(h.maxMultipartPayloadBytes); err != nil {
			if strings.Contains(strings.ToLower(err.Error()), "request body too large") {
				c.JSON(http.StatusRequestEntityTooLarge, gin.H{
					"error": fmt.Sprintf("total file upload size exceeds %s", h.maxMultipartPayloadLabel),
				})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart payload"})
			return
		}
		content = strings.TrimSpace(c.PostForm("content"))
		formFiles := c.Request.MultipartForm.File["files"]
		parsedFiles, err := parseUploadedFiles(formFiles)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		files = parsedFiles
	} else {
		var req createMessageRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
			return
		}
		content = strings.TrimSpace(req.Content)
	}

	if content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	var assistantMessageID string
	var err error
	if len(files) > 0 {
		msg, withFilesErr := h.chat.AddUserMessageAndGenerateWithFiles(c.Request.Context(), conversationID, content, files)
		err = withFilesErr
		assistantMessageID = msg.ID
	} else {
		msg, noFileErr := h.chat.AddUserMessageAndGenerate(c.Request.Context(), conversationID, content)
		err = noFileErr
		assistantMessageID = msg.ID
	}
	if err != nil {
		if len(files) > 0 {
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

func (h *Handlers) CreateFailedMessage(c *gin.Context) {
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

	msg, err := h.chat.AddFailedUserMessage(c.Request.Context(), conversationID, content, req.Attachments)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, msg)
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

func parseUploadedFiles(formFiles []*multipart.FileHeader) ([]attachments.UploadedFile, error) {
	files := make([]attachments.UploadedFile, 0, len(formFiles))
	for _, fileHeader := range formFiles {
		file, err := fileHeader.Open()
		if err != nil {
			return nil, fmt.Errorf("open %s: %w", fileHeader.Filename, err)
		}

		raw, readErr := io.ReadAll(file)
		closeErr := file.Close()
		if readErr != nil {
			return nil, fmt.Errorf("read %s: %w", fileHeader.Filename, readErr)
		}
		if closeErr != nil {
			return nil, fmt.Errorf("close %s: %w", fileHeader.Filename, closeErr)
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

	if err := h.chat.RenameConversation(c.Request.Context(), conversationID, req.Title); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *Handlers) ArchiveConversation(c *gin.Context) {
	if err := h.chat.ArchiveConversation(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) RestoreConversation(c *gin.Context) {
	if err := h.chat.RestoreConversation(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) DeleteConversation(c *gin.Context) {
	if err := h.chat.DeleteConversation(c.Request.Context(), c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) StopConversationGeneration(c *gin.Context) {
	if stopped := h.chat.StopGeneration(c.Param("id")); !stopped {
		c.JSON(http.StatusConflict, gin.H{"error": "no generation in progress"})
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) DownloadMessageAttachment(c *gin.Context) {
	conversationID := c.Param("id")
	messageID := c.Param("messageId")
	attachmentIndex, err := attachments.ParseAttachmentIndex(c.Param("attachmentIndex"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	path, storedName, err := h.chat.GetMessageAttachment(c.Request.Context(), conversationID, messageID, attachmentIndex)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.FileAttachment(path, storedName)
}

func (h *Handlers) StreamConversation(c *gin.Context) {
	conversationID := c.Param("id")
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
