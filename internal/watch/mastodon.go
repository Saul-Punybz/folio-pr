package watch

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
)

// Mastodon: línea de tiempo pública de un hashtag, sin cuenta, en tres
// instancias grandes (las tres respondieron 200 el 16 nov 2026; mstdn.social, 422).
var mastodonInstances = []string{"https://mastodon.social", "https://mas.to", "https://mastodon.online"}

type mastoStatus struct {
	URL       string `json:"url"`
	URI       string `json:"uri"`
	CreatedAt string `json:"created_at"`
	Content   string `json:"content"`
	Account   struct {
		Acct        string `json:"acct"`
		DisplayName string `json:"display_name"`
	} `json:"account"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

var reTags = regexp.MustCompile(`<[^>]*>`)

// stripHTML quita etiquetas y entidades del contenido de Mastodon.
func stripHTML(s string) string {
	s = strings.NewReplacer("<br>", " ", "<br/>", " ", "<br />", " ", "</p>", " ").Replace(s)
	return strings.Join(strings.Fields(html.UnescapeString(reTags.ReplaceAllString(s, " "))), " ")
}

// parseMastodonStatuses lee la respuesta de /api/v1/timelines/tag/<tag>.
func parseMastodonStatuses(body []byte) ([]socialPost, error) {
	var raw []mastoStatus
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("mastodon: json: %w", err)
	}
	out := make([]socialPost, 0, len(raw))
	for _, s := range raw {
		link := s.URL
		if link == "" {
			link = s.URI
		}
		if !strings.HasPrefix(link, "http") {
			continue
		}
		text := stripHTML(s.Content)
		tags := make([]string, 0, len(s.Tags))
		for _, t := range s.Tags {
			tags = append(tags, t.Name)
		}
		author := s.Account.DisplayName
		if author == "" {
			author = s.Account.Acct
		}
		t, _ := time.Parse(time.RFC3339, s.CreatedAt)
		out = append(out, socialPost{URL: link, Author: author, Text: text, MatchText: text, Tags: tags, Published: t})
	}
	return out, nil
}

// ScanMastodon lee los hashtags distintivos de la marca en Mastodon.
func ScanMastodon(ctx context.Context, org models.WatchlistOrg, m Matcher, deps Deps) int {
	saved := 0
	for _, h := range org.Hashtags {
		h = normTag(h)
		if h == "" {
			continue
		}
		for _, inst := range mastodonInstances {
			if saved >= maxResultsPerAgent || ctx.Err() != nil {
				return saved
			}
			body, code, err := fetch(ctx, inst+"/api/v1/timelines/tag/"+url.PathEscape(h)+"?limit=20", "application/json")
			if err != nil {
				slog.Warn("watchlist/mastodon: fetch", "org", org.Name, "tag", h, "code", code, "err", err)
				continue
			}
			posts, err := parseMastodonStatuses(body)
			if err != nil {
				slog.Warn("watchlist/mastodon: parse", "err", err)
				continue
			}
			for _, p := range posts {
				if saved >= maxResultsPerAgent {
					break
				}
				mt, ok := acceptPost(p, org, m, false)
				if !ok {
					continue
				}
				if mt.Tag == "" && mt.Person == "" {
					mt.Tag = h
				}
				if saveHit(ctx, deps, org, "mastodon", mt, postTitle(p), p.URL, p.Text, p.Published) {
					saved++
				}
			}
		}
	}
	if saved > 0 {
		slog.Info("watchlist/mastodon: done", "org", org.Name, "new_hits", saved)
	}
	return saved
}
