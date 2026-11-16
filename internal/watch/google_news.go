package watch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/Saul-Punybz/folio/internal/models"
)

// Google News en español de PR y en inglés de EE. UU. (la prensa de fondos y
// aliados como LISC o NFF sale en inglés).
var googleNewsLocales = []string{"hl=es-419&gl=PR&ceid=PR:es-419", "hl=en-US&gl=US&ceid=US:en"}

// ScanGoogleNews lee Google News RSS (PR, español) para cada búsqueda.
func ScanGoogleNews(ctx context.Context, org models.WatchlistOrg, queries []string, m Matcher, deps Deps) int {
	hits := 0
	for _, query := range queries {
		for _, loc := range googleNewsLocales {
			if hits >= maxResultsPerAgent || ctx.Err() != nil {
				break
			}
			feedURL := fmt.Sprintf("https://news.google.com/rss/search?q=%s&%s", url.QueryEscape(query), loc)
			items, err := parseFeed(ctx, feedURL)
			if err != nil {
				slog.Warn("watchlist/google_news: parse feed", "query", query, "err", err)
				continue
			}
			for _, item := range items {
				if hits >= maxResultsPerAgent {
					break
				}
				if item.Link == "" || isSpamHit(item.Link, item.Title, item.Description) {
					continue
				}
				mt, ok := matchItem(item.Title+" "+stripHTML(item.Description), org, m)
				if !ok {
					continue
				}
				if saveHit(ctx, deps, org, "google_news", mt, item.Title, item.Link, stripHTML(item.Description), item.Published) {
					hits++
				}
			}
		}
	}
	if hits > 0 {
		slog.Info("watchlist/google_news: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
