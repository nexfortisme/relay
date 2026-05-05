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

var ErrVideoSummaryUnsupported = errors.New("video feed items cannot be summarized")

type LLMSettings struct {
	LLMURL    string
	LLMModel  string
	LLMAPIKey string
}

type SettingsLoader func(ctx context.Context, userID string) LLMSettings

type Service struct {
	store    *store.Store
	settings SettingsLoader
	logger   *slog.Logger
	client   *http.Client

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

func (s *Service) Start(ctx context.Context) {
	go s.schedulerLoop(ctx)
}

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

func (s *Service) pollFeed(ctx context.Context, feed store.Feed) {
	now := time.Now().UTC()
	nextCheck := now.Add(time.Duration(clampPollingMinutes(feed.PollingIntervalMinutes)) * time.Minute)
	parsed, err := s.fetchFeed(ctx, feed.URL)
	if err != nil {
		s.logger.Warn("feed poll failed", "feed_id", feed.ID, "url", feed.URL, "error", err)
		_ = s.store.UpdateFeedCheckState(context.Background(), feed.ID, now, nextCheck, err.Error())
		return
	}
	items := s.storeItems(feed.UserID, feed.ID, parsed.Items, now)
	inserted, err := s.store.CreateFeedItems(ctx, items)
	if err != nil {
		s.logger.Warn("failed to store feed items", "feed_id", feed.ID, "error", err)
		_ = s.store.UpdateFeedCheckState(context.Background(), feed.ID, now, nextCheck, err.Error())
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

func (s *Service) storeItems(userID, feedID string, parsed []ParsedItem, now time.Time) []store.FeedItem {
	items := make([]store.FeedItem, 0, len(parsed))
	for _, item := range parsed {
		externalID := strings.TrimSpace(item.ExternalID)
		if externalID == "" {
			externalID = stableExternalID(item.URL, item.Title, item.PublishedAt)
		}
		items = append(items, store.FeedItem{
			ID:          uuid.NewString(),
			UserID:      userID,
			FeedID:      feedID,
			ExternalID:  externalID,
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

func (s *Service) acquireSummarySlot(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case s.summarySem <- struct{}{}:
		return nil
	}
}

func (s *Service) releaseSummarySlot() {
	select {
	case <-s.summarySem:
	default:
	}
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
				"Feed: %s\nTitle: %s\nURL: %s\nPublished: %s\n\n%s",
				item.FeedTitle,
				item.Title,
				item.URL,
				formatOptionalTime(item.PublishedAt),
				content,
			),
		},
	}
	stream := provider.GenerateStream(ctx, messages, tools.NoopRuntime{})
	var builder strings.Builder
	for event := range stream {
		if event.Err != nil {
			return "", event.Err
		}
		if event.Token != "" {
			builder.WriteString(event.Token)
		}
	}
	summary := strings.TrimSpace(builder.String())
	if summary == "" {
		return "", fmt.Errorf("summary was empty")
	}
	return summary, nil
}

func (s *Service) generateFeedName(ctx context.Context, userID string, parsed ParsedFeed) string {
	settings := s.settings(ctx, userID)
	if strings.TrimSpace(settings.LLMURL) == "" || strings.TrimSpace(settings.LLMModel) == "" {
		return hostname(parsed.URL)
	}
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	provider := llm.NewHTTPProviderWithReasoningEffort(settings.LLMURL, settings.LLMModel, settings.LLMAPIKey, 45*time.Second, "none")
	messages := []llm.ChatMessage{
		{Role: "system", Content: "Name this RSS/Atom feed in 2 to 6 words. Return only the name."},
		{Role: "user", Content: fmt.Sprintf("URL: %s\nSite: %s\nDescription: %s", parsed.URL, parsed.SiteURL, parsed.Description)},
	}
	stream := provider.GenerateStream(ctx, messages, tools.NoopRuntime{})
	var builder strings.Builder
	for event := range stream {
		if event.Err != nil {
			return hostname(parsed.URL)
		}
		if event.Token != "" {
			builder.WriteString(event.Token)
		}
	}
	return clampText(strings.Trim(builder.String(), "\"' \n\t"), 80)
}

func summarySystemPrompt(mode string, targetCharacters int) string {
	switch mode {
	case "expanded":
		return "Summarize this feed item for a reader. Use a short heading and 4 to 6 concise bullets. Focus on concrete facts and useful context."
	default:
		target := normalizeSummaryTargetCharacters(targetCharacters)
		return fmt.Sprintf("Write a concise plain-text description of this feed item for an inbox preview. Aim for about %d characters so it fills a single preview line. Return only the description, with no markdown. Focus on concrete facts and avoid speculation.", target)
	}
}

func firstTargetCharacters(values []int) int {
	if len(values) == 0 {
		return 0
	}
	return values[0]
}

func normalizeSummaryTargetCharacters(value int) int {
	if value <= 0 {
		return defaultSummaryTargetChars
	}
	if value < minSummaryTargetChars {
		return minSummaryTargetChars
	}
	if value > maxSummaryTargetChars {
		return maxSummaryTargetChars
	}
	return value
}

func normalizeSummaryMode(mode string) string {
	switch strings.TrimSpace(strings.ToLower(mode)) {
	case "expanded", "expand":
		return "expanded"
	case "summary", "summarize", "resummary", "resummarize", "":
		return "summary"
	default:
		return "summary"
	}
}

func clampPollingMinutes(minutes int) int {
	if minutes <= 0 {
		return defaultPollingMinutes
	}
	if minutes < minPollingMinutes {
		return minPollingMinutes
	}
	return minutes
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
