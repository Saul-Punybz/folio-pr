package handlers

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
)

// SourcesHandler groups source management HTTP handlers.
type SourcesHandler struct {
	Sources *models.SourceStore
}

// ListSources handles GET /api/sources — returns ALL sources (active and inactive).
func (h *SourcesHandler) ListSources(w http.ResponseWriter, r *http.Request) {
	sources, err := h.Sources.ListAll(r.Context())
	if err != nil {
		slog.Error("list sources", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if sources == nil {
		sources = []models.Source{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sources": sources,
		"count":   len(sources),
	})
}

// CreateSource handles POST /api/sources.
func (h *SourcesHandler) CreateSource(w http.ResponseWriter, r *http.Request) {
	var src models.Source
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if src.Name == "" || src.BaseURL == "" || src.Region == "" || src.FeedType == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name, base_url, region, and feed_type are required"})
		return
	}

	if err := h.Sources.Create(r.Context(), &src); err != nil {
		slog.Error("create source", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create source"})
		return
	}

	writeJSON(w, http.StatusCreated, src)
}

// UpdateSource handles PUT /api/sources/{id}.
func (h *SourcesHandler) UpdateSource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid source id"})
		return
	}

	var src models.Source
	if err := json.NewDecoder(r.Body).Decode(&src); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	src.ID = id

	if err := h.Sources.Update(r.Context(), &src); err != nil {
		slog.Error("update source", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update source"})
		return
	}

	writeJSON(w, http.StatusOK, src)
}

// ToggleSource handles PATCH /api/sources/{id}/toggle.
func (h *SourcesHandler) ToggleSource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid source id"})
		return
	}

	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.Sources.ToggleActive(r.Context(), id, body.Active); err != nil {
		slog.Error("toggle source", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not toggle source"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"id": id, "active": body.Active})
}

// DeleteSource handles DELETE /api/sources/{id}.
func (h *SourcesHandler) DeleteSource(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid source id"})
		return
	}

	if err := h.Sources.Delete(r.Context(), id); err != nil {
		slog.Error("delete source", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete source"})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// QuickCreateSource handles POST /api/sources/quick.
// Accepts just a URL, auto-detects if it's RSS/Atom, and creates a source.
func (h *SourcesHandler) QuickCreateSource(w http.ResponseWriter, r *http.Request) {
	var body struct {
		URL    string `json:"url"`
		Region string `json:"region,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if body.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	region := body.Region
	if region == "" {
		region = "PR"
	}

	parsed, err := url.Parse(body.URL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid url"})
		return
	}

	// Probe the URL to detect feed type.
	result, err := probeURL(body.URL)
	if err != nil {
		slog.Error("quick source: probe", "url", body.URL, "err", err)
		writeJSON(w, http.StatusUnprocessableEntity, map[string]string{
			"error": fmt.Sprintf("could not probe URL: %v", err),
		})
		return
	}

	src := models.Source{
		BaseURL:  fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host),
		Region:   region,
		Active:   true,
		FeedType: result.feedType,
	}

	if result.feedType == "rss" {
		src.FeedURL = result.feedURL
		src.Name = result.title
		if src.Name == "" {
			src.Name = parsed.Host
		}
	} else {
		// Scrape source — user will need to configure selectors later.
		src.FeedType = "scrape"
		src.Name = parsed.Host
		src.ListURLs = []string{body.URL}
	}

	if err := h.Sources.Create(r.Context(), &src); err != nil {
		slog.Error("quick source: create", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create source"})
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"source":    src,
		"feed_type": result.feedType,
		"detected":  result.feedType == "rss",
		"message":   quickSourceMessage(result.feedType),
	})
}

type probeResult struct {
	feedType string // "rss" or "scrape"
	feedURL  string // resolved feed URL (might differ from input)
	title    string // feed title if found
}

var reRSSLink = regexp.MustCompile(`<link[^>]+type=["']application/(rss|atom)\+xml["'][^>]*>`)
var reHrefAttr = regexp.MustCompile(`href=["']([^"']+)["']`)
var reTitleAttr = regexp.MustCompile(`title=["']([^"']+)["']`)

func probeURL(rawURL string) (*probeResult, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Folio/1.0")
	req.Header.Set("Accept", "application/rss+xml, application/atom+xml, application/xml, text/xml, text/html")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // 512KB limit for probing
	if err != nil {
		return nil, err
	}

	ct := resp.Header.Get("Content-Type")

	// Check if the response itself is XML/RSS/Atom.
	if isXMLContentType(ct) || looksLikeXML(bodyBytes) {
		title := extractFeedTitle(bodyBytes)
		return &probeResult{feedType: "rss", feedURL: rawURL, title: title}, nil
	}

	// It's HTML — look for <link rel="alternate" type="application/rss+xml">
	if feedURL := findRSSLinkInHTML(string(bodyBytes), rawURL); feedURL != "" {
		// Try to extract the title from the RSS link tag or fall back to probing
		title := findRSSLinkTitle(string(bodyBytes))
		return &probeResult{feedType: "rss", feedURL: feedURL, title: title}, nil
	}

	// No RSS found — treat as scrape
	return &probeResult{feedType: "scrape"}, nil
}

func isXMLContentType(ct string) bool {
	ct = strings.ToLower(ct)
	return strings.Contains(ct, "xml") || strings.Contains(ct, "rss") || strings.Contains(ct, "atom")
}

func looksLikeXML(data []byte) bool {
	trimmed := strings.TrimSpace(string(data[:min(500, len(data))]))
	return strings.HasPrefix(trimmed, "<?xml") || strings.HasPrefix(trimmed, "<rss") || strings.HasPrefix(trimmed, "<feed")
}

func extractFeedTitle(data []byte) string {
	// Try RSS
	type rssProbe struct {
		XMLName xml.Name `xml:"rss"`
		Channel struct {
			Title string `xml:"title"`
		} `xml:"channel"`
	}
	var rss rssProbe
	if err := xml.Unmarshal(data, &rss); err == nil && rss.Channel.Title != "" {
		return strings.TrimSpace(rss.Channel.Title)
	}

	// Try Atom
	type atomProbe struct {
		XMLName xml.Name `xml:"feed"`
		Title   string   `xml:"title"`
	}
	var atom atomProbe
	if err := xml.Unmarshal(data, &atom); err == nil && atom.Title != "" {
		return strings.TrimSpace(atom.Title)
	}

	return ""
}

func findRSSLinkInHTML(html, baseURL string) string {
	matches := reRSSLink.FindAllString(html, 5)
	for _, m := range matches {
		href := reHrefAttr.FindStringSubmatch(m)
		if len(href) >= 2 {
			feedURL := strings.TrimSpace(href[1])
			// Resolve relative URLs
			if strings.HasPrefix(feedURL, "/") {
				if parsed, err := url.Parse(baseURL); err == nil {
					feedURL = fmt.Sprintf("%s://%s%s", parsed.Scheme, parsed.Host, feedURL)
				}
			}
			return feedURL
		}
	}
	return ""
}

func findRSSLinkTitle(html string) string {
	matches := reRSSLink.FindAllString(html, 1)
	if len(matches) > 0 {
		title := reTitleAttr.FindStringSubmatch(matches[0])
		if len(title) >= 2 {
			return strings.TrimSpace(title[1])
		}
	}
	return ""
}

func quickSourceMessage(feedType string) string {
	if feedType == "rss" {
		return "RSS feed detected and source created. Articles will appear on next worker cycle."
	}
	return "No RSS feed detected. Source created as scrape type — configure CSS selectors in Settings > Sources."
}
