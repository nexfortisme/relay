package feeds

import (
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const maxFeedBodyBytes = 5 << 20

type ParsedFeed struct {
	URL         string       `json:"url"`
	Title       string       `json:"title"`
	SiteURL     string       `json:"siteUrl"`
	Description string       `json:"description"`
	Items       []ParsedItem `json:"items"`
}

type ParsedItem struct {
	ExternalID  string
	Title       string
	URL         string
	Author      string
	PublishedAt *time.Time
	Preview     string
	Content     string
	MediaType   string
	MediaURL    string
}

type rssDocument struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title            string         `xml:"title"`
	Link             string         `xml:"link"`
	GUID             string         `xml:"guid"`
	Description      string         `xml:"description"`
	ContentEncoded   string         `xml:"encoded"`
	ContentEncodedNS string         `xml:"http://purl.org/rss/1.0/modules/content/ encoded"`
	Author           string         `xml:"author"`
	Creator          string         `xml:"creator"`
	CreatorNS        string         `xml:"http://purl.org/dc/elements/1.1/ creator"`
	PubDate          string         `xml:"pubDate"`
	Enclosures       []rssEnclosure `xml:"enclosure"`
	MediaContents    []mediaContent `xml:"http://search.yahoo.com/mrss/ content"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type mediaContent struct {
	URL    string `xml:"url,attr"`
	Type   string `xml:"type,attr"`
	Medium string `xml:"medium,attr"`
}

type atomFeed struct {
	Title    string      `xml:"title"`
	Subtitle string      `xml:"subtitle"`
	Links    []atomLink  `xml:"link"`
	Entries  []atomEntry `xml:"entry"`
}

type atomLink struct {
	Rel  string `xml:"rel,attr"`
	Href string `xml:"href,attr"`
	Type string `xml:"type,attr"`
}

type atomEntry struct {
	ID        string     `xml:"id"`
	Title     string     `xml:"title"`
	Summary   string     `xml:"summary"`
	Content   string     `xml:"content"`
	Updated   string     `xml:"updated"`
	Published string     `xml:"published"`
	Links     []atomLink `xml:"link"`
	Author    atomAuthor `xml:"author"`
}

type atomAuthor struct {
	Name string `xml:"name"`
}

var tagRe = regexp.MustCompile(`<[^>]+>`)
var spaceRe = regexp.MustCompile(`\s+`)
var spaceBeforePunctRe = regexp.MustCompile(`\s+([.,;:!?])`)

func parseFeed(feedURL string, raw []byte) (ParsedFeed, error) {
	if len(raw) == 0 {
		return ParsedFeed{}, fmt.Errorf("feed response was empty")
	}
	if parsed, err := parseRSS(feedURL, raw); err == nil && len(parsed.Items) > 0 {
		return parsed, nil
	}
	if parsed, err := parseAtom(feedURL, raw); err == nil && len(parsed.Items) > 0 {
		return parsed, nil
	}
	return ParsedFeed{}, fmt.Errorf("response did not look like an RSS or Atom feed")
}

func parseRSS(feedURL string, raw []byte) (ParsedFeed, error) {
	var doc rssDocument
	if err := decodeXML(raw, &doc); err != nil {
		return ParsedFeed{}, err
	}
	if strings.TrimSpace(doc.Channel.Title) == "" && len(doc.Channel.Items) == 0 {
		return ParsedFeed{}, fmt.Errorf("rss channel missing")
	}
	parsed := ParsedFeed{
		URL:         feedURL,
		Title:       cleanText(doc.Channel.Title),
		SiteURL:     absolutizeURL(feedURL, strings.TrimSpace(doc.Channel.Link)),
		Description: cleanText(doc.Channel.Description),
		Items:       make([]ParsedItem, 0, len(doc.Channel.Items)),
	}
	for _, item := range doc.Channel.Items {
		content := firstNonEmpty(item.ContentEncodedNS, item.ContentEncoded, item.Description)
		published := parseFeedTime(item.PubDate)
		link := absolutizeURL(feedURL, strings.TrimSpace(item.Link))
		title := cleanText(item.Title)
		plainContent := cleanText(content)
		mediaType, mediaURL := detectItemMedia(link, rssMediaCandidates(feedURL, item))
		parsed.Items = append(parsed.Items, ParsedItem{
			ExternalID:  stableExternalID(item.GUID, link, title, published),
			Title:       fallbackTitle(title, plainContent),
			URL:         link,
			Author:      cleanText(firstNonEmpty(item.CreatorNS, item.Creator, item.Author)),
			PublishedAt: published,
			Preview:     clampText(plainContent, 360),
			Content:     plainContent,
			MediaType:   mediaType,
			MediaURL:    mediaURL,
		})
	}
	return parsed, nil
}

func parseAtom(feedURL string, raw []byte) (ParsedFeed, error) {
	var doc atomFeed
	if err := decodeXML(raw, &doc); err != nil {
		return ParsedFeed{}, err
	}
	if strings.TrimSpace(doc.Title) == "" && len(doc.Entries) == 0 {
		return ParsedFeed{}, fmt.Errorf("atom feed missing")
	}
	parsed := ParsedFeed{
		URL:         feedURL,
		Title:       cleanText(doc.Title),
		SiteURL:     absolutizeURL(feedURL, atomLinkHref(doc.Links)),
		Description: cleanText(doc.Subtitle),
		Items:       make([]ParsedItem, 0, len(doc.Entries)),
	}
	for _, entry := range doc.Entries {
		content := firstNonEmpty(entry.Content, entry.Summary)
		published := parseFeedTime(firstNonEmpty(entry.Published, entry.Updated))
		link := absolutizeURL(feedURL, atomLinkHref(entry.Links))
		title := cleanText(entry.Title)
		plainContent := cleanText(content)
		mediaType, mediaURL := detectItemMedia(link, atomMediaCandidates(feedURL, entry))
		parsed.Items = append(parsed.Items, ParsedItem{
			ExternalID:  stableExternalID(entry.ID, link, title, published),
			Title:       fallbackTitle(title, plainContent),
			URL:         link,
			Author:      cleanText(entry.Author.Name),
			PublishedAt: published,
			Preview:     clampText(plainContent, 360),
			Content:     plainContent,
			MediaType:   mediaType,
			MediaURL:    mediaURL,
		})
	}
	return parsed, nil
}

func decodeXML(raw []byte, out any) error {
	decoder := xml.NewDecoder(strings.NewReader(string(raw)))
	decoder.Strict = false
	return decoder.Decode(out)
}

func atomLinkHref(links []atomLink) string {
	for _, link := range links {
		rel := strings.TrimSpace(strings.ToLower(link.Rel))
		if rel == "" || rel == "alternate" {
			return strings.TrimSpace(link.Href)
		}
	}
	if len(links) > 0 {
		return strings.TrimSpace(links[0].Href)
	}
	return ""
}

type mediaCandidate struct {
	URL         string
	ContentType string
	Medium      string
}

func rssMediaCandidates(feedURL string, item rssItem) []mediaCandidate {
	candidates := make([]mediaCandidate, 0, len(item.Enclosures)+len(item.MediaContents))
	for _, enclosure := range item.Enclosures {
		candidates = append(candidates, mediaCandidate{
			URL:         absolutizeURL(feedURL, enclosure.URL),
			ContentType: enclosure.Type,
		})
	}
	for _, media := range item.MediaContents {
		candidates = append(candidates, mediaCandidate{
			URL:         absolutizeURL(feedURL, media.URL),
			ContentType: media.Type,
			Medium:      media.Medium,
		})
	}
	return candidates
}

func atomMediaCandidates(feedURL string, entry atomEntry) []mediaCandidate {
	candidates := make([]mediaCandidate, 0, len(entry.Links))
	for _, link := range entry.Links {
		candidates = append(candidates, mediaCandidate{
			URL:         absolutizeURL(feedURL, link.Href),
			ContentType: link.Type,
		})
	}
	return candidates
}

func detectItemMedia(primaryURL string, candidates []mediaCandidate) (string, string) {
	all := make([]mediaCandidate, 0, len(candidates)+1)
	all = append(all, mediaCandidate{URL: primaryURL})
	all = append(all, candidates...)
	for _, candidate := range all {
		mediaURL := strings.TrimSpace(candidate.URL)
		if mediaURL == "" {
			continue
		}
		if isYouTubeURL(mediaURL) {
			return "youtube", mediaURL
		}
		if isVimeoURL(mediaURL) {
			return "vimeo", mediaURL
		}
		if isVideoCandidate(candidate) {
			return "video", mediaURL
		}
	}
	return "", ""
}

func isVideoCandidate(candidate mediaCandidate) bool {
	contentType := strings.ToLower(strings.TrimSpace(candidate.ContentType))
	medium := strings.ToLower(strings.TrimSpace(candidate.Medium))
	if strings.HasPrefix(contentType, "video/") || medium == "video" {
		return true
	}
	parsed, err := url.Parse(candidate.URL)
	if err != nil {
		return false
	}
	path := strings.ToLower(parsed.Path)
	for _, suffix := range []string{".mp4", ".webm", ".ogv", ".ogg", ".mov", ".m4v"} {
		if strings.HasSuffix(path, suffix) {
			return true
		}
	}
	return false
}

func isYouTubeURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	path := strings.ToLower(parsed.Path)
	return host == "youtu.be" ||
		host == "youtube.com" && (strings.HasPrefix(path, "/watch") || strings.HasPrefix(path, "/shorts/") || strings.HasPrefix(path, "/embed/"))
}

func isVimeoURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.TrimPrefix(strings.ToLower(parsed.Hostname()), "www.")
	return host == "vimeo.com" || host == "player.vimeo.com"
}

func validateFeedURL(rawURL string) (string, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return "", fmt.Errorf("feed URL is required")
	}
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Host == "" {
		return "", fmt.Errorf("feed URL must be absolute")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("feed URL must use http or https")
	}
	return parsed.String(), nil
}

func absolutizeURL(baseURL string, maybeRelative string) string {
	trimmed := strings.TrimSpace(maybeRelative)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err == nil && parsed.IsAbs() {
		return parsed.String()
	}
	base, err := url.Parse(baseURL)
	if err != nil {
		return trimmed
	}
	return base.ResolveReference(parsed).String()
}

func hostname(rawURL string) string {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "Untitled feed"
	}
	host := strings.TrimPrefix(parsed.Hostname(), "www.")
	if host == "" {
		return "Untitled feed"
	}
	return host
}

func cleanText(value string) string {
	withoutTags := tagRe.ReplaceAllString(value, " ")
	unescaped := html.UnescapeString(withoutTags)
	normalized := strings.TrimSpace(spaceRe.ReplaceAllString(unescaped, " "))
	return spaceBeforePunctRe.ReplaceAllString(normalized, "$1")
}

func clampText(value string, limit int) string {
	trimmed := strings.TrimSpace(value)
	if limit <= 0 || len(trimmed) <= limit {
		return trimmed
	}
	return strings.TrimSpace(trimmed[:limit]) + "..."
}

func fallbackTitle(title string, content string) string {
	if strings.TrimSpace(title) != "" {
		return title
	}
	if content != "" {
		return clampText(content, 80)
	}
	return "Untitled item"
}

func stableExternalID(candidates ...any) string {
	for _, candidate := range candidates {
		switch value := candidate.(type) {
		case string:
			if strings.TrimSpace(value) != "" {
				return strings.TrimSpace(value)
			}
		case *time.Time:
			if value != nil && !value.IsZero() {
				return value.UTC().Format(time.RFC3339Nano)
			}
		}
	}
	return fmt.Sprintf("item-%d", time.Now().UnixNano())
}

func parseFeedTime(value string) *time.Time {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC3339,
		time.RFC3339Nano,
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 02 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"Mon, 02 Jan 2006 15:04:05 MST",
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02 15:04:05",
		"2006-01-02",
	}
	for _, layout := range layouts {
		if parsed, err := time.Parse(layout, trimmed); err == nil {
			t := parsed.UTC()
			return &t
		}
	}
	return nil
}

func limitedReadAll(reader io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(reader, maxFeedBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(raw) > maxFeedBodyBytes {
		return nil, fmt.Errorf("feed response exceeds %d bytes", maxFeedBodyBytes)
	}
	return raw, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
