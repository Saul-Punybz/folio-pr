package handlers

import (
	"encoding/xml"
	"fmt"
	"html"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
)

// FeedHandler serves public RSS feeds authenticated by feed token.
type FeedHandler struct {
	Users *models.UserStore
	Hits  *models.WatchlistHitStore
}

// ServeFeed serves an RSS 2.0 XML feed of watchlist hits for the user
// identified by the feed token in the URL. No session auth required.
func (h *FeedHandler) ServeFeed(w http.ResponseWriter, r *http.Request) {
	token := chi.URLParam(r, "token")
	if token == "" {
		http.NotFound(w, r)
		return
	}

	user, err := h.Users.GetByFeedToken(r.Context(), token)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	hits, err := h.Hits.ListRecentByUser(r.Context(), user.ID, 100)
	if err != nil {
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	// HTTP caching: use most recent hit's CreatedAt as Last-Modified.
	if len(hits) > 0 {
		lastMod := hits[0].CreatedAt.UTC()
		w.Header().Set("Last-Modified", lastMod.Format(http.TimeFormat))
		etag := fmt.Sprintf(`"%x-%d"`, lastMod.Unix(), len(hits))
		w.Header().Set("ETag", etag)

		// Handle conditional GET (If-Modified-Since).
		if ifMod := r.Header.Get("If-Modified-Since"); ifMod != "" {
			if t, err := http.ParseTime(ifMod); err == nil && !lastMod.After(t) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
		// Handle conditional GET (If-None-Match).
		if ifNone := r.Header.Get("If-None-Match"); ifNone != "" {
			if strings.Contains(ifNone, etag) {
				w.WriteHeader(http.StatusNotModified)
				return
			}
		}
	}
	w.Header().Set("Cache-Control", "public, max-age=1800")

	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	baseURL := fmt.Sprintf("%s://%s", scheme, host)
	selfURL := fmt.Sprintf("%s/feed/%s.xml", baseURL, token)

	lastBuild := time.Now().UTC().Format(time.RFC1123Z)
	if len(hits) > 0 {
		lastBuild = hits[0].CreatedAt.UTC().Format(time.RFC1123Z)
	}

	feed := rssChannel{
		Title:       fmt.Sprintf("Folio Vigilancia — %s", user.Email),
		Link:        baseURL,
		Description: "Menciones de organizaciones monitoreadas",
		Language:    "es",
		LastBuild:   lastBuild,
		TTL:         360,
		AtomLink: rssAtomLink{
			Href: selfURL,
			Rel:  "self",
			Type: "application/rss+xml",
		},
	}

	for _, hit := range hits {
		// Plain text description (for readers that only show description).
		desc := hit.Snippet
		if hit.Sentiment != "" {
			desc += fmt.Sprintf(" [%s]", hit.Sentiment)
		}
		if hit.AIDraft != nil && *hit.AIDraft != "" {
			preview := *hit.AIDraft
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			desc += "\n\nBorrador PR: " + preview
		}

		// Rich HTML for content:encoded.
		contentHTML := buildContentHTML(hit)

		item := rssItem{
			Title:          fmt.Sprintf("[%s] %s", hit.OrgName, hit.Title),
			Link:           hit.URL,
			Desc:           desc,
			ContentEncoded: cdataStr{Value: contentHTML},
			Author:         hit.OrgName,
			PubDate:        hit.CreatedAt.UTC().Format(time.RFC1123Z),
			GUID: rssGUID{
				IsPermaLink: "false",
				Value:       hit.ID.String(),
			},
			Category: hit.SourceType,
		}
		feed.Items = append(feed.Items, item)
	}

	rss := rssFeed{
		Version:   "2.0",
		NSContent: "http://purl.org/rss/1.0/modules/content/",
		NSAtom:    "http://www.w3.org/2005/Atom",
		Channel:   feed,
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	enc.Encode(rss)
}

// buildContentHTML creates rich HTML for the content:encoded field.
func buildContentHTML(hit models.WatchlistHit) string {
	var b strings.Builder

	// Snippet as paragraph.
	if hit.Snippet != "" {
		b.WriteString("<p>")
		b.WriteString(html.EscapeString(hit.Snippet))
		b.WriteString("</p>")
	}

	// Sentiment badge.
	if hit.Sentiment != "" {
		color := "#6b7280" // gray
		switch hit.Sentiment {
		case "positive":
			color = "#059669"
		case "negative":
			color = "#dc2626"
		}
		b.WriteString(fmt.Sprintf(
			`<p><span style="display:inline-block;padding:2px 8px;border-radius:4px;font-size:12px;font-weight:bold;color:#fff;background:%s">%s</span></p>`,
			color, html.EscapeString(strings.ToUpper(hit.Sentiment)),
		))
	}

	// AI draft preview as blockquote.
	if hit.AIDraft != nil && *hit.AIDraft != "" {
		preview := *hit.AIDraft
		if len(preview) > 500 {
			preview = preview[:500] + "..."
		}
		b.WriteString("<blockquote style=\"border-left:3px solid #6366f1;padding-left:12px;color:#4b5563;\">")
		b.WriteString("<strong>Borrador PR:</strong><br/>")
		b.WriteString(html.EscapeString(preview))
		b.WriteString("</blockquote>")
	}

	// Source type + org name footer.
	b.WriteString("<p style=\"font-size:11px;color:#9ca3af;\">")
	b.WriteString(html.EscapeString(strings.ToUpper(hit.SourceType)))
	if hit.OrgName != "" {
		b.WriteString(" &mdash; ")
		b.WriteString(html.EscapeString(hit.OrgName))
	}
	b.WriteString("</p>")

	return b.String()
}

// GetFeedURL returns the RSS feed URL for the authenticated user.
// Generates a feed token if the user doesn't have one yet.
func (h *FeedHandler) GetFeedURL(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	token, err := h.Users.SetFeedToken(r.Context(), user.ID)
	if err != nil {
		http.Error(w, `{"error":"failed to generate feed token"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"url": fmt.Sprintf("/feed/%s.xml", token),
	})
}

// RegenerateFeedURL generates a new feed token, invalidating the old one.
func (h *FeedHandler) RegenerateFeedURL(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	token, err := h.Users.ResetFeedToken(r.Context(), user.ID)
	if err != nil {
		http.Error(w, `{"error":"failed to regenerate feed token"}`, http.StatusInternalServerError)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{
		"url": fmt.Sprintf("/feed/%s.xml", token),
	})
}

// ── RSS XML types ────────────────────────────────────────────────

type rssFeed struct {
	XMLName   xml.Name   `xml:"rss"`
	Version   string     `xml:"version,attr"`
	NSContent string     `xml:"xmlns:content,attr"`
	NSAtom    string     `xml:"xmlns:atom,attr"`
	Channel   rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string      `xml:"title"`
	Link        string      `xml:"link"`
	Description string      `xml:"description"`
	Language    string      `xml:"language"`
	LastBuild   string      `xml:"lastBuildDate"`
	TTL         int         `xml:"ttl"`
	AtomLink    rssAtomLink `xml:"atom:link"`
	Items       []rssItem   `xml:"item"`
}

type rssAtomLink struct {
	Href string `xml:"href,attr"`
	Rel  string `xml:"rel,attr"`
	Type string `xml:"type,attr"`
}

type rssItem struct {
	Title          string   `xml:"title"`
	Link           string   `xml:"link"`
	Desc           string   `xml:"description"`
	ContentEncoded cdataStr `xml:"content:encoded"`
	Author         string   `xml:"author"`
	PubDate        string   `xml:"pubDate"`
	GUID           rssGUID  `xml:"guid"`
	Category       string   `xml:"category"`
}

type rssGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type cdataStr struct {
	Value string `xml:",cdata"`
}
