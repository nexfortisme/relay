package httpapi

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type renameConversationRequest struct {
	Title string `json:"title"`
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
