package watch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/Saul-Punybz/folio/internal/models"
)

// ScanReddit lee la búsqueda pública de Reddit en RSS. Ojo: desde 2025 Reddit
// suele responder 403/429 sin llave; si falla, se anota y se sigue.
func ScanReddit(ctx context.Context, org models.WatchlistOrg, queries []string, m Matcher, deps Deps) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}
		feedURL := fmt.Sprintf("https://www.reddit.com/search.rss?q=%s&sort=new", url.QueryEscape(query))
		items, err := parseFeed(ctx, feedURL)
		if err != nil {
			slog.Warn("watchlist/reddit: parse feed", "query", query, "err", err)
			break // si Reddit bloquea una, bloquea todas
		}
		for _, item := range items {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.Link == "" || isSpamHit(item.Link, item.Title, item.Description) {
				continue
			}
			mm := m
			mm.NeedPR = true
			mt, ok := matchItem(item.Title+" "+stripHTML(item.Description), org, mm)
			if !ok {
				continue
			}
			if saveHit(ctx, deps, org, "reddit", mt, item.Title, item.Link, stripHTML(item.Description), item.Published) {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("watchlist/reddit: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
