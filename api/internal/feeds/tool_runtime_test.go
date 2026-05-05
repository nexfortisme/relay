package feeds

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/nexfortisme/relay/internal/auth"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

func TestToolRuntimeListAndGetFeedItemsUserIsolationReadOnlyAndFilters(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	now := time.Now().UTC()
	createTestFeed(t, st, store.Feed{ID: "feed-user-1", UserID: "user-1", URL: "https://example.com/one.xml", Title: "Example One", NextCheckAt: now.Add(time.Hour), LastCheckedAt: &now, CreatedAt: now, UpdatedAt: now})
	createTestFeed(t, st, store.Feed{ID: "feed-user-2", UserID: "user-2", URL: "https://example.com/two.xml", Title: "Example Two", NextCheckAt: now.Add(time.Hour), LastCheckedAt: &now, CreatedAt: now, UpdatedAt: now})
	createTestItems(t, st, []store.FeedItem{
		{ID: "user-1-unread", UserID: "user-1", FeedID: "feed-user-1", ExternalID: "u1-unread", Title: "Unread", URL: "https://example.com/unread", Content: "unread content", CreatedAt: now, UpdatedAt: now},
		{ID: "user-1-read", UserID: "user-1", FeedID: "feed-user-1", ExternalID: "u1-read", Title: "Read", URL: "https://example.com/read", Content: "read content", Read: true, CreatedAt: now.Add(-time.Minute), UpdatedAt: now},
		{ID: "user-2-unread", UserID: "user-2", FeedID: "feed-user-2", ExternalID: "u2-unread", Title: "Other User", URL: "https://example.com/other", Content: "private", CreatedAt: now, UpdatedAt: now},
	})

	runtime := NewToolRuntime(NewService(st, nil, nil))
	ctx := auth.ContextWithUserID(context.Background(), "user-1")

	feedsResult, err := runtime.Execute(ctx, tools.Call{Name: "list_feeds"})
	if err != nil {
		t.Fatalf("list feeds: %v", err)
	}
	feedsOutput := feedsResult.Output.(listFeedsOutput)
	if len(feedsOutput.Feeds) != 1 || feedsOutput.Feeds[0].ID != "feed-user-1" || feedsOutput.Feeds[0].UnreadCount != 1 {
		t.Fatalf("unexpected list_feeds output: %#v", feedsOutput)
	}

	unreadResult, err := runtime.Execute(ctx, tools.Call{
		Name:      "get_feed_items",
		Arguments: map[string]any{"view": "unread", "contentMaxChars": 0},
	})
	if err != nil {
		t.Fatalf("get unread items: %v", err)
	}
	unreadOutput := unreadResult.Output.(feedItemsOutput)
	if len(unreadOutput.Items) != 1 || unreadOutput.Items[0].ID != "user-1-unread" {
		t.Fatalf("expected only user-1 unread item, got %#v", unreadOutput.Items)
	}
	if unreadOutput.Items[0].ContentExcerpt != "" || !unreadOutput.ContentOmitted {
		t.Fatalf("expected content excerpts to be omitted, got %#v", unreadOutput)
	}

	unreadStillUnread, err := st.ListFeedItems(context.Background(), "user-1", store.FeedItemFilter{View: "unread"})
	if err != nil {
		t.Fatalf("list unread after tool call: %v", err)
	}
	if len(unreadStillUnread) != 1 || unreadStillUnread[0].ID != "user-1-unread" {
		t.Fatalf("feed tool should not mark items read, got %#v", unreadStillUnread)
	}

	allResult, err := runtime.Execute(ctx, tools.Call{
		Name:      "get_feed_items",
		Arguments: map[string]any{"view": "all", "contentMaxChars": 0},
	})
	if err != nil {
		t.Fatalf("get all items: %v", err)
	}
	allOutput := allResult.Output.(feedItemsOutput)
	if len(allOutput.Items) != 2 {
		t.Fatalf("expected unread and read user-1 items, got %#v", allOutput.Items)
	}
}

func TestToolRuntimeFeedMatchingAmbiguityLimitAndTruncation(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	now := time.Now().UTC()
	publishedLatest := now.Add(-time.Minute)
	publishedOlder := now.Add(-time.Hour)
	createTestFeed(t, st, store.Feed{ID: "go-blog", UserID: "user-1", URL: "https://example.com/go-blog.xml", SiteURL: "https://example.com/go", Title: "Go Blog", NextCheckAt: now.Add(time.Hour), LastCheckedAt: &now, CreatedAt: now, UpdatedAt: now})
	createTestFeed(t, st, store.Feed{ID: "go-time", UserID: "user-1", URL: "https://example.com/go-time.xml", Title: "Go Time", NextCheckAt: now.Add(time.Hour), LastCheckedAt: &now, CreatedAt: now, UpdatedAt: now})
	createTestItems(t, st, []store.FeedItem{
		{ID: "older", UserID: "user-1", FeedID: "go-blog", ExternalID: "older", Title: "Older", URL: "https://example.com/older", PublishedAt: &publishedOlder, Content: strings.Repeat("older ", 20), CreatedAt: publishedOlder, UpdatedAt: now},
		{ID: "latest", UserID: "user-1", FeedID: "go-blog", ExternalID: "latest", Title: "Latest", URL: "https://example.com/latest", PublishedAt: &publishedLatest, Content: strings.Repeat("latest ", 20), CreatedAt: publishedLatest, UpdatedAt: now},
	})

	runtime := NewToolRuntime(NewService(st, nil, nil))
	ctx := auth.ContextWithUserID(context.Background(), "user-1")

	ambiguousResult, err := runtime.Execute(ctx, tools.Call{
		Name:      "get_feed_items",
		Arguments: map[string]any{"feed": "Go", "view": "all"},
	})
	if err != nil {
		t.Fatalf("get ambiguous items: %v", err)
	}
	ambiguousOutput := ambiguousResult.Output.(feedItemsOutput)
	if ambiguousOutput.Error == "" || len(ambiguousOutput.Candidates) != 2 || len(ambiguousOutput.Items) != 0 || !ambiguousResult.IsError {
		t.Fatalf("expected ambiguous feed candidates, got result=%#v output=%#v", ambiguousResult, ambiguousOutput)
	}

	contentMaxChars := 12
	matchedResult, err := runtime.Execute(ctx, tools.Call{
		Name: "get_feed_items",
		Arguments: map[string]any{
			"feed":            "Go Blog",
			"view":            "latest",
			"limit":           1,
			"contentMaxChars": contentMaxChars,
		},
	})
	if err != nil {
		t.Fatalf("get matched items: %v", err)
	}
	matchedOutput := matchedResult.Output.(feedItemsOutput)
	if matchedOutput.Error != "" {
		t.Fatalf("unexpected matched output error: %#v", matchedOutput)
	}
	if matchedOutput.Feed == nil || matchedOutput.Feed.ID != "go-blog" {
		t.Fatalf("expected Go Blog feed match, got %#v", matchedOutput.Feed)
	}
	if len(matchedOutput.Items) != 1 || matchedOutput.Items[0].ID != "latest" {
		t.Fatalf("expected latest item after limit, got %#v", matchedOutput.Items)
	}
	if !matchedOutput.Truncated || matchedOutput.AvailableItemCount != 2 {
		t.Fatalf("expected item limit truncation metadata, got %#v", matchedOutput)
	}
	if matchedOutput.Items[0].ContentExcerpt != "latest lates" || !matchedOutput.Items[0].ContentTruncated || !matchedOutput.ContentWasClipped {
		t.Fatalf("expected clipped content excerpt, got %#v", matchedOutput.Items[0])
	}
}

func createTestFeed(t *testing.T, st *store.Store, feed store.Feed) {
	t.Helper()
	if feed.PollingIntervalMinutes == 0 {
		feed.PollingIntervalMinutes = 30
	}
	if _, err := st.CreateFeed(context.Background(), feed); err != nil {
		t.Fatalf("create feed %s: %v", feed.ID, err)
	}
}

func createTestItems(t *testing.T, st *store.Store, items []store.FeedItem) {
	t.Helper()
	if _, err := st.CreateFeedItems(context.Background(), items); err != nil {
		t.Fatalf("create feed items: %v", err)
	}
}
