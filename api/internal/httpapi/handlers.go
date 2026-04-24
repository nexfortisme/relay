package httpapi

import (
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nexfortisme/relay/internal/chat"
)

type Handlers struct {
	chat   *chat.Service
	logger *slog.Logger
}

var streamUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func NewHandlers(chatService *chat.Service, logger *slog.Logger) *Handlers {
	return &Handlers{chat: chatService, logger: logger}
}

func (h *Handlers) CreateConversation(c *gin.Context) {
	conversation, err := h.chat.CreateConversation(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, conversation)
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

type renameConversationRequest struct {
	Title string `json:"title"`
}

func (h *Handlers) CreateMessage(c *gin.Context) {
	conversationID := c.Param("id")
	var req createMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	if req.Content == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "content is required"})
		return
	}

	assistantMessage, err := h.chat.AddUserMessageAndGenerate(c.Request.Context(), conversationID, req.Content)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"assistantMessageId": assistantMessage.ID,
	})
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
