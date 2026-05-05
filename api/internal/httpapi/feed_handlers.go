package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nexfortisme/relay/internal/feeds"
	"github.com/nexfortisme/relay/internal/store"
)

type checkFeedRequest struct {
	URL string `json:"url"`
}

type createFeedRequest struct {
	URL                    string              `json:"url"`
	Name                   string              `json:"name"`
	Title                  string              `json:"title"`
	PollingIntervalMinutes int                 `json:"pollingIntervalMinutes"`
	AutoSummarize          bool                `json:"autoSummarize"`
	AutoAddToNotebook      bool                `json:"autoAddToNotebook"`
	NotebookID             string              `json:"notebookId"`
	Backfill               backfillHTTPRequest `json:"backfill"`
}

type backfillHTTPRequest struct {
	Mode  string `json:"mode"`
	Limit int    `json:"limit"`
	Since string `json:"since"`
}

type updateFeedRequest struct {
	Name                   *string `json:"name"`
	Title                  *string `json:"title"`
	PollingIntervalMinutes *int    `json:"pollingIntervalMinutes"`
	AutoSummarize          *bool   `json:"autoSummarize"`
	AutoAddToNotebook      *bool   `json:"autoAddToNotebook"`
	NotebookID             *string `json:"notebookId"`
}

type updateFeedItemRequest struct {
	Read    *bool `json:"read"`
	Starred *bool `json:"starred"`
}

type summarizeFeedItemRequest struct {
	Mode string `json:"mode"`
}

func (h *Handlers) CheckFeed(c *gin.Context) {
	var req checkFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	result, err := h.feeds.CheckFeed(c.Request.Context(), req.URL)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handlers) CreateFeed(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req createFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	backfill, err := parseBackfill(req.Backfill)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	title := strings.TrimSpace(req.Name)
	if title == "" {
		title = strings.TrimSpace(req.Title)
	}
	feed, items, err := h.feeds.CreateFeed(c.Request.Context(), userID, feeds.CreateFeedRequest{
		URL:                    req.URL,
		Title:                  title,
		PollingIntervalMinutes: req.PollingIntervalMinutes,
		AutoSummarize:          req.AutoSummarize,
		AutoAddToNotebook:      req.AutoAddToNotebook,
		NotebookID:             req.NotebookID,
		Backfill:               backfill,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"feed": feed, "items": items})
}

func (h *Handlers) ListFeeds(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	feeds, err := h.feeds.ListFeeds(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": feeds})
}

func (h *Handlers) UpdateFeed(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req updateFeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	title := req.Title
	if title == nil {
		title = req.Name
	}
	feed, err := h.feeds.UpdateFeed(c.Request.Context(), userID, c.Param("id"), store.FeedUpdate{
		Title:                  title,
		PollingIntervalMinutes: req.PollingIntervalMinutes,
		AutoSummarize:          req.AutoSummarize,
		AutoAddToNotebook:      req.AutoAddToNotebook,
		NotebookID:             req.NotebookID,
	})
	if err != nil {
		writeFeedError(c, err)
		return
	}
	c.JSON(http.StatusOK, feed)
}

func (h *Handlers) ListFeedItems(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	items, err := h.feeds.ListItems(c.Request.Context(), userID, store.FeedItemFilter{
		View:   c.DefaultQuery("view", "unread"),
		FeedID: strings.TrimSpace(c.Query("feedId")),
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (h *Handlers) GetFeedItem(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	item, err := h.feeds.GetItem(c.Request.Context(), userID, c.Param("id"))
	if err != nil {
		writeFeedError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handlers) UpdateFeedItem(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req updateFeedItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	item, err := h.feeds.PatchItem(c.Request.Context(), userID, c.Param("id"), feeds.ItemPatch{
		Read:    req.Read,
		Starred: req.Starred,
	})
	if err != nil {
		writeFeedError(c, err)
		return
	}
	c.JSON(http.StatusOK, item)
}

func (h *Handlers) SummarizeFeedItem(c *gin.Context) {
	userID, ok := userIDFromGin(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	var req summarizeFeedItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	item, err := h.feeds.SummarizeItem(c.Request.Context(), userID, c.Param("id"), req.Mode)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
			return
		}
		if errors.Is(err, feeds.ErrVideoSummaryUnsupported) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, item)
}

func parseBackfill(req backfillHTTPRequest) (feeds.BackfillOptions, error) {
	mode := strings.TrimSpace(strings.ToLower(req.Mode))
	if mode == "" {
		mode = "latest"
	}
	options := feeds.BackfillOptions{
		Mode:  mode,
		Limit: req.Limit,
	}
	if mode == "since" {
		if strings.TrimSpace(req.Since) == "" {
			return options, errors.New("backfill date is required")
		}
		parsed, err := time.Parse("2006-01-02", strings.TrimSpace(req.Since))
		if err != nil {
			return options, errors.New("backfill date must use YYYY-MM-DD")
		}
		options.Since = &parsed
	}
	return options, nil
}

func writeFeedError(c *gin.Context, err error) {
	if errors.Is(err, store.ErrNotFound) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
		return
	}
	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
}
