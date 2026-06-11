package feeds

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/nexfortisme/relay/internal/llm"
	"github.com/nexfortisme/relay/internal/prompts"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

const (
	defaultPollingMinutes     = 30
	defaultSummaryTargetChars = 150
	maxSummaryTargetChars     = 320
	minPollingMinutes         = 5
	minSummaryTargetChars     = 80
	schedulerInterval         = 30 * time.Second
)

var (
	ErrVideoSummaryUnsupported = errors.New("video feed items cannot be summarized")

	feedSummaryExpandedPrompt        = prompts.MustLoad(prompts.FeedSummaryExpanded)
	feedSummaryPreviewPromptTemplate = prompts.MustLoad(prompts.FeedSummaryPreview)
	feedSummaryUserPromptTemplate    = prompts.MustLoad(prompts.FeedSummaryUser)
	feedNameSystemPrompt             = prompts.MustLoad(prompts.FeedNameSystem)
	feedNameUserPromptTemplate       = prompts.MustLoad(prompts.FeedNameUser)
)

type LLMSettings struct {
	LLMURL    string
	LLMModel  string
	LLMAPIKey string
}

// SettingsLoader resolves the per-user LLM settings used for summaries and
// feed naming; it is injected by the app wiring to avoid depending on the
// chat package directly.
type SettingsLoader func(ctx context.Context, userID string) LLMSettings

// Service manages RSS/Atom subscriptions: validating and creating feeds,
// polling them on a schedule, and generating per-item LLM summaries.
type Service struct {
	store    *store.Store
	settings SettingsLoader
	logger   *slog.Logger
	client   *http.Client

	// summarySem bounds concurrent LLM summary generations.
	summarySem chan struct{}
}

type CheckResult struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	SiteURL     string `json:"siteUrl"`
	Description string `json:"description"`
	ItemCount   int    `json:"itemCount"`
}

type CreateFeedRequest struct {
	URL                    string
	Title                  string
	PollingIntervalMinutes int
	AutoSummarize          bool
	AutoAddToNotebook      bool
	NotebookID             string
	Backfill               BackfillOptions
}

type BackfillOptions struct {
	Mode  string
	Limit int
	Since *time.Time
}

type ItemPatch struct {
	Read    *bool
	Starred *bool
}

func NewService(st *store.Store, settings SettingsLoader, logger *slog.Logger) *Service {
	if settings == nil {
		settings = func(context.Context, string) LLMSettings { return LLMSettings{} }
	}
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		store:      st,
		settings:   settings,
		logger:     logger,
		client:     &http.Client{Timeout: 20 * time.Second},
		summarySem: make(chan struct{}, 2),
	}
}

// Start launches the background polling scheduler; it returns immediately and
// the scheduler stops when ctx is cancelled.
func (s *Service) Start(ctx context.Context) {
	go s.schedulerLoop(ctx)
}

// CheckFeed fetches and parses a candidate URL without persisting anything,
// so the UI can preview a feed before the user subscribes.
func (s *Service) CheckFeed(ctx context.Context, rawURL string) (CheckResult, error) {
	parsed, err := s.fetchFeed(ctx, rawURL)
	if err != nil {
		return CheckResult{}, err
	}
	return CheckResult{
		URL:         parsed.URL,
		Title:       parsed.Title,
		SiteURL:     parsed.SiteURL,
		Description: parsed.Description,
		ItemCount:   len(parsed.Items),
	}, nil
}

// CreateFeed subscribes a user to a feed and backfills its initial items.
// The title falls back through: user-supplied → feed's own title → LLM-generated
// name → feed hostname.
func (s *Service) CreateFeed(ctx context.Context, userID string, req CreateFeedRequest) (store.Feed, []store.FeedItem, error) {
	parsed, err := s.fetchFeed(ctx, req.URL)
	if err != nil {
		return store.Feed{}, nil, err
	}
	now := time.Now().UTC()
	pollingMinutes := clampPollingMinutes(req.PollingIntervalMinutes)
	title := strings.TrimSpace(req.Title)
	if title == "" {
		title = strings.TrimSpace(parsed.Title)
	}
	if title == "" {
		title = s.generateFeedName(ctx, userID, parsed)
	}
	if title == "" {
		title = hostname(parsed.URL)
	}

	feed := store.Feed{
		ID:                     uuid.NewString(),
		UserID:                 userID,
		URL:                    parsed.URL,
		Title:                  title,
		SiteURL:                parsed.SiteURL,
		Description:            parsed.Description,
		PollingIntervalMinutes: pollingMinutes,
		AutoSummarize:          req.AutoSummarize,
		AutoAddToNotebook:      req.AutoAddToNotebook,
		NotebookID:             strings.TrimSpace(req.NotebookID),
		LastCheckedAt:          &now,
		NextCheckAt:            now.Add(time.Duration(pollingMinutes) * time.Minute),
		CreatedAt:              now,
		UpdatedAt:              now,
	}
	created, err := s.store.CreateFeed(ctx, feed)
	if err != nil {
		return store.Feed{}, nil, err
	}

	items := s.itemsForBackfill(userID, created.ID, parsed.Items, req.Backfill, now)
	inserted, err := s.store.CreateFeedItems(ctx, items)
	if err != nil {
		return store.Feed{}, nil, err
	}
	if created.AutoSummarize {
		s.enqueueSummaries(inserted)
	}
	refreshed, err := s.store.GetFeed(ctx, userID, created.ID)
	if err != nil {
		return created, inserted, nil
	}
	return refreshed, inserted, nil
}

func (s *Service) ListFeeds(ctx context.Context, userID string) ([]store.Feed, error) {
	return s.store.ListFeeds(ctx, userID)
}

func (s *Service) UpdateFeed(ctx context.Context, userID, feedID string, update store.FeedUpdate) (store.Feed, error) {
	intervalChanged := update.PollingIntervalMinutes != nil
	if update.PollingIntervalMinutes != nil {
		minutes := clampPollingMinutes(*update.PollingIntervalMinutes)
		update.PollingIntervalMinutes = &minutes
	}
	feed, err := s.store.UpdateFeed(ctx, userID, feedID, update)
	if err != nil {
		return store.Feed{}, err
	}
	if intervalChanged {
		next := time.Now().UTC().Add(time.Duration(feed.PollingIntervalMinutes) * time.Minute)
		if err := s.store.UpdateFeedNextCheck(ctx, userID, feedID, next); err != nil {
			return store.Feed{}, err
		}
		feed.NextCheckAt = next
	}
	return feed, nil
}

func (s *Service) DeleteFeed(ctx context.Context, userID, feedID string) error {
	return s.store.DeleteFeed(ctx, userID, feedID)
}

func (s *Service) ListItems(ctx context.Context, userID string, filter store.FeedItemFilter) ([]store.FeedItem, error) {
	return s.store.ListFeedItems(ctx, userID, filter)
}

func (s *Service) GetItem(ctx context.Context, userID, itemID string) (store.FeedItem, error) {
	return s.store.GetFeedItem(ctx, userID, itemID)
}

func (s *Service) PatchItem(ctx context.Context, userID, itemID string, patch ItemPatch) (store.FeedItem, error) {
	return s.store.UpdateFeedItemFlags(ctx, userID, itemID, patch.Read, patch.Starred)
}

func (s *Service) MarkFeedRead(ctx context.Context, userID, feedID string) (store.Feed, int, error) {
	updatedCount, err := s.store.MarkFeedItemsRead(ctx, userID, feedID)
	if err != nil {
		return store.Feed{}, 0, err
	}
	feed, err := s.store.GetFeed(ctx, userID, feedID)
	if err != nil {
		return store.Feed{}, 0, err
	}
	return feed, updatedCount, nil
}

// SummarizeItem generates an LLM summary for a feed item and persists it.
// Concurrency is bounded by the summary semaphore; progress is written to the
// item's summary status so the UI can show working/error states. The optional
// targetCharacters applies to the short "summary" mode only.
func (s *Service) SummarizeItem(ctx context.Context, userID, itemID, mode string, targetCharacters ...int) (store.FeedItem, error) {
	mode = normalizeSummaryMode(mode)
	item, err := s.store.GetFeedItem(ctx, userID, itemID)
	if err != nil {
		return store.FeedItem{}, err
	}
	if item.MediaType != "" {
		return store.FeedItem{}, ErrVideoSummaryUnsupported
	}
	if err := s.acquireSummarySlot(ctx); err != nil {
		return store.FeedItem{}, err
	}
	defer s.releaseSummarySlot()

	_ = s.store.SetFeedItemSummaryStatus(context.Background(), userID, itemID, "working", "")
	summary, err := s.generateSummary(ctx, userID, item, mode, firstTargetCharacters(targetCharacters))
	if err != nil {
		_ = s.store.SetFeedItemSummaryStatus(context.Background(), userID, itemID, "error", err.Error())
		return store.FeedItem{}, err
	}
	return s.store.SetFeedItemSummary(ctx, userID, itemID, summary)
}

func (s *Service) schedulerLoop(ctx context.Context) {
	ticker := time.NewTicker(schedulerInterval)
	defer ticker.Stop()
	s.runDuePolls(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runDuePolls(ctx)
		}
	}
}

func (s *Service) runDuePolls(ctx context.Context) {
	feeds, err := s.store.ListDueFeeds(ctx, time.Now().UTC())
	if err != nil {
		s.logger.Warn("failed to list due feeds", "error", err)
		return
	}
	for _, feed := range feeds {
		if ctx.Err() != nil {
			return
		}
		s.pollFeed(ctx, feed)
	}
}

// pollFeed fetches one feed, stores any items newer than the last poll, and
// records check state either way so a broken feed surfaces its error in the UI
// and is still retried on the next interval.
func (s *Service) pollFeed(ctx context.Context, feed store.Feed) {
	now := time.Now().UTC()
	nextCheck := now.Add(time.Duration(clampPollingMinutes(feed.PollingIntervalMinutes)) * time.Minute)
	// Uses context.Background so the failure is recorded even when the poll
	// failed because ctx was cancelled mid-flight.
	fail := func(stage string, err error) {
		s.logger.Warn(stage, "feed_id", feed.ID, "url", feed.URL, "error", err)
		_ = s.store.UpdateFeedCheckState(context.Background(), feed.ID, now, nextCheck, err.Error())
	}

	parsed, err := s.fetchFeed(ctx, feed.URL)
	if err != nil {
		fail("feed poll failed", err)
		return
	}
	items, err := s.itemsForPoll(ctx, feed, parsed.Items, now)
	if err != nil {
		fail("failed to prepare feed items", err)
		return
	}
	inserted, err := s.store.CreateFeedItems(ctx, items)
	if err != nil {
		fail("failed to store feed items", err)
		return
	}
	if err := s.store.UpdateFeedCheckState(ctx, feed.ID, now, nextCheck, ""); err != nil {
		s.logger.Warn("failed to update feed check state", "feed_id", feed.ID, "error", err)
	}
	if feed.AutoSummarize {
		s.enqueueSummaries(inserted)
	}
}

func (s *Service) fetchFeed(ctx context.Context, rawURL string) (ParsedFeed, error) {
	feedURL, err := validateFeedURL(rawURL)
	if err != nil {
		return ParsedFeed{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return ParsedFeed{}, err
	}
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml;q=0.9, */*;q=0.5")
	req.Header.Set("User-Agent", "RelayFeeds/1.0")
	res, err := s.client.Do(req)
	if err != nil {
		return ParsedFeed{}, fmt.Errorf("fetch feed: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode >= 400 {
		io.Copy(io.Discard, io.LimitReader(res.Body, 512))
		return ParsedFeed{}, fmt.Errorf("feed returned status %d", res.StatusCode)
	}
	raw, err := limitedReadAll(res.Body)
	if err != nil {
		return ParsedFeed{}, fmt.Errorf("read feed: %w", err)
	}
	parsed, err := parseFeed(feedURL, raw)
	if err != nil {
		return ParsedFeed{}, err
	}
	return parsed, nil
}

// itemsForBackfill selects which already-published items to import when a
// feed is first created: "all", everything "since" a date, or the latest N
// (the default, capped at 20 when no limit is given).
func (s *Service) itemsForBackfill(userID, feedID string, parsed []ParsedItem, backfill BackfillOptions, now time.Time) []store.FeedItem {
	mode := strings.TrimSpace(strings.ToLower(backfill.Mode))
	if mode == "" {
		mode = "latest"
	}
	limit := backfill.Limit
	if limit <= 0 {
		limit = 20
	}
	selected := make([]ParsedItem, 0, len(parsed))
	for _, item := range parsed {
		switch mode {
		case "all":
			selected = append(selected, item)
		case "since":
			if backfill.Since != nil && item.PublishedAt != nil && !item.PublishedAt.Before(*backfill.Since) {
				selected = append(selected, item)
			}
		default:
			if len(selected) < limit {
				selected = append(selected, item)
			}
		}
	}
	return s.storeItems(userID, feedID, selected, now)
}

// itemsForPoll picks the new items from a polled feed. It cuts off at the
// newest item already stored (the cursor); when the feed has no stored items
// yet it falls back to comparing publish times against the last check.
func (s *Service) itemsForPoll(ctx context.Context, feed store.Feed, parsed []ParsedItem, now time.Time) ([]store.FeedItem, error) {
	cursor, err := s.store.LatestFeedItemCursor(ctx, feed.UserID, feed.ID)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return s.storeItems(feed.UserID, feed.ID, parsedItemsAfterLastCheck(parsed, feed.LastCheckedAt), now), nil
		}
		return nil, err
	}
	return s.storeItems(feed.UserID, feed.ID, parsedItemsAfterCursor(parsed, cursor), now), nil
}

// parsedItemsAfterCursor returns the items that precede the cursor item in the
// feed (feeds list newest first). Matching by external ID is authoritative;
// publish-time comparison covers feeds whose IDs change between fetches.
func parsedItemsAfterCursor(parsed []ParsedItem, cursor store.FeedItemCursor) []ParsedItem {
	selected := make([]ParsedItem, 0, len(parsed))
	for _, item := range parsed {
		if cursor.ExternalID != "" && parsedItemExternalID(item) == cursor.ExternalID {
			break
		}
		if cursor.PublishedAt != nil {
			if item.PublishedAt != nil && item.PublishedAt.After(*cursor.PublishedAt) {
				selected = append(selected, item)
			}
			continue
		}
		selected = append(selected, item)
	}
	return selected
}

// parsedItemsAfterLastCheck keeps only items published after the feed's last
// successful check — the dedupe fallback when no item cursor exists yet.
func parsedItemsAfterLastCheck(parsed []ParsedItem, lastCheckedAt *time.Time) []ParsedItem {
	if lastCheckedAt == nil || lastCheckedAt.IsZero() {
		return parsed
	}
	selected := make([]ParsedItem, 0, len(parsed))
	for _, item := range parsed {
		if item.PublishedAt != nil && item.PublishedAt.After(*lastCheckedAt) {
			selected = append(selected, item)
		}
	}
	return selected
}

func (s *Service) storeItems(userID, feedID string, parsed []ParsedItem, now time.Time) []store.FeedItem {
	items := make([]store.FeedItem, 0, len(parsed))
	for _, item := range parsed {
		items = append(items, store.FeedItem{
			ID:          uuid.NewString(),
			UserID:      userID,
			FeedID:      feedID,
			ExternalID:  parsedItemExternalID(item),
			Title:       item.Title,
			URL:         item.URL,
			Author:      item.Author,
			PublishedAt: item.PublishedAt,
			Preview:     item.Preview,
			Content:     item.Content,
			MediaType:   item.MediaType,
			MediaURL:    item.MediaURL,
			Read:        false,
			Starred:     false,
			CreatedAt:   now,
			UpdatedAt:   now,
		})
	}
	return items
}

// parsedItemExternalID prefers the feed's own GUID and falls back to a hash of
// URL/title/date so items from GUID-less feeds still dedupe stably.
func parsedItemExternalID(item ParsedItem) string {
	externalID := strings.TrimSpace(item.ExternalID)
	if externalID == "" {
		externalID = stableExternalID(item.URL, item.Title, item.PublishedAt)
	}
	return externalID
}

// enqueueSummaries kicks off background auto-summaries for newly inserted
// items. Video items are skipped (no article text to summarize), and each
// summary runs in its own goroutine gated by the summary semaphore.
func (s *Service) enqueueSummaries(items []store.FeedItem) {
	for _, item := range items {
		if item.MediaType != "" {
			continue
		}
		item := item
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			if _, err := s.SummarizeItem(ctx, item.UserID, item.ID, "summary"); err != nil {
				s.logger.Warn("auto-summary failed", "item_id", item.ID, "error", err)
			}
		}()
	}
}

// acquireSummarySlot blocks until one of the limited concurrent-summary slots
// is free, bounding how many LLM summary calls run at once.
func (s *Service) acquireSummarySlot(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.summarySem <- struct{}{}:
		return nil
	}
}

func (s *Service) releaseSummarySlot() {
	<-s.summarySem
}

func (s *Service) generateSummary(ctx context.Context, userID string, item store.FeedItem, mode string, targetCharacters int) (string, error) {
	settings := s.settings(ctx, userID)
	if strings.TrimSpace(settings.LLMURL) == "" || strings.TrimSpace(settings.LLMModel) == "" {
		return "", fmt.Errorf("LLM settings are required for summaries")
	}
	provider := llm.NewHTTPProviderWithReasoningEffort(settings.LLMURL, settings.LLMModel, settings.LLMAPIKey, 3*time.Minute, "none")
	content := strings.TrimSpace(item.Content)
	if content == "" {
		content = item.Preview
	}
	messages := []llm.ChatMessage{
		{
			Role:    "system",
			Content: summarySystemPrompt(mode, targetCharacters),
		},
		{
			Role: "user",
			Content: fmt.Sprintf(
				feedSummaryUserPromptTemplate,
				item.FeedTitle,
				item.Title,
				item.URL,
				formatOptionalTime(item.PublishedAt),
				content,
			),
		},
	}
	generated, err := llm.CollectText(provider.GenerateStream(ctx, messages, tools.NoopRuntime{}))
	if err != nil {
		return "", err
	}
	summary := strings.TrimSpace(generated)
	if summary == "" {
		return "", fmt.Errorf("summary was empty")
	}
	return summary, nil
}

// generateFeedName asks the LLM for a display name when a feed provides no
// usable title; any failure quietly falls back to the feed's hostname.
func (s *Service) generateFeedName(ctx context.Context, userID string, parsed ParsedFeed) string {
	settings := s.settings(ctx, userID)
	if strings.TrimSpace(settings.LLMURL) == "" || strings.TrimSpace(settings.LLMModel) == "" {
		return hostname(parsed.URL)
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	provider := llm.NewHTTPProviderWithReasoningEffort(settings.LLMURL, settings.LLMModel, settings.LLMAPIKey, 45*time.Second, "none")
	messages := []llm.ChatMessage{
		{Role: "system", Content: feedNameSystemPrompt},
		{Role: "user", Content: fmt.Sprintf(feedNameUserPromptTemplate, parsed.URL, parsed.SiteURL, parsed.Description)},
	}
	generated, err := llm.CollectText(provider.GenerateStream(ctx, messages, tools.NoopRuntime{}))
	if err != nil {
		return hostname(parsed.URL)
	}
	return clampText(strings.Trim(generated, "\"' \n\t"), 80)
}

func summarySystemPrompt(mode string, targetCharacters int) string {
	switch mode {
	case "expanded":
		return feedSummaryExpandedPrompt
	default:
		target := normalizeSummaryTargetCharacters(targetCharacters)
		return fmt.Sprintf(feedSummaryPreviewPromptTemplate, target)
	}
}

func firstTargetCharacters(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

// normalizeSummaryTargetCharacters applies the default for unset values and
// clamps explicit requests into the supported range.
func normalizeSummaryTargetCharacters(value int) int {
	if value <= 0 {
		return defaultSummaryTargetChars
	}
	return min(max(value, minSummaryTargetChars), maxSummaryTargetChars)
}

// normalizeSummaryMode maps any client-supplied mode string to one of the two
// supported modes; everything that isn't an "expanded" variant is a summary.
func normalizeSummaryMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "expanded", "expand":
		return "expanded"
	default:
		return "summary"
	}
}

// clampPollingMinutes applies the default for unset values and enforces the
// minimum interval so a misconfigured feed can't hammer its origin.
func clampPollingMinutes(minutes int) int {
	if minutes <= 0 {
		return defaultPollingMinutes
	}
	return max(minutes, minPollingMinutes)
}

func formatOptionalTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "unknown"
	}
	return t.UTC().Format(time.RFC3339)
}

func IsNotFound(err error) bool {
	return errors.Is(err, store.ErrNotFound)
}
