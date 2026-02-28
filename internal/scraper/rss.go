// Package scraper provides feed parsing, HTML scraping, and content processing
// for the Folio ingestion pipeline.
package scraper

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// reImgSrc matches src attribute in <img> tags.
var reImgSrc = regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)

// FeedItem represents a single item parsed from an RSS or Atom feed.
type FeedItem struct {
	Title       string
	Link        string
	Description string
	Published   time.Time
	GUID        string
	ImageURL    string
}

// rssRoot is the top-level XML element for RSS 2.0 feeds.
type rssRoot struct {
	XMLName xml.Name   `xml:"rss"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string         `xml:"title"`
	Link        string         `xml:"link"`
	Description string         `xml:"description"`
	PubDate     string         `xml:"pubDate"`
	GUID        string         `xml:"guid"`
	Enclosure   rssEnclosure   `xml:"enclosure"`
	MediaContent []rssMedia    `xml:"content"`
}

type rssEnclosure struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

type rssMedia struct {
	URL  string `xml:"url,attr"`
	Type string `xml:"type,attr"`
}

// atomFeed is the top-level XML element for Atom feeds.
type atomFeed struct {
	XMLName xml.Name   `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title   string     `xml:"title"`
	Links   []atomLink `xml:"link"`
	Summary string     `xml:"summary"`
	Content string     `xml:"content"`
	Updated string     `xml:"updated"`
	ID      string     `xml:"id"`
}

type atomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

const (
	feedUserAgent = "Folio/1.0 (+https://github.com/Saul-Punybz/folio)"
	feedTimeout   = 30 * time.Second
)

// ParseFeed fetches and parses an RSS 2.0 or Atom feed from the given URL,
// returning the list of items found.
func ParseFeed(ctx context.Context, feedURL string) ([]FeedItem, error) {
	ctx, cancel := context.WithTimeout(ctx, feedTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, feedURL, nil)
	if err != nil {
		return nil, fmt.Errorf("rss: create request: %w", err)
	}
	req.Header.Set("User-Agent", feedUserAgent)
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rss: fetch %s: %w", feedURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("rss: fetch %s: status %d", feedURL, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10 MB limit
	if err != nil {
		return nil, fmt.Errorf("rss: read body: %w", err)
	}

	// Try RSS 2.0 first.
	items, err := parseRSS(body)
	if err == nil && len(items) > 0 {
		return items, nil
	}

	// Fall back to Atom.
	items, err = parseAtom(body)
	if err == nil && len(items) > 0 {
		return items, nil
	}

	return nil, fmt.Errorf("rss: unrecognized feed format at %s", feedURL)
}

// parseRSS attempts to decode RSS 2.0 XML.
func parseRSS(data []byte) ([]FeedItem, error) {
	var root rssRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	if len(root.Channel.Items) == 0 {
		return nil, fmt.Errorf("no RSS items found")
	}

	items := make([]FeedItem, 0, len(root.Channel.Items))
	for _, ri := range root.Channel.Items {
		item := FeedItem{
			Title:       strings.TrimSpace(ri.Title),
			Link:        strings.TrimSpace(ri.Link),
			Description: strings.TrimSpace(ri.Description),
			GUID:        strings.TrimSpace(ri.GUID),
			Published:   parseDate(ri.PubDate),
			ImageURL:    extractRSSImageURL(ri),
		}
		if item.GUID == "" {
			item.GUID = item.Link
		}
		items = append(items, item)
	}

	return items, nil
}

// parseAtom attempts to decode Atom XML.
func parseAtom(data []byte) ([]FeedItem, error) {
	var feed atomFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	if len(feed.Entries) == 0 {
		return nil, fmt.Errorf("no Atom entries found")
	}

	items := make([]FeedItem, 0, len(feed.Entries))
	for _, entry := range feed.Entries {
		link := atomEntryLink(entry.Links)
		description := entry.Summary
		if description == "" {
			description = entry.Content
		}

		item := FeedItem{
			Title:       strings.TrimSpace(entry.Title),
			Link:        strings.TrimSpace(link),
			Description: strings.TrimSpace(description),
			GUID:        strings.TrimSpace(entry.ID),
			Published:   parseDate(entry.Updated),
		}
		if item.GUID == "" {
			item.GUID = item.Link
		}
		items = append(items, item)
	}

	return items, nil
}

// atomEntryLink extracts the best link from an Atom entry. It prefers rel="alternate"
// or the first href found.
func atomEntryLink(links []atomLink) string {
	for _, l := range links {
		if l.Rel == "alternate" || l.Rel == "" {
			return l.Href
		}
	}
	if len(links) > 0 {
		return links[0].Href
	}
	return ""
}

// extractRSSImageURL tries to find an image URL from an RSS item, checking:
// 1. <enclosure> with an image type
// 2. <media:content> with an image type
// 3. <img> tag in the description HTML
func extractRSSImageURL(ri rssItem) string {
	// Check enclosure (e.g., <enclosure url="..." type="image/jpeg"/>).
	if ri.Enclosure.URL != "" && strings.HasPrefix(ri.Enclosure.Type, "image/") {
		return strings.TrimSpace(ri.Enclosure.URL)
	}

	// Check media:content elements.
	for _, mc := range ri.MediaContent {
		if mc.URL != "" && (mc.Type == "" || strings.HasPrefix(mc.Type, "image/")) {
			return strings.TrimSpace(mc.URL)
		}
	}

	// Fall back to extracting <img src="..."> from description HTML.
	if ri.Description != "" {
		matches := reImgSrc.FindStringSubmatch(ri.Description)
		if len(matches) >= 2 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// parseDate tries several common date formats used in RSS and Atom feeds.
func parseDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	formats := []string{
		time.RFC1123Z,                  // Mon, 02 Jan 2006 15:04:05 -0700
		time.RFC1123,                   // Mon, 02 Jan 2006 15:04:05 MST
		time.RFC3339,                   // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano,               // 2006-01-02T15:04:05.999999999Z07:00
		"2006-01-02T15:04:05Z",         // ISO without offset
		"2006-01-02T15:04:05",          // ISO without timezone
		"2006-01-02",                   // Date only
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 MST",
	}

	for _, f := range formats {
		t, err := time.Parse(f, s)
		if err == nil {
			return t
		}
	}

	return time.Time{}
}
