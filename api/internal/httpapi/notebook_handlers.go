package httpapi

import (
	"errors"
	"mime"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nexfortisme/relay/internal/store"
)

const maxNotebookFileBytes = 512 << 20 // 512 MB

// --- Notebooks CRUD ---

type createNotebookRequest struct {
	Name             string `json:"name"`
	Description      string `json:"description"`
	SystemPrompt     string `json:"systemPrompt"`
	SkillPrompt      string `json:"skillPrompt"`
	IncludeInGeneral bool   `json:"includeInGeneral"`
}

type updateNotebookRequest struct {
	Name             *string `json:"name"`
	Description      *string `json:"description"`
	SystemPrompt     *string `json:"systemPrompt"`
	SkillPrompt      *string `json:"skillPrompt"`
	IncludeInGeneral *bool   `json:"includeInGeneral"`
}

func (h *Handlers) CreateNotebook(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req createNotebookRequest
	if err := c.ShouldBindJSON(&req); err != nil || strings.TrimSpace(req.Name) == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is required"})
		return
	}
	nb, err := h.notebooks.CreateNotebook(c.Request.Context(), userID, req.Name, req.Description, req.SystemPrompt, req.SkillPrompt, req.IncludeInGeneral)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, nb)
}

func (h *Handlers) ListNotebooks(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	nbs, err := h.notebooks.ListNotebooks(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": nbs})
}

func (h *Handlers) GetNotebook(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	nb, err := h.notebooks.GetNotebook(c.Request.Context(), userID, c.Param("notebookId"))
	if err != nil {
		writeNotebookError(c, err)
		return
	}
	c.JSON(http.StatusOK, nb)
}

func (h *Handlers) UpdateNotebook(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req updateNotebookRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	patch := store.NotebookPatch{
		Name:             req.Name,
		Description:      req.Description,
		SystemPrompt:     req.SystemPrompt,
		SkillPrompt:      req.SkillPrompt,
		IncludeInGeneral: req.IncludeInGeneral,
	}
	nb, err := h.notebooks.UpdateNotebook(c.Request.Context(), userID, c.Param("notebookId"), patch)
	if err != nil {
		writeNotebookError(c, err)
		return
	}
	c.JSON(http.StatusOK, nb)
}

func (h *Handlers) DeleteNotebook(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err := h.notebooks.DeleteNotebook(c.Request.Context(), userID, c.Param("notebookId")); err != nil {
		writeNotebookError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

// --- Notebook Files ---

func (h *Handlers) UploadNotebookFile(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	notebookID := c.Param("notebookId")

	// Limit body to 512 MB + small overhead for multipart framing
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxNotebookFileBytes+4096)

	// Use 64 MB in-memory buffer; larger parts spill to temp files automatically
	if err := c.Request.ParseMultipartForm(64 << 20); err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "too large") {
			c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "file exceeds 512 MB limit"})
			return
		}
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid multipart payload"})
		return
	}

	fileHeaders := c.Request.MultipartForm.File["file"]
	if len(fileHeaders) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "file is required"})
		return
	}
	fh := fileHeaders[0]

	f, err := fh.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "open upload"})
		return
	}
	defer f.Close()

	contentType := fh.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	nf, err := h.notebooks.UploadFile(c.Request.Context(), userID, notebookID, fh.Filename, contentType, f)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, nf)
}

func (h *Handlers) ListNotebookFiles(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	files, err := h.notebooks.ListFiles(c.Request.Context(), userID, c.Param("notebookId"))
	if err != nil {
		writeNotebookError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": files})
}

func (h *Handlers) DeleteNotebookFile(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	if err := h.notebooks.DeleteFile(c.Request.Context(), userID, c.Param("notebookId"), c.Param("fileId")); err != nil {
		writeNotebookError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handlers) DownloadNotebookFile(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	data, contentType, name, err := h.notebooks.GetFileData(c.Request.Context(), userID, c.Param("notebookId"), c.Param("fileId"))
	if err != nil {
		writeNotebookError(c, err)
		return
	}
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	disp := mime.FormatMediaType("attachment", map[string]string{"filename": name})
	c.Header("Content-Disposition", disp)
	c.Data(http.StatusOK, contentType, data)
}

// --- Jobs ---

func (h *Handlers) GetPendingJobCount(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	notebookID := c.Param("notebookId")
	ctx := c.Request.Context()
	// Verify ownership
	if _, err := h.notebooks.GetNotebook(ctx, userID, notebookID); err != nil {
		writeNotebookError(c, err)
		return
	}
	count, err := h.notebooks.PendingJobCount(ctx, notebookID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": count})
}

// --- Notebook Conversation ---

func (h *Handlers) CreateNotebookConversation(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	notebookID := c.Param("notebookId")
	ctx := c.Request.Context()

	// Verify notebook ownership
	if _, err := h.notebooks.GetNotebook(ctx, userID, notebookID); err != nil {
		writeNotebookError(c, err)
		return
	}

	conv, err := h.chat.CreateConversation(ctx, userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	if err := h.chat.SetConversationNotebookID(ctx, conv.ID, notebookID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"id":         conv.ID,
		"title":      conv.Title,
		"notebookId": notebookID,
		"archived":   false,
		"favorite":   false,
		"createdAt":  conv.CreatedAt,
		"updatedAt":  conv.CreatedAt,
	})
}

func (h *Handlers) ListNotebookConversations(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	notebookID := c.Param("notebookId")
	ctx := c.Request.Context()

	if _, err := h.notebooks.GetNotebook(ctx, userID, notebookID); err != nil {
		writeNotebookError(c, err)
		return
	}

	includeArchived := c.Query("includeArchived") == "1" || c.Query("includeArchived") == "true"
	convs, err := h.chat.ListNotebookConversations(ctx, userID, notebookID, includeArchived)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": convs})
}

// --- CSV Viewer ---

func (h *Handlers) GetCSVTableData(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	columns, rows, err := h.notebooks.GetCSVTableData(c.Request.Context(), userID, c.Param("notebookId"), c.Param("fileId"))
	if err != nil {
		writeNotebookError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"columns": columns, "rows": rows})
}

// GetNotebookPageImage serves the rendered JPEG image stored for a PDF page.
func (h *Handlers) GetNotebookPageImage(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	pageNum, err := strconv.Atoi(c.Param("pageNum"))
	if err != nil || pageNum < 1 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid page number"})
		return
	}
	data, contentType, err := h.notebooks.GetPageImage(c.Request.Context(), userID, c.Param("notebookId"), c.Param("fileId"), pageNum)
	if err != nil {
		writeNotebookError(c, err)
		return
	}
	c.Data(http.StatusOK, contentType, data)
}

// writeNotebookError maps store errors to HTTP status codes.
func writeNotebookError(c *gin.Context, err error) {
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
}
