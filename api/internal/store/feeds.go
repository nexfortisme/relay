package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Feed struct {
	ID                     string     `json:"id"`
	UserID                 string     `json:"-"`
	URL                    string     `json:"url"`
	Title                  string     `json:"title"`
	SiteURL                string     `json:"siteUrl"`
	Description            string     `json:"description,omitempty"`
	PollingIntervalMinutes int        `json:"pollingIntervalMinutes"`
	AutoSummarize          bool       `json:"autoSummarize"`
	AutoAddToNotebook      bool       `json:"autoAddToNotebook"`
	NotebookID             string     `json:"notebookId,omitempty"`
	LastCheckedAt          *time.Time `json:"lastCheckedAt,omitempty"`
	NextCheckAt            time.Time  `json:"nextCheckAt"`
	LastError              string     `json:"lastError,omitempty"`
	UnreadCount            int        `json:"unreadCount"`
	CreatedAt              time.Time  `json:"createdAt"`
	UpdatedAt              time.Time  `json:"updatedAt"`
}

type FeedItem struct {
	ID            string     `json:"id"`
	UserID        string     `json:"-"`
	FeedID        string     `json:"feedId"`
	FeedTitle     string     `json:"feedTitle,omitempty"`
	ExternalID    string     `json:"externalId"`
	Title         string     `json:"title"`
	URL           string     `json:"url"`
	Author        string     `json:"author,omitempty"`
	PublishedAt   *time.Time `json:"publishedAt,omitempty"`
	Preview       string     `json:"preview"`
	Content       string     `json:"content,omitempty"`
	MediaType     string     `json:"mediaType,omitempty"`
	MediaURL      string     `json:"mediaUrl,omitempty"`
	Summary       string     `json:"summary,omitempty"`
	SummaryStatus string     `json:"summaryStatus,omitempty"`
	SummaryError  string     `json:"summaryError,omitempty"`
	Read          bool       `json:"read"`
	Starred       bool       `json:"starred"`
	CreatedAt     time.Time  `json:"createdAt"`
	UpdatedAt     time.Time  `json:"updatedAt"`
}

type FeedUpdate struct {
	Title                  *string
	PollingIntervalMinutes *int
	AutoSummarize          *bool
	AutoAddToNotebook      *bool
	NotebookID             *string
}

type FeedItemFilter struct {
	View   string
	FeedID string
}

type FeedItemCursor struct {
	ExternalID  string
	PublishedAt *time.Time
}

func (s *Store) CreateFeed(ctx context.Context, feed Feed) (Feed, error) {
	now := feed.CreatedAt
	if now.IsZero() {
		now = time.Now().UTC()
	}
	if feed.UpdatedAt.IsZero() {
		feed.UpdatedAt = now
	}
	if feed.NextCheckAt.IsZero() {
		feed.NextCheckAt = now.Add(time.Duration(feed.PollingIntervalMinutes) * time.Minute)
	}
	if feed.PollingIntervalMinutes <= 0 {
		feed.PollingIntervalMinutes = 30
	}

	_, err := s.db.ExecContext(ctx, `
INSERT INTO feeds(id, user_id, url, title, site_url, description, polling_interval_minutes, auto_summarize, auto_add_to_notebook, notebook_id, last_checked_at, next_check_at, last_error, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, feed.ID, feed.UserID, feed.URL, feed.Title, feed.SiteURL, feed.Description, feed.PollingIntervalMinutes, feed.AutoSummarize, feed.AutoAddToNotebook, feed.NotebookID, nullableTime(feed.LastCheckedAt), feed.NextCheckAt.UTC(), feed.LastError, now.UTC(), feed.UpdatedAt.UTC())
	if err != nil {
		return Feed{}, fmt.Errorf("insert feed: %w", err)
	}
	feed.CreatedAt = now.UTC()
	feed.UpdatedAt = feed.UpdatedAt.UTC()
	feed.NextCheckAt = feed.NextCheckAt.UTC()
	return feed, nil
}

func (s *Store) ListFeeds(ctx context.Context, userID string) ([]Feed, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT f.id, f.user_id, f.url, f.title, f.site_url, f.description, f.polling_interval_minutes, f.auto_summarize, f.auto_add_to_notebook, f.notebook_id, f.last_checked_at, f.next_check_at, f.last_error, f.created_at, f.updated_at,
       COALESCE(SUM(CASE WHEN i.read = 0 THEN 1 ELSE 0 END), 0) AS unread_count
FROM feeds f
LEFT JOIN feed_items i ON i.feed_id = f.id
WHERE f.user_id = ?
GROUP BY f.id
ORDER BY lower(f.title) ASC, f.created_at ASC
`, userID)
	if err != nil {
		return nil, fmt.Errorf("list feeds: %w", err)
	}
	defer rows.Close()
	out := make([]Feed, 0)
	for rows.Next() {
		feed, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, feed)
	}
	return out, rows.Err()
}

func (s *Store) GetFeed(ctx context.Context, userID, feedID string) (Feed, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT f.id, f.user_id, f.url, f.title, f.site_url, f.description, f.polling_interval_minutes, f.auto_summarize, f.auto_add_to_notebook, f.notebook_id, f.last_checked_at, f.next_check_at, f.last_error, f.created_at, f.updated_at,
       COALESCE(SUM(CASE WHEN i.read = 0 THEN 1 ELSE 0 END), 0) AS unread_count
FROM feeds f
LEFT JOIN feed_items i ON i.feed_id = f.id
WHERE f.user_id = ? AND f.id = ?
GROUP BY f.id
`, userID, feedID)
	feed, err := scanFeed(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Feed{}, ErrNotFound
	}
	return feed, err
}

func (s *Store) ListDueFeeds(ctx context.Context, now time.Time) ([]Feed, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT f.id, f.user_id, f.url, f.title, f.site_url, f.description, f.polling_interval_minutes, f.auto_summarize, f.auto_add_to_notebook, f.notebook_id, f.last_checked_at, f.next_check_at, f.last_error, f.created_at, f.updated_at,
       0 AS unread_count
FROM feeds f
WHERE f.next_check_at <= ?
ORDER BY f.next_check_at ASC
`, now.UTC())
	if err != nil {
		return nil, fmt.Errorf("list due feeds: %w", err)
	}
	defer rows.Close()
	out := make([]Feed, 0)
	for rows.Next() {
		feed, err := scanFeed(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, feed)
	}
	return out, rows.Err()
}

func (s *Store) UpdateFeed(ctx context.Context, userID, feedID string, update FeedUpdate) (Feed, error) {
	feed, err := s.GetFeed(ctx, userID, feedID)
	if err != nil {
		return Feed{}, err
	}
	if update.Title != nil {
		feed.Title = strings.TrimSpace(*update.Title)
	}
	if update.PollingIntervalMinutes != nil {
		feed.PollingIntervalMinutes = *update.PollingIntervalMinutes
	}
	if update.AutoSummarize != nil {
		feed.AutoSummarize = *update.AutoSummarize
	}
	if update.AutoAddToNotebook != nil {
		feed.AutoAddToNotebook = *update.AutoAddToNotebook
	}
	if update.NotebookID != nil {
		feed.NotebookID = strings.TrimSpace(*update.NotebookID)
	}
	feed.UpdatedAt = time.Now().UTC()

	if feed.Title == "" {
		return Feed{}, fmt.Errorf("title is required")
	}
	if feed.PollingIntervalMinutes <= 0 {
		return Feed{}, fmt.Errorf("polling interval is required")
	}

	_, err = s.db.ExecContext(ctx, `
UPDATE feeds
SET title = ?, polling_interval_minutes = ?, auto_summarize = ?, auto_add_to_notebook = ?, notebook_id = ?, updated_at = ?
WHERE user_id = ? AND id = ?
`, feed.Title, feed.PollingIntervalMinutes, feed.AutoSummarize, feed.AutoAddToNotebook, feed.NotebookID, feed.UpdatedAt, userID, feedID)
	if err != nil {
		return Feed{}, fmt.Errorf("update feed: %w", err)
	}
	return s.GetFeed(ctx, userID, feedID)
}

func (s *Store) DeleteFeed(ctx context.Context, userID, feedID string) error {
	result, err := s.db.ExecContext(ctx, `DELETE FROM feeds WHERE user_id = ? AND id = ?`, userID, feedID)
	if err != nil {
		return fmt.Errorf("delete feed: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) UpdateFeedCheckState(ctx context.Context, feedID string, checkedAt time.Time, nextCheckAt time.Time, lastError string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE feeds
SET last_checked_at = ?, next_check_at = ?, last_error = ?, updated_at = ?
WHERE id = ?
`, checkedAt.UTC(), nextCheckAt.UTC(), lastError, time.Now().UTC(), feedID)
	if err != nil {
		return fmt.Errorf("update feed check state: %w", err)
	}
	return nil
}

func (s *Store) UpdateFeedNextCheck(ctx context.Context, userID, feedID string, nextCheckAt time.Time) error {
	result, err := s.db.ExecContext(ctx, `
UPDATE feeds
SET next_check_at = ?, updated_at = ?
WHERE user_id = ? AND id = ?
`, nextCheckAt.UTC(), time.Now().UTC(), userID, feedID)
	if err != nil {
		return fmt.Errorf("update feed next check: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) CreateFeedItems(ctx context.Context, items []FeedItem) ([]FeedItem, error) {
	if len(items) == 0 {
		return nil, nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin feed item insert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	inserted := make([]FeedItem, 0, len(items))
	for _, item := range items {
		now := item.CreatedAt
		if now.IsZero() {
			now = time.Now().UTC()
		}
		if item.UpdatedAt.IsZero() {
			item.UpdatedAt = now
		}
		result, err := tx.ExecContext(ctx, `
INSERT OR IGNORE INTO feed_items(id, user_id, feed_id, external_id, title, url, author, published_at, preview, content, media_type, media_url, summary, summary_status, summary_error, read, starred, created_at, updated_at)
VALUES(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
`, item.ID, item.UserID, item.FeedID, item.ExternalID, item.Title, item.URL, item.Author, nullableTime(item.PublishedAt), item.Preview, item.Content, item.MediaType, item.MediaURL, item.Summary, item.SummaryStatus, item.SummaryError, item.Read, item.Starred, now.UTC(), item.UpdatedAt.UTC())
		if err != nil {
			return nil, fmt.Errorf("insert feed item: %w", err)
		}
		affected, _ := result.RowsAffected()
		if affected > 0 {
			item.CreatedAt = now.UTC()
			item.UpdatedAt = item.UpdatedAt.UTC()
			inserted = append(inserted, item)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit feed item insert: %w", err)
	}
	return inserted, nil
}

func (s *Store) LatestFeedItemCursor(ctx context.Context, userID, feedID string) (FeedItemCursor, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT external_id, published_at
FROM feed_items
WHERE user_id = ? AND feed_id = ?
ORDER BY COALESCE(published_at, created_at) DESC, created_at DESC
LIMIT 1
`, userID, feedID)
	var cursor FeedItemCursor
	var published sql.NullTime
	if err := row.Scan(&cursor.ExternalID, &published); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return FeedItemCursor{}, ErrNotFound
		}
		return FeedItemCursor{}, fmt.Errorf("latest feed item cursor: %w", err)
	}
	if published.Valid {
		t := published.Time
		cursor.PublishedAt = &t
	}
	return cursor, nil
}

func (s *Store) ListFeedItems(ctx context.Context, userID string, filter FeedItemFilter) ([]FeedItem, error) {
	where := []string{"i.user_id = ?"}
	args := []any{userID}
	switch filter.View {
	case "starred":
		where = append(where, "i.starred = 1")
	case "all":
	case "", "unread":
		where = append(where, "i.read = 0")
	default:
		return nil, fmt.Errorf("unknown feed item view %q", filter.View)
	}
	if filter.FeedID != "" {
		where = append(where, "i.feed_id = ?")
		args = append(args, filter.FeedID)
	}

	query := fmt.Sprintf(`
SELECT i.id, i.user_id, i.feed_id, f.title, i.external_id, i.title, i.url, i.author, i.published_at, i.preview, i.content, i.media_type, i.media_url, i.summary, i.summary_status, i.summary_error, i.read, i.starred, i.created_at, i.updated_at
FROM feed_items i
JOIN feeds f ON f.id = i.feed_id
WHERE %s
ORDER BY COALESCE(i.published_at, i.created_at) DESC, i.created_at DESC
LIMIT 300
`, strings.Join(where, " AND "))
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list feed items: %w", err)
	}
	defer rows.Close()
	out := make([]FeedItem, 0)
	for rows.Next() {
		item, err := scanFeedItem(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *Store) GetFeedItem(ctx context.Context, userID, itemID string) (FeedItem, error) {
	row := s.db.QueryRowContext(ctx, `
SELECT i.id, i.user_id, i.feed_id, f.title, i.external_id, i.title, i.url, i.author, i.published_at, i.preview, i.content, i.media_type, i.media_url, i.summary, i.summary_status, i.summary_error, i.read, i.starred, i.created_at, i.updated_at
FROM feed_items i
JOIN feeds f ON f.id = i.feed_id
WHERE i.user_id = ? AND i.id = ?
`, userID, itemID)
	item, err := scanFeedItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return FeedItem{}, ErrNotFound
	}
	return item, err
}

func (s *Store) UpdateFeedItemFlags(ctx context.Context, userID, itemID string, read *bool, starred *bool) (FeedItem, error) {
	item, err := s.GetFeedItem(ctx, userID, itemID)
	if err != nil {
		return FeedItem{}, err
	}
	if read != nil {
		item.Read = *read
	}
	if starred != nil {
		item.Starred = *starred
	}
	_, err = s.db.ExecContext(ctx, `
UPDATE feed_items
SET read = ?, starred = ?, updated_at = ?
WHERE user_id = ? AND id = ?
`, item.Read, item.Starred, time.Now().UTC(), userID, itemID)
	if err != nil {
		return FeedItem{}, fmt.Errorf("update feed item flags: %w", err)
	}
	return s.GetFeedItem(ctx, userID, itemID)
}

func (s *Store) MarkFeedItemsRead(ctx context.Context, userID, feedID string) (int, error) {
	if _, err := s.GetFeed(ctx, userID, feedID); err != nil {
		return 0, err
	}
	result, err := s.db.ExecContext(ctx, `
UPDATE feed_items
SET read = 1, updated_at = ?
WHERE user_id = ? AND feed_id = ? AND read = 0
`, time.Now().UTC(), userID, feedID)
	if err != nil {
		return 0, fmt.Errorf("mark feed items read: %w", err)
	}
	affected, _ := result.RowsAffected()
	return int(affected), nil
}

func (s *Store) SetFeedItemSummaryStatus(ctx context.Context, userID, itemID, status, errText string) error {
	_, err := s.db.ExecContext(ctx, `
UPDATE feed_items
SET summary_status = ?, summary_error = ?, updated_at = ?
WHERE user_id = ? AND id = ?
`, status, errText, time.Now().UTC(), userID, itemID)
	if err != nil {
		return fmt.Errorf("set feed item summary status: %w", err)
	}
	return nil
}

func (s *Store) SetFeedItemSummary(ctx context.Context, userID, itemID, summary string) (FeedItem, error) {
	_, err := s.db.ExecContext(ctx, `
UPDATE feed_items
SET summary = ?, summary_status = 'ready', summary_error = '', updated_at = ?
WHERE user_id = ? AND id = ?
`, strings.TrimSpace(summary), time.Now().UTC(), userID, itemID)
	if err != nil {
		return FeedItem{}, fmt.Errorf("set feed item summary: %w", err)
	}
	return s.GetFeedItem(ctx, userID, itemID)
}

type feedScanner interface {
	Scan(dest ...any) error
}

func scanFeed(scanner feedScanner) (Feed, error) {
	var feed Feed
	var lastChecked sql.NullTime
	if err := scanner.Scan(
		&feed.ID,
		&feed.UserID,
		&feed.URL,
		&feed.Title,
		&feed.SiteURL,
		&feed.Description,
		&feed.PollingIntervalMinutes,
		&feed.AutoSummarize,
		&feed.AutoAddToNotebook,
		&feed.NotebookID,
		&lastChecked,
		&feed.NextCheckAt,
		&feed.LastError,
		&feed.CreatedAt,
		&feed.UpdatedAt,
		&feed.UnreadCount,
	); err != nil {
		return Feed{}, err
	}
	if lastChecked.Valid {
		t := lastChecked.Time
		feed.LastCheckedAt = &t
	}
	return feed, nil
}

func scanFeedItem(scanner feedScanner) (FeedItem, error) {
	var item FeedItem
	var published sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.UserID,
		&item.FeedID,
		&item.FeedTitle,
		&item.ExternalID,
		&item.Title,
		&item.URL,
		&item.Author,
		&published,
		&item.Preview,
		&item.Content,
		&item.MediaType,
		&item.MediaURL,
		&item.Summary,
		&item.SummaryStatus,
		&item.SummaryError,
		&item.Read,
		&item.Starred,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return FeedItem{}, err
	}
	if published.Valid {
		t := published.Time
		item.PublishedAt = &t
	}
	return item, nil
}

func nullableTime(t *time.Time) any {
	if t == nil || t.IsZero() {
		return nil
	}
	return t.UTC()
}
