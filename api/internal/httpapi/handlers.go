package httpapi

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nexfortisme/relay/internal/attachments"
	"github.com/nexfortisme/relay/internal/chat"
	"github.com/nexfortisme/relay/internal/feeds"
	"github.com/nexfortisme/relay/internal/notebooks"
	"github.com/nexfortisme/relay/internal/store"
)

const maxSingleFileBytes = 50 << 20

type Handlers struct {
	chat                     *chat.Service
	feeds                    *feeds.Service
	notebooks                *notebooks.Service
	logger                   *slog.Logger
	maxMultipartPayloadBytes int64
	maxMultipartPayloadLabel string
}

type messagePayload struct {
	Content string
	Files   []attachments.UploadedFile
}

type requestError struct {
	status  int
	message string
}

func NewHandlers(chatService *chat.Service, feedsService *feeds.Service, notebooksService *notebooks.Service, logger *slog.Logger, maxMultipartPayloadBytes int64) *Handlers {
	if maxMultipartPayloadBytes <= 0 {
		maxMultipartPayloadBytes = 30 << 20 // 30MB
	}
	return &Handlers{
		chat:                     chatService,
		feeds:                    feedsService,
		notebooks:                notebooksService,
		logger:                   logger,
		maxMultipartPayloadBytes: maxMultipartPayloadBytes,
		maxMultipartPayloadLabel: bytesLabel(maxMultipartPayloadBytes),
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
