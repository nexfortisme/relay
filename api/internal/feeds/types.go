package feeds

import "time"

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
