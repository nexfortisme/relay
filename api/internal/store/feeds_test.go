package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestStoreFeedsItemsDedupeAndFlags(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	feed, err := st.CreateFeed(ctx, Feed{
		ID:                     "feed-1",
		UserID:                 "user-1",
		URL:                    "https://example.com/rss.xml",
		Title:                  "Example",
		PollingIntervalMinutes: 30,
		NextCheckAt:            now.Add(30 * time.Minute),
		CreatedAt:              now,
		UpdatedAt:              now,
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}

	published := now.Add(-time.Hour)
	inserted, err := st.CreateFeedItems(ctx, []FeedItem{
		{
			ID:          "item-1",
			UserID:      "user-1",
			FeedID:      feed.ID,
			ExternalID:  "same-guid",
			Title:       "First",
			URL:         "https://example.com/first",
			PublishedAt: &published,
			Preview:     "Preview",
			Content:     "Content",
			MediaType:   "youtube",
			MediaURL:    "https://www.youtube.com/watch?v=abc123",
			CreatedAt:   now,
			UpdatedAt:   now,
		},
		{
			ID:         "item-2",
			UserID:     "user-1",
			FeedID:     feed.ID,
			ExternalID: "same-guid",
			Title:      "Duplicate",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	})
	if err != nil {
		t.Fatalf("create feed items: %v", err)
	}
	if len(inserted) != 1 {
		t.Fatalf("expected one inserted item after dedupe, got %d", len(inserted))
	}

	items, err := st.ListFeedItems(ctx, "user-1", FeedItemFilter{View: "unread"})
	if err != nil {
		t.Fatalf("list unread items: %v", err)
	}
	if len(items) != 1 || items[0].FeedTitle != "Example" {
		t.Fatalf("unexpected unread items: %#v", items)
	}
	if items[0].MediaType != "youtube" || items[0].MediaURL == "" {
		t.Fatalf("expected media fields to round-trip, got %#v", items[0])
	}

	read := true
	starred := true
	updated, err := st.UpdateFeedItemFlags(ctx, "user-1", inserted[0].ID, &read, &starred)
	if err != nil {
		t.Fatalf("update item flags: %v", err)
	}
	if !updated.Read || !updated.Starred {
		t.Fatalf("expected read+starred item, got %#v", updated)
	}

	unread, err := st.ListFeedItems(ctx, "user-1", FeedItemFilter{View: "unread"})
	if err != nil {
		t.Fatalf("list unread after update: %v", err)
	}
	if len(unread) != 0 {
		t.Fatalf("expected no unread items, got %d", len(unread))
	}

	starredItems, err := st.ListFeedItems(ctx, "user-1", FeedItemFilter{View: "starred"})
	if err != nil {
		t.Fatalf("list starred items: %v", err)
	}
	if len(starredItems) != 1 {
		t.Fatalf("expected one starred item, got %d", len(starredItems))
	}
}

func TestStoreDueFeedsAndCheckState(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := st.CreateFeed(ctx, Feed{
		ID:                     "due-feed",
		UserID:                 "user-1",
		URL:                    "https://example.com/rss.xml",
		Title:                  "Due",
		PollingIntervalMinutes: 15,
		NextCheckAt:            now.Add(-time.Minute),
		CreatedAt:              now,
		UpdatedAt:              now,
	}); err != nil {
		t.Fatalf("create due feed: %v", err)
	}
	if _, err := st.CreateFeed(ctx, Feed{
		ID:                     "later-feed",
		UserID:                 "user-1",
		URL:                    "https://example.com/later.xml",
		Title:                  "Later",
		PollingIntervalMinutes: 15,
		NextCheckAt:            now.Add(time.Hour),
		CreatedAt:              now,
		UpdatedAt:              now,
	}); err != nil {
		t.Fatalf("create later feed: %v", err)
	}

	due, err := st.ListDueFeeds(ctx, now)
	if err != nil {
		t.Fatalf("list due feeds: %v", err)
	}
	if len(due) != 1 || due[0].ID != "due-feed" {
		t.Fatalf("unexpected due feeds: %#v", due)
	}

	next := now.Add(15 * time.Minute)
	if err := st.UpdateFeedCheckState(ctx, "due-feed", now, next, ""); err != nil {
		t.Fatalf("update check state: %v", err)
	}
	feed, err := st.GetFeed(ctx, "user-1", "due-feed")
	if err != nil {
		t.Fatalf("get feed: %v", err)
	}
	if feed.LastCheckedAt == nil || !feed.NextCheckAt.Equal(next) {
		t.Fatalf("unexpected check state: %#v", feed)
	}
}

func TestStoreMarkFeedItemsReadScopesFeedAndUser(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	feeds := []Feed{
		{
			ID:                     "feed-1",
			UserID:                 "user-1",
			URL:                    "https://example.com/rss.xml",
			Title:                  "Primary",
			PollingIntervalMinutes: 30,
			NextCheckAt:            now.Add(30 * time.Minute),
			CreatedAt:              now,
			UpdatedAt:              now,
		},
		{
			ID:                     "feed-2",
			UserID:                 "user-1",
			URL:                    "https://example.com/other.xml",
			Title:                  "Other",
			PollingIntervalMinutes: 30,
			NextCheckAt:            now.Add(30 * time.Minute),
			CreatedAt:              now,
			UpdatedAt:              now,
		},
		{
			ID:                     "feed-3",
			UserID:                 "user-2",
			URL:                    "https://example.net/rss.xml",
			Title:                  "Other User",
			PollingIntervalMinutes: 30,
			NextCheckAt:            now.Add(30 * time.Minute),
			CreatedAt:              now,
			UpdatedAt:              now,
		},
	}
	for _, feed := range feeds {
		if _, err := st.CreateFeed(ctx, feed); err != nil {
			t.Fatalf("create feed %s: %v", feed.ID, err)
		}
	}
	if _, err := st.CreateFeedItems(ctx, []FeedItem{
		{ID: "item-1", UserID: "user-1", FeedID: "feed-1", ExternalID: "guid-1", Title: "First", CreatedAt: now, UpdatedAt: now},
		{ID: "item-2", UserID: "user-1", FeedID: "feed-1", ExternalID: "guid-2", Title: "Second", CreatedAt: now, UpdatedAt: now},
		{ID: "item-3", UserID: "user-1", FeedID: "feed-2", ExternalID: "guid-3", Title: "Third", CreatedAt: now, UpdatedAt: now},
		{ID: "item-4", UserID: "user-2", FeedID: "feed-3", ExternalID: "guid-4", Title: "Fourth", CreatedAt: now, UpdatedAt: now},
	}); err != nil {
		t.Fatalf("create feed items: %v", err)
	}

	updated, err := st.MarkFeedItemsRead(ctx, "user-1", "feed-1")
	if err != nil {
		t.Fatalf("mark feed items read: %v", err)
	}
	if updated != 2 {
		t.Fatalf("expected two updated rows, got %d", updated)
	}
	feed1, err := st.GetFeed(ctx, "user-1", "feed-1")
	if err != nil {
		t.Fatalf("get feed 1: %v", err)
	}
	if feed1.UnreadCount != 0 {
		t.Fatalf("expected feed 1 unread count 0, got %d", feed1.UnreadCount)
	}
	feed2, err := st.GetFeed(ctx, "user-1", "feed-2")
	if err != nil {
		t.Fatalf("get feed 2: %v", err)
	}
	if feed2.UnreadCount != 1 {
		t.Fatalf("expected feed 2 unread count untouched, got %d", feed2.UnreadCount)
	}
	feed3, err := st.GetFeed(ctx, "user-2", "feed-3")
	if err != nil {
		t.Fatalf("get feed 3: %v", err)
	}
	if feed3.UnreadCount != 1 {
		t.Fatalf("expected other user feed unread count untouched, got %d", feed3.UnreadCount)
	}
	if _, err := st.MarkFeedItemsRead(ctx, "user-2", "feed-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found for other user's feed, got %v", err)
	}
}

func TestStoreDeleteFeedCascadesItemsAndScopesUser(t *testing.T) {
	st, err := New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	if _, err := st.CreateFeed(ctx, Feed{
		ID:                     "feed-1",
		UserID:                 "user-1",
		URL:                    "https://example.com/rss.xml",
		Title:                  "Example",
		PollingIntervalMinutes: 30,
		NextCheckAt:            now.Add(30 * time.Minute),
		CreatedAt:              now,
		UpdatedAt:              now,
	}); err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if _, err := st.CreateFeedItems(ctx, []FeedItem{
		{
			ID:         "item-1",
			UserID:     "user-1",
			FeedID:     "feed-1",
			ExternalID: "guid-1",
			Title:      "First",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	}); err != nil {
		t.Fatalf("create feed items: %v", err)
	}

	if err := st.DeleteFeed(ctx, "user-2", "feed-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected not found for other user delete, got %v", err)
	}
	if _, err := st.GetFeed(ctx, "user-1", "feed-1"); err != nil {
		t.Fatalf("feed should still exist after other user delete: %v", err)
	}

	if err := st.DeleteFeed(ctx, "user-1", "feed-1"); err != nil {
		t.Fatalf("delete feed: %v", err)
	}
	if _, err := st.GetFeed(ctx, "user-1", "feed-1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected feed not found after delete, got %v", err)
	}
	items, err := st.ListFeedItems(ctx, "user-1", FeedItemFilter{View: "all"})
	if err != nil {
		t.Fatalf("list items after delete: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("expected feed items to cascade delete, got %#v", items)
	}
}
