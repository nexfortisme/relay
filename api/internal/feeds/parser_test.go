package feeds

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/nexfortisme/relay/internal/store"
)

func TestParseRSSFeed(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<rss version="2.0" xmlns:content="http://purl.org/rss/1.0/modules/content/" xmlns:dc="http://purl.org/dc/elements/1.1/">
  <channel>
    <title>Example RSS</title>
    <link>https://example.com</link>
    <description>News &amp; notes</description>
    <item>
      <title>First item</title>
      <link>/posts/first</link>
      <guid>first-guid</guid>
      <pubDate>Mon, 04 May 2026 20:04:00 -0400</pubDate>
      <dc:creator>Riley</dc:creator>
      <description><![CDATA[<p>Hello <strong>world</strong>.</p>]]></description>
      <content:encoded><![CDATA[<p>Longer <strong>body</strong>.</p>]]></content:encoded>
    </item>
  </channel>
</rss>`)

	feed, err := parseFeed("https://example.com/rss.xml", raw)
	if err != nil {
		t.Fatalf("parse rss: %v", err)
	}
	if feed.Title != "Example RSS" {
		t.Fatalf("unexpected title %q", feed.Title)
	}
	if len(feed.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(feed.Items))
	}
	item := feed.Items[0]
	if item.URL != "https://example.com/posts/first" {
		t.Fatalf("expected absolute item URL, got %q", item.URL)
	}
	if item.Preview != "Longer body." {
		t.Fatalf("expected cleaned preview, got %q", item.Preview)
	}
	if item.Author != "Riley" {
		t.Fatalf("expected namespaced author, got %q", item.Author)
	}
	if item.PublishedAt == nil {
		t.Fatal("expected parsed published date")
	}
}

func TestParseRSSDetectsVideoMedia(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<rss version="2.0"><channel>
  <title>Videos</title>
  <item>
    <title>Episode</title>
    <link>https://example.com/watch</link>
    <guid>episode-1</guid>
    <enclosure url="https://cdn.example.com/episode.mp4" type="video/mp4"/>
    <description>Video item</description>
  </item>
  <item>
    <title>YouTube item</title>
    <link>https://www.youtube.com/watch?v=abc123</link>
    <guid>yt-1</guid>
  </item>
</channel></rss>`)

	feed, err := parseFeed("https://example.com/rss.xml", raw)
	if err != nil {
		t.Fatalf("parse rss: %v", err)
	}
	if feed.Items[0].MediaType != "video" || feed.Items[0].MediaURL != "https://cdn.example.com/episode.mp4" {
		t.Fatalf("expected direct video media, got %#v", feed.Items[0])
	}
	if feed.Items[1].MediaType != "youtube" || feed.Items[1].MediaURL != "https://www.youtube.com/watch?v=abc123" {
		t.Fatalf("expected youtube media, got %#v", feed.Items[1])
	}
}

func TestParseAtomFeed(t *testing.T) {
	raw := []byte(`<?xml version="1.0"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Example Atom</title>
  <link href="https://example.com/"/>
  <entry>
    <id>tag:example.com,2026:first</id>
    <title>Atom item</title>
    <link rel="alternate" href="https://example.com/a"/>
    <published>2026-05-04T20:04:00Z</published>
    <author><name>Alex</name></author>
    <summary>Short summary</summary>
  </entry>
</feed>`)

	feed, err := parseFeed("https://example.com/atom.xml", raw)
	if err != nil {
		t.Fatalf("parse atom: %v", err)
	}
	if feed.Title != "Example Atom" {
		t.Fatalf("unexpected title %q", feed.Title)
	}
	if len(feed.Items) != 1 || feed.Items[0].Author != "Alex" {
		t.Fatalf("unexpected items: %#v", feed.Items)
	}
}

func TestParseInvalidFeed(t *testing.T) {
	_, err := parseFeed("https://example.com/feed", []byte("<html>nope</html>"))
	if err == nil || !strings.Contains(err.Error(), "RSS or Atom") {
		t.Fatalf("expected invalid feed error, got %v", err)
	}
}

func TestSummaryQueueHasTwoSlots(t *testing.T) {
	service := NewService(nil, nil, nil)
	if cap(service.summarySem) != 2 {
		t.Fatalf("expected 2 summary slots, got %d", cap(service.summarySem))
	}
}

func TestSummaryPromptUsesTargetCharactersForInboxDescription(t *testing.T) {
	prompt := summarySystemPrompt("summary", 42)
	if !strings.Contains(prompt, "80 characters") {
		t.Fatalf("expected prompt to clamp small target to 80 characters, got %q", prompt)
	}
	if !strings.Contains(prompt, "no markdown") {
		t.Fatalf("expected prompt to forbid markdown, got %q", prompt)
	}
}

func TestSummarizeItemRejectsVideoItems(t *testing.T) {
	st, err := store.New(":memory:")
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	defer st.Close()

	ctx := context.Background()
	now := time.Now().UTC()
	feed, err := st.CreateFeed(ctx, store.Feed{
		ID:                     "feed-1",
		UserID:                 "user-1",
		URL:                    "https://example.com/rss.xml",
		Title:                  "Videos",
		PollingIntervalMinutes: 30,
		NextCheckAt:            now.Add(time.Hour),
		CreatedAt:              now,
		UpdatedAt:              now,
	})
	if err != nil {
		t.Fatalf("create feed: %v", err)
	}
	items, err := st.CreateFeedItems(ctx, []store.FeedItem{
		{
			ID:         "item-1",
			UserID:     "user-1",
			FeedID:     feed.ID,
			ExternalID: "yt-1",
			Title:      "Video",
			URL:        "https://www.youtube.com/watch?v=abc123",
			MediaType:  "youtube",
			MediaURL:   "https://www.youtube.com/watch?v=abc123",
			CreatedAt:  now,
			UpdatedAt:  now,
		},
	})
	if err != nil {
		t.Fatalf("create item: %v", err)
	}

	service := NewService(st, nil, nil)
	if _, err := service.SummarizeItem(ctx, "user-1", items[0].ID, "summary"); !errors.Is(err, ErrVideoSummaryUnsupported) {
		t.Fatalf("expected video summary error, got %v", err)
	}
}
