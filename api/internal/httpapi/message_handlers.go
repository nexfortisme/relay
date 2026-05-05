package httpapi

import (
	"bytes"
	"context"
	"errors"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nexfortisme/relay/internal/chat"
)

type createMessageRequest struct {
	Content string `json:"content"`
}

type createFailedMessageRequest struct {
	Content     string   `json:"content"`
	Attachments []string `json:"attachments"`
}

type requeueMessageResponse struct {
	UserMessage        interface{} `json:"userMessage"`
	AssistantMessageID string      `json:"assistantMessageId"`
}

var streamUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
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
	c.JSON(http.StatusAccepted, gin.H{
		"userMessage":        userMessage,
		"assistantMessageId": assistantMessage.ID,
	})
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

func (h *Handlers) queueAssistantResponse(ctx context.Context, userID, conversationID string, payload messagePayload) (string, error) {
	if len(payload.Files) > 0 {
		msg, err := h.chat.AddUserMessageAndGenerateWithFiles(ctx, userID, conversationID, payload.Content, payload.Files)
		return msg.ID, err
	}
	msg, err := h.chat.AddUserMessageAndGenerate(ctx, userID, conversationID, payload.Content)
	return msg.ID, err
}
