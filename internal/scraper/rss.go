// Package scraper provides feed parsing, HTML scraping, and content processing
// for the Folio ingestion pipeline.
package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"html"
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
	Categories  []string // <category> de RSS / term de Atom, tal cual vienen
	SourceName  string   // <source> de RSS (Google News: el medio real)
	SourceURL   string   // atributo url de <source>
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
	Title        string       `xml:"title"`
	Link         string       `xml:"link"`
	Description  string       `xml:"description"`
	PubDate      string       `xml:"pubDate"`
	DCDate       string       `xml:"date"`
	GUID         string       `xml:"guid"`
	Categories   []string     `xml:"category"`
	Enclosure    rssEnclosure `xml:"enclosure"`
	MediaContent []rssMedia   `xml:"content"`
	Source       rssSource    `xml:"source"`
}

type rssSource struct {
	URL  string `xml:"url,attr"`
	Name string `xml:",chardata"`
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
	XMLName xml.Name    `xml:"feed"`
	Entries []atomEntry `xml:"entry"`
}

type atomEntry struct {
	Title      string         `xml:"title"`
	Links      []atomLink     `xml:"link"`
	Summary    string         `xml:"summary"`
	Content    string         `xml:"content"`
	Updated    string         `xml:"updated"`
	Published  string         `xml:"published"`
	ID         string         `xml:"id"`
	Categories []atomCategory `xml:"category"`
}

type atomCategory struct {
	Term  string `xml:"term,attr"`
	Label string `xml:"label,attr"`
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
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/feed+json, application/json, application/xml, text/xml")

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

	items, err := parseAnyFeed(body)
	if err == nil && len(items) > 0 {
		return items, nil
	}
	if err == errEmptyFeed {
		return nil, fmt.Errorf("rss: %s es un feed válido pero no trae ítems", feedURL)
	}
	return nil, fmt.Errorf("rss: unrecognized feed format at %s", feedURL)
}

var errEmptyFeed = fmt.Errorf("feed vacío")

// parseAnyFeed detecta el formato del cuerpo y lo decodifica: RSS 2.0, Atom,
// RSS 1.0 (RDF), JSON Feed, la API REST de WordPress (arreglo de posts) y
// sitemaps de Google News (urlset con news:news).
func parseAnyFeed(body []byte) ([]FeedItem, error) {
	trimmed := bytes.TrimLeft(body, "\xef\xbb\xbf \t\r\n")
	if len(trimmed) > 0 && (trimmed[0] == '{' || trimmed[0] == '[') {
		return parseJSONFeed(trimmed)
	}
	for _, parse := range []func([]byte) ([]FeedItem, error){parseRSS, parseAtom, parseRDF, parseNewsSitemap} {
		if items, err := parse(body); err == nil && len(items) > 0 {
			return items, nil
		}
	}
	// ¿Es un feed bien formado pero sin ítems (p. ej. WordPress sin posts)?
	head := string(trimmed)
	if len(head) > 4096 {
		head = head[:4096]
	}
	if strings.Contains(head, "<rss") || strings.Contains(head, "<feed") || strings.Contains(head, "<rdf:RDF") {
		return nil, errEmptyFeed
	}
	return nil, fmt.Errorf("formato no reconocido")
}

// rdfRoot es la raíz de RSS 1.0 (rdf:RDF), con los ítems como hijos directos.
type rdfRoot struct {
	XMLName xml.Name  `xml:"RDF"`
	Items   []rssItem `xml:"item"`
}

// parseRDF decodifica RSS 1.0.
func parseRDF(data []byte) ([]FeedItem, error) {
	var root rdfRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		return nil, err
	}
	if len(root.Items) == 0 {
		return nil, fmt.Errorf("no RDF items found")
	}
	items := make([]FeedItem, 0, len(root.Items))
	for _, ri := range root.Items {
		it := FeedItem{
			Title:       strings.TrimSpace(ri.Title),
			Link:        strings.TrimSpace(ri.Link),
			Description: strings.TrimSpace(ri.Description),
			GUID:        strings.TrimSpace(ri.GUID),
			Published:   parseDate(firstNonEmpty(ri.PubDate, ri.DCDate)),
		}
		if it.GUID == "" {
			it.GUID = it.Link
		}
		items = append(items, it)
	}
	return items, nil
}

// newsSitemap es un sitemap de Google News: cada <url> trae título y fecha.
type newsSitemap struct {
	XMLName xml.Name         `xml:"urlset"`
	URLs    []newsSitemapURL `xml:"url"`
}

type newsSitemapURL struct {
	Loc   string `xml:"loc"`
	Date  string `xml:"news>publication_date"`
	Title string `xml:"news>title"`
}

// parseNewsSitemap decodifica un sitemap de Google News. Los <url> sin
// news:title (sitemaps normales) se ignoran.
func parseNewsSitemap(data []byte) ([]FeedItem, error) {
	var sm newsSitemap
	if err := xml.Unmarshal(data, &sm); err != nil {
		return nil, err
	}
	var items []FeedItem
	for _, u := range sm.URLs {
		link := strings.TrimSpace(u.Loc)
		title := strings.TrimSpace(u.Title)
		if link == "" || title == "" {
			continue
		}
		items = append(items, FeedItem{Title: title, Link: link, GUID: link, Published: parseDate(u.Date)})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no news sitemap entries found")
	}
	return items, nil
}

// jsonFeed cubre JSON Feed 1.x ({"items":[...]}).
type jsonFeed struct {
	Items []struct {
		ID            string   `json:"id"`
		URL           string   `json:"url"`
		Title         string   `json:"title"`
		ContentHTML   string   `json:"content_html"`
		ContentText   string   `json:"content_text"`
		Summary       string   `json:"summary"`
		Image         string   `json:"image"`
		DatePublished string   `json:"date_published"`
		DateModified  string   `json:"date_modified"`
		Tags          []string `json:"tags"`
	} `json:"items"`
}

// wpPost es un post de la API REST de WordPress (/wp-json/wp/v2/posts), útil
// cuando el sitio bloquea /feed pero deja abierta la API.
type wpPost struct {
	Date    string `json:"date"`
	DateGMT string `json:"date_gmt"`
	Link    string `json:"link"`
	Title   struct {
		Rendered string `json:"rendered"`
	} `json:"title"`
	Excerpt struct {
		Rendered string `json:"rendered"`
	} `json:"excerpt"`
	Content struct {
		Rendered string `json:"rendered"`
	} `json:"content"`
	Image string `json:"jetpack_featured_media_url"`
}

// parseJSONFeed decodifica JSON Feed o un arreglo de posts de WordPress REST.
func parseJSONFeed(data []byte) ([]FeedItem, error) {
	if data[0] == '[' {
		var posts []wpPost
		if err := json.Unmarshal(data, &posts); err != nil {
			return nil, err
		}
		var items []FeedItem
		for _, p := range posts {
			link := strings.TrimSpace(p.Link)
			if link == "" {
				continue
			}
			desc := p.Excerpt.Rendered
			if desc == "" {
				desc = p.Content.Rendered
			}
			// date_gmt viene sin zona: es UTC. "date" es hora local del sitio.
			pub := parseDateIn(p.DateGMT, time.UTC)
			if pub.IsZero() {
				pub = parseDate(p.Date)
			}
			items = append(items, FeedItem{
				Title:       strings.TrimSpace(html.UnescapeString(p.Title.Rendered)),
				Link:        link,
				Description: strings.TrimSpace(desc),
				Published:   pub,
				GUID:        link,
				ImageURL:    strings.TrimSpace(p.Image),
			})
		}
		if len(items) == 0 {
			return nil, fmt.Errorf("no WordPress posts found")
		}
		return items, nil
	}
	var jf jsonFeed
	if err := json.Unmarshal(data, &jf); err != nil {
		return nil, err
	}
	var items []FeedItem
	for _, it := range jf.Items {
		link := strings.TrimSpace(it.URL)
		if link == "" {
			continue
		}
		desc := firstNonEmpty(it.Summary, it.ContentHTML, it.ContentText)
		guid := strings.TrimSpace(it.ID)
		if guid == "" {
			guid = link
		}
		items = append(items, FeedItem{
			Title:       strings.TrimSpace(it.Title),
			Link:        link,
			Description: strings.TrimSpace(desc),
			Published:   parseDate(firstNonEmpty(it.DatePublished, it.DateModified)),
			GUID:        guid,
			ImageURL:    strings.TrimSpace(it.Image),
			Categories:  it.Tags,
		})
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("no JSON Feed items found")
	}
	return items, nil
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
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
			Published:   parseDate(firstNonEmpty(ri.PubDate, ri.DCDate)),
			ImageURL:    extractRSSImageURL(ri),
			Categories:  ri.Categories,
			SourceName:  strings.TrimSpace(ri.Source.Name),
			SourceURL:   strings.TrimSpace(ri.Source.URL),
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
			Published:   parseDate(firstNonEmpty(entry.Published, entry.Updated)),
		}
		for _, c := range entry.Categories {
			if c.Label != "" {
				item.Categories = append(item.Categories, c.Label)
			} else {
				item.Categories = append(item.Categories, c.Term)
			}
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
func parseDate(s string) time.Time { return parseDateIn(s, prLocation()) }

// prLocation es la zona de Puerto Rico (AST, sin horario de verano); las
// fechas sin zona de los feeds locales se interpretan ahí.
func prLocation() *time.Location {
	if loc, err := time.LoadLocation("America/Puerto_Rico"); err == nil {
		return loc
	}
	return time.FixedZone("AST", -4*3600)
}

// parseDateIn es parseDate pero con la zona a usar cuando la fecha no trae una.
func parseDateIn(s string, loc *time.Location) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}

	formats := []string{
		time.RFC1123Z,          // Mon, 02 Jan 2006 15:04:05 -0700
		time.RFC1123,           // Mon, 02 Jan 2006 15:04:05 MST
		time.RFC3339,           // 2006-01-02T15:04:05Z07:00
		time.RFC3339Nano,       // 2006-01-02T15:04:05.999999999Z07:00
		"2006-01-02T15:04:05Z", // ISO without offset
		"2006-01-02T15:04:05",  // ISO without timezone
		"2006-01-02",           // Date only
		"Mon, 2 Jan 2006 15:04:05 -0700",
		"Mon, 2 Jan 2006 15:04:05 MST",
		"02 Jan 2006 15:04:05 -0700",
		"02 Jan 2006 15:04:05 MST",
		"Mon, Jan 2 2006 03:04:05 PM", // Telemundo PR / WAPA: "Mon, Nov 16 2026 09:45:54 AM"
		"2006-01-02T15:04:05-0700",
		"2006-01-02 15:04:05",
	}

	for _, f := range formats {
		t, err := time.ParseInLocation(f, s, loc)
		if err == nil {
			return t
		}
	}

	return time.Time{}
}
