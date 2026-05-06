package feeds

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nexfortisme/relay/internal/store"
)

type serviceTestFeedItem struct {
	ID          string
	Title       string
	PublishedAt time.Time
}

func TestServicePollOnlyStoresItemsNewerThanInitialBackfill(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	currentItems := []serviceTestFeedItem{
		{ID: "guid-new", Title: "New", PublishedAt: now},
		{ID: "guid-old", Title: "Old", PublishedAt: now.Add(-time.Hour)},
		{ID: "guid-older", Title: "Older", PublishedAt: now.Add(-2 * time.Hour)},
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rss+xml")
		_, _ = io.WriteString(w, serviceTestRSS(server.URL, currentItems))
	}))
	defer server.Close()

	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	service := NewService(st, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	feed, inserted, err := service.CreateFeed(ctx, "user-1", CreateFeedRequest{
		URL:                    server.URL + "/rss.xml",
		PollingIntervalMinutes: 30,
		Backfill:               BackfillOptions{Mode: "latest", Limit: 1},
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	if len(inserted) != 1 || inserted[0].ExternalID != "guid-new" {
		t.Fatalf("expected only latest item to be backfilled, got %#v", inserted)
	}

	service.pollFeed(ctx, feed)
	items := listServiceTestFeedItems(t, st, feed.ID)
	if got := serviceTestExternalIDs(items); strings.Join(got, ",") != "guid-new" {
		t.Fatalf("expected skipped older items to stay skipped, got %v", got)
	}

	currentItems = append([]serviceTestFeedItem{
		{ID: "guid-newer", Title: "Newer", PublishedAt: now.Add(time.Hour)},
	}, currentItems...)
	service.pollFeed(ctx, feed)
	items = listServiceTestFeedItems(t, st, feed.ID)
	if got := serviceTestExternalIDs(items); strings.Join(got, ",") != "guid-newer,guid-new" {
		t.Fatalf("expected only the newer item to be added, got %v", got)
	}
}

func serviceTestRSS(baseURL string, items []serviceTestFeedItem) string {
	var builder strings.Builder
	builder.WriteString(`<?xml version="1.0"?><rss version="2.0"><channel>`)
	builder.WriteString(`<title>Watermark Feed</title>`)
	builder.WriteString(`<link>` + baseURL + `</link>`)
	for _, item := range items {
		_, _ = fmt.Fprintf(
			&builder,
			`<item><title>%s</title><link>%s/%s</link><guid>%s</guid><pubDate>%s</pubDate><description>%s preview</description></item>`,
			item.Title,
			baseURL,
			item.ID,
			item.ID,
			item.PublishedAt.Format(time.RFC1123Z),
			item.Title,
		)
	}
	builder.WriteString(`</channel></rss>`)
	return builder.String()
}

func listServiceTestFeedItems(t *testing.T, st *store.Store, feedID string) []store.FeedItem {
	t.Helper()
	items, err := st.ListFeedItems(context.Background(), "user-1", store.FeedItemFilter{
		View:   "all",
		FeedID: feedID,
	})
	if err != nil {
		t.Fatalf("list feed items: %v", err)
	}
	return items
}

func serviceTestExternalIDs(items []store.FeedItem) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ExternalID)
	}
	return ids
}
