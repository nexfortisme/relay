package httpapi

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nexfortisme/relay/internal/feeds"
	"github.com/nexfortisme/relay/internal/store"
)

func TestFeedHandlersCreateListPatchAndIsolateUsers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	feedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = io.WriteString(w, `<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>Handler Feed</title>
  <link>`+feedServerPlaceholder+`</link>
  <item>
    <title>Handler Item</title>
    <link>/first</link>
    <guid>handler-guid</guid>
    <description>Handler preview</description>
  </item>
</channel></rss>`)
	}))
	defer feedServer.Close()

	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	service := feeds.NewService(st, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	handlers := NewHandlers(nil, service, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		userID := c.GetHeader("X-User-ID")
		if userID == "" {
			userID = "user-1"
		}
		c.Set("userID", userID)
		c.Next()
	})
	router.POST("/feeds", handlers.CreateFeed)
	router.GET("/feeds", handlers.ListFeeds)
	router.GET("/feeds/items", handlers.ListFeedItems)
	router.PATCH("/feeds/items/:id", handlers.UpdateFeedItem)
	router.POST("/feeds/:id/mark-read", handlers.MarkFeedRead)
	router.DELETE("/feeds/:id", handlers.DeleteFeed)

	createBody := strings.NewReader(`{"url":"` + feedServer.URL + `/rss.xml","backfill":{"mode":"latest","limit":1},"pollingIntervalMinutes":30}`)
	createReq := httptest.NewRequest(http.MethodPost, "/feeds", createBody)
	createReq.Header.Set("Content-Type", "application/json")
	createRes := httptest.NewRecorder()
	router.ServeHTTP(createRes, createReq)
	if createRes.Code != http.StatusCreated {
		t.Fatalf("create feed status=%d body=%s", createRes.Code, createRes.Body.String())
	}

	var createPayload struct {
		Items []store.FeedItem `json:"items"`
	}
	if err := json.Unmarshal(createRes.Body.Bytes(), &createPayload); err != nil {
		t.Fatalf("decode create response: %v", err)
	}
	if len(createPayload.Items) != 1 {
		t.Fatalf("expected one backfilled item, got %d", len(createPayload.Items))
	}
	itemID := createPayload.Items[0].ID

	listFeedsReq := httptest.NewRequest(http.MethodGet, "/feeds", nil)
	listFeedsRes := httptest.NewRecorder()
	router.ServeHTTP(listFeedsRes, listFeedsReq)
	if listFeedsRes.Code != http.StatusOK {
		t.Fatalf("list feeds status=%d body=%s", listFeedsRes.Code, listFeedsRes.Body.String())
	}
	var listFeedsPayload struct {
		Items []store.Feed `json:"items"`
	}
	if err := json.Unmarshal(listFeedsRes.Body.Bytes(), &listFeedsPayload); err != nil {
		t.Fatalf("decode list feeds response: %v", err)
	}
	if len(listFeedsPayload.Items) != 1 || listFeedsPayload.Items[0].UnreadCount != 1 {
		t.Fatalf("unexpected feeds payload: %#v", listFeedsPayload.Items)
	}

	otherUserReq := httptest.NewRequest(http.MethodGet, "/feeds/items?view=unread", nil)
	otherUserReq.Header.Set("X-User-ID", "user-2")
	otherUserRes := httptest.NewRecorder()
	router.ServeHTTP(otherUserRes, otherUserReq)
	if otherUserRes.Code != http.StatusOK {
		t.Fatalf("other user list status=%d body=%s", otherUserRes.Code, otherUserRes.Body.String())
	}
	var otherUserPayload struct {
		Items []store.FeedItem `json:"items"`
	}
	if err := json.Unmarshal(otherUserRes.Body.Bytes(), &otherUserPayload); err != nil {
		t.Fatalf("decode other user response: %v", err)
	}
	if len(otherUserPayload.Items) != 0 {
		t.Fatalf("expected user isolation, got %#v", otherUserPayload.Items)
	}

	otherUserMarkReq := httptest.NewRequest(http.MethodPost, "/feeds/"+listFeedsPayload.Items[0].ID+"/mark-read", nil)
	otherUserMarkReq.Header.Set("X-User-ID", "user-2")
	otherUserMarkRes := httptest.NewRecorder()
	router.ServeHTTP(otherUserMarkRes, otherUserMarkReq)
	if otherUserMarkRes.Code != http.StatusNotFound {
		t.Fatalf("other user mark read status=%d body=%s", otherUserMarkRes.Code, otherUserMarkRes.Body.String())
	}

	markReq := httptest.NewRequest(http.MethodPost, "/feeds/"+listFeedsPayload.Items[0].ID+"/mark-read", nil)
	markRes := httptest.NewRecorder()
	router.ServeHTTP(markRes, markReq)
	if markRes.Code != http.StatusOK {
		t.Fatalf("mark read status=%d body=%s", markRes.Code, markRes.Body.String())
	}
	var markPayload struct {
		Feed         store.Feed `json:"feed"`
		UpdatedCount int        `json:"updatedCount"`
	}
	if err := json.Unmarshal(markRes.Body.Bytes(), &markPayload); err != nil {
		t.Fatalf("decode mark read response: %v", err)
	}
	if markPayload.UpdatedCount != 1 || markPayload.Feed.UnreadCount != 0 {
		t.Fatalf("unexpected mark read payload: %#v", markPayload)
	}

	patchReq := httptest.NewRequest(http.MethodPatch, "/feeds/items/"+itemID, strings.NewReader(`{"read":true}`))
	patchReq.Header.Set("Content-Type", "application/json")
	patchRes := httptest.NewRecorder()
	router.ServeHTTP(patchRes, patchReq)
	if patchRes.Code != http.StatusOK {
		t.Fatalf("patch item status=%d body=%s", patchRes.Code, patchRes.Body.String())
	}

	unreadReq := httptest.NewRequest(http.MethodGet, "/feeds/items?view=unread", nil)
	unreadRes := httptest.NewRecorder()
	router.ServeHTTP(unreadRes, unreadReq)
	var unreadPayload struct {
		Items []store.FeedItem `json:"items"`
	}
	if err := json.Unmarshal(unreadRes.Body.Bytes(), &unreadPayload); err != nil {
		t.Fatalf("decode unread response: %v", err)
	}
	if len(unreadPayload.Items) != 0 {
		t.Fatalf("expected read item removed from unread view, got %#v", unreadPayload.Items)
	}

	otherUserDeleteReq := httptest.NewRequest(http.MethodDelete, "/feeds/"+listFeedsPayload.Items[0].ID, nil)
	otherUserDeleteReq.Header.Set("X-User-ID", "user-2")
	otherUserDeleteRes := httptest.NewRecorder()
	router.ServeHTTP(otherUserDeleteRes, otherUserDeleteReq)
	if otherUserDeleteRes.Code != http.StatusNotFound {
		t.Fatalf("other user delete status=%d body=%s", otherUserDeleteRes.Code, otherUserDeleteRes.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/feeds/"+listFeedsPayload.Items[0].ID, nil)
	deleteRes := httptest.NewRecorder()
	router.ServeHTTP(deleteRes, deleteReq)
	if deleteRes.Code != http.StatusNoContent {
		t.Fatalf("delete feed status=%d body=%s", deleteRes.Code, deleteRes.Body.String())
	}

	itemsAfterDeleteReq := httptest.NewRequest(http.MethodGet, "/feeds/items?view=all", nil)
	itemsAfterDeleteRes := httptest.NewRecorder()
	router.ServeHTTP(itemsAfterDeleteRes, itemsAfterDeleteReq)
	var itemsAfterDeletePayload struct {
		Items []store.FeedItem `json:"items"`
	}
	if err := json.Unmarshal(itemsAfterDeleteRes.Body.Bytes(), &itemsAfterDeletePayload); err != nil {
		t.Fatalf("decode items after delete response: %v", err)
	}
	if len(itemsAfterDeletePayload.Items) != 0 {
		t.Fatalf("expected feed delete to remove items, got %#v", itemsAfterDeletePayload.Items)
	}
}

const feedServerPlaceholder = "https://example.com"
