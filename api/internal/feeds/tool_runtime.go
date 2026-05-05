package feeds

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/nexfortisme/relay/internal/auth"
	"github.com/nexfortisme/relay/internal/store"
	"github.com/nexfortisme/relay/internal/tools"
)

const (
	defaultFeedToolLimit           = 20
	maxFeedToolLimit               = 50
	defaultFeedToolContentMaxChars = 1200
	maxFeedToolContentMaxChars     = 4000
)

type ToolRuntime struct {
	service *Service
}

type feedItemsInput struct {
	Feed            string `json:"feed,omitempty"`
	View            string `json:"view,omitempty"`
	Limit           int    `json:"limit,omitempty"`
	ContentMaxChars *int   `json:"contentMaxChars,omitempty"`
}

type feedToolFeed struct {
	ID            string     `json:"id"`
	Title         string     `json:"title"`
	URL           string     `json:"url"`
	SiteURL       string     `json:"siteUrl,omitempty"`
	Description   string     `json:"description,omitempty"`
	UnreadCount   int        `json:"unreadCount"`
	Status        string     `json:"status"`
	LastCheckedAt *time.Time `json:"lastCheckedAt,omitempty"`
	NextCheckAt   time.Time  `json:"nextCheckAt"`
	LastError     string     `json:"lastError,omitempty"`
}

type feedToolItem struct {
	ID               string     `json:"id"`
	FeedID           string     `json:"feedId"`
	FeedTitle        string     `json:"feedTitle"`
	Title            string     `json:"title"`
	URL              string     `json:"url"`
	Author           string     `json:"author,omitempty"`
	PublishedAt      *time.Time `json:"publishedAt,omitempty"`
	Summary          string     `json:"summary,omitempty"`
	Preview          string     `json:"preview,omitempty"`
	ContentExcerpt   string     `json:"contentExcerpt,omitempty"`
	ContentTruncated bool       `json:"contentTruncated"`
	MediaType        string     `json:"mediaType,omitempty"`
	MediaURL         string     `json:"mediaUrl,omitempty"`
	Read             bool       `json:"read"`
	Starred          bool       `json:"starred"`
	CreatedAt        time.Time  `json:"createdAt"`
}

type listFeedsOutput struct {
	Feeds []feedToolFeed `json:"feeds"`
	Count int            `json:"count"`
}

type feedItemsOutput struct {
	Items              []feedToolItem `json:"items"`
	Count              int            `json:"count"`
	View               string         `json:"view"`
	Feed               *feedToolFeed  `json:"feed,omitempty"`
	Candidates         []feedToolFeed `json:"candidates,omitempty"`
	Error              string         `json:"error,omitempty"`
	Limit              int            `json:"limit"`
	Truncated          bool           `json:"truncated"`
	ContentMaxChars    int            `json:"contentMaxChars"`
	ContentOmitted     bool           `json:"contentOmitted"`
	ContentWasClipped  bool           `json:"contentWasClipped"`
	AvailableItemCount int            `json:"availableItemCount"`
}

type feedMatch struct {
	feed       *store.Feed
	candidates []store.Feed
	notFound   bool
}

func NewToolRuntime(service *Service) *ToolRuntime {
	return &ToolRuntime{service: service}
}

func (r *ToolRuntime) Definitions(context.Context) ([]tools.Definition, error) {
	return []tools.Definition{
		{
			Name:        "list_feeds",
			Description: "List the authenticated user's RSS/Atom feeds with titles, URLs, unread counts, last-check status, and errors. Use this before get_feed_items when the requested feed name is ambiguous.",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{},
				"required":   []any{},
			},
		},
		{
			Name:        "get_feed_items",
			Description: "Read stored feed inbox items for the authenticated user. Use view=unread for new posts and view=all for latest posts. Optionally filter by feed ID, title, site URL, or feed URL.",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"feed": map[string]any{
						"type":        "string",
						"description": "Optional feed ID, title, site URL, or feed URL. Leave empty to read items across all feeds.",
					},
					"view": map[string]any{
						"type":        "string",
						"enum":        []any{"unread", "all", "starred"},
						"description": "Which stored items to return. unread is for new posts; all is for latest posts.",
					},
					"limit": map[string]any{
						"type":        "integer",
						"minimum":     1,
						"maximum":     maxFeedToolLimit,
						"description": "Maximum number of items to return. Defaults to 20 and caps at 50.",
					},
					"contentMaxChars": map[string]any{
						"type":        "integer",
						"minimum":     0,
						"maximum":     maxFeedToolContentMaxChars,
						"description": "Maximum characters of stored content excerpt per item. Defaults to 1200; use 0 to omit content excerpts.",
					},
				},
				"required": []any{},
			},
		},
	}, nil
}

func (r *ToolRuntime) Execute(ctx context.Context, call tools.Call) (tools.Result, error) {
	if r.service == nil {
		return tools.Result{}, fmt.Errorf("feed tool runtime not configured")
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return tools.Result{}, fmt.Errorf("feed tools require an authenticated user")
	}

	switch call.Name {
	case "list_feeds":
		output, err := r.listFeeds(ctx, userID)
		if err != nil {
			return tools.Result{}, err
		}
		return tools.Result{Name: call.Name, Output: output}, nil
	case "get_feed_items":
		output, err := r.getFeedItems(ctx, userID, call.Arguments)
		if err != nil {
			return tools.Result{}, err
		}
		return tools.Result{Name: call.Name, Output: output, IsError: output.Error != ""}, nil
	default:
		return tools.Result{}, fmt.Errorf("unknown feed tool: %s", call.Name)
	}
}

func (r *ToolRuntime) listFeeds(ctx context.Context, userID string) (listFeedsOutput, error) {
	feeds, err := r.service.ListFeeds(ctx, userID)
	if err != nil {
		return listFeedsOutput{}, err
	}
	out := make([]feedToolFeed, 0, len(feeds))
	for _, feed := range feeds {
		out = append(out, feedForTool(feed))
	}
	return listFeedsOutput{Feeds: out, Count: len(out)}, nil
}

func (r *ToolRuntime) getFeedItems(ctx context.Context, userID string, args map[string]any) (feedItemsOutput, error) {
	input, err := decodeFeedItemsInput(args)
	if err != nil {
		return feedItemsOutput{}, err
	}
	view := normalizeFeedToolView(input.View)
	limit := normalizeFeedToolLimit(input.Limit)
	contentMaxChars := normalizeFeedToolContentMaxChars(input.ContentMaxChars)

	feeds, err := r.service.ListFeeds(ctx, userID)
	if err != nil {
		return feedItemsOutput{}, err
	}

	match := matchFeedForTool(feeds, input.Feed)
	output := feedItemsOutput{
		Items:           []feedToolItem{},
		View:            view,
		Limit:           limit,
		ContentMaxChars: contentMaxChars,
		ContentOmitted:  contentMaxChars == 0,
	}
	if match.notFound {
		output.Error = fmt.Sprintf("no feed matched %q", strings.TrimSpace(input.Feed))
		output.Candidates = feedCandidates(feeds)
		return output, nil
	}
	if len(match.candidates) > 0 {
		output.Error = fmt.Sprintf("feed query %q matched multiple feeds", strings.TrimSpace(input.Feed))
		output.Candidates = feedCandidates(match.candidates)
		return output, nil
	}

	filter := store.FeedItemFilter{View: view}
	if match.feed != nil {
		filter.FeedID = match.feed.ID
		toolFeed := feedForTool(*match.feed)
		output.Feed = &toolFeed
	}

	items, err := r.service.ListItems(ctx, userID, filter)
	if err != nil {
		return feedItemsOutput{}, err
	}
	output.AvailableItemCount = len(items)
	if len(items) > limit {
		output.Truncated = true
		items = items[:limit]
	}
	output.Items = make([]feedToolItem, 0, len(items))
	for _, item := range items {
		toolItem, clipped := feedItemForTool(item, contentMaxChars)
		output.ContentWasClipped = output.ContentWasClipped || clipped
		output.Items = append(output.Items, toolItem)
	}
	output.Count = len(output.Items)
	return output, nil
}

func decodeFeedItemsInput(args map[string]any) (feedItemsInput, error) {
	if args == nil {
		return feedItemsInput{}, nil
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return feedItemsInput{}, fmt.Errorf("marshal feed tool arguments: %w", err)
	}
	var input feedItemsInput
	if err := json.Unmarshal(encoded, &input); err != nil {
		return feedItemsInput{}, fmt.Errorf("parse feed tool arguments: %w", err)
	}
	return input, nil
}

func feedForTool(feed store.Feed) feedToolFeed {
	status := "ok"
	if feed.LastCheckedAt == nil {
		status = "unchecked"
	}
	if strings.TrimSpace(feed.LastError) != "" {
		status = "error"
	}
	return feedToolFeed{
		ID:            feed.ID,
		Title:         feed.Title,
		URL:           feed.URL,
		SiteURL:       feed.SiteURL,
		Description:   feed.Description,
		UnreadCount:   feed.UnreadCount,
		Status:        status,
		LastCheckedAt: feed.LastCheckedAt,
		NextCheckAt:   feed.NextCheckAt,
		LastError:     feed.LastError,
	}
}

func feedItemForTool(item store.FeedItem, contentMaxChars int) (feedToolItem, bool) {
	excerpt, clipped := clampToolText(item.Content, contentMaxChars)
	return feedToolItem{
		ID:               item.ID,
		FeedID:           item.FeedID,
		FeedTitle:        item.FeedTitle,
		Title:            item.Title,
		URL:              item.URL,
		Author:           item.Author,
		PublishedAt:      item.PublishedAt,
		Summary:          item.Summary,
		Preview:          item.Preview,
		ContentExcerpt:   excerpt,
		ContentTruncated: clipped,
		MediaType:        item.MediaType,
		MediaURL:         item.MediaURL,
		Read:             item.Read,
		Starred:          item.Starred,
		CreatedAt:        item.CreatedAt,
	}, clipped
}

func feedCandidates(feeds []store.Feed) []feedToolFeed {
	out := make([]feedToolFeed, 0, len(feeds))
	for _, feed := range feeds {
		out = append(out, feedForTool(feed))
	}
	return out
}

func matchFeedForTool(feeds []store.Feed, query string) feedMatch {
	query = normalizeFeedMatchText(query)
	if query == "" {
		return feedMatch{}
	}

	exact := make([]store.Feed, 0)
	for _, feed := range feeds {
		if feedMatchesExactly(feed, query) {
			exact = append(exact, feed)
		}
	}
	if len(exact) == 1 {
		return feedMatch{feed: &exact[0]}
	}
	if len(exact) > 1 {
		return feedMatch{candidates: exact}
	}

	partial := make([]store.Feed, 0)
	for _, feed := range feeds {
		if feedMatchesPartially(feed, query) {
			partial = append(partial, feed)
		}
	}
	if len(partial) == 1 {
		return feedMatch{feed: &partial[0]}
	}
	if len(partial) > 1 {
		return feedMatch{candidates: partial}
	}
	return feedMatch{notFound: true}
}

func feedMatchesExactly(feed store.Feed, query string) bool {
	for _, candidate := range feedMatchCandidates(feed) {
		if normalizeFeedMatchText(candidate) == query {
			return true
		}
	}
	return false
}

func feedMatchesPartially(feed store.Feed, query string) bool {
	for _, candidate := range feedMatchCandidates(feed) {
		normalized := normalizeFeedMatchText(candidate)
		if normalized != "" && strings.Contains(normalized, query) {
			return true
		}
	}
	return false
}

func feedMatchCandidates(feed store.Feed) []string {
	return []string{feed.ID, feed.Title, feed.URL, feed.SiteURL}
}

func normalizeFeedMatchText(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	value = strings.TrimSuffix(value, "/")
	return strings.Join(strings.Fields(value), " ")
}

func normalizeFeedToolView(view string) string {
	switch strings.ToLower(strings.TrimSpace(view)) {
	case "all", "latest":
		return "all"
	case "starred", "saved":
		return "starred"
	default:
		return "unread"
	}
}

func normalizeFeedToolLimit(limit int) int {
	if limit <= 0 {
		return defaultFeedToolLimit
	}
	if limit > maxFeedToolLimit {
		return maxFeedToolLimit
	}
	return limit
}

func normalizeFeedToolContentMaxChars(value *int) int {
	if value == nil {
		return defaultFeedToolContentMaxChars
	}
	if *value <= 0 {
		return 0
	}
	if *value > maxFeedToolContentMaxChars {
		return maxFeedToolContentMaxChars
	}
	return *value
}

func clampToolText(value string, maxChars int) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" || maxChars <= 0 {
		return "", false
	}
	runes := []rune(value)
	if len(runes) <= maxChars {
		return value, false
	}
	return string(runes[:maxChars]), true
}
