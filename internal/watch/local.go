package watch

import (
	"context"
	"log/slog"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
)

// localWindowHours: noticias de medios de PR que Folio ingirió en los últimos 14 días.
const localWindowHours = 14 * 24

// ScanLocalArticles revisa las noticias que Folio ya descargó.
func ScanLocalArticles(ctx context.Context, org models.WatchlistOrg, m Matcher, deps Deps) int {
	if deps.Articles == nil {
		return 0
	}
	recent, err := deps.Articles.ListRecent(ctx, localWindowHours)
	if err != nil {
		slog.Error("watchlist/local: list recent articles", "err", err)
		return 0
	}
	hits := 0
	for _, article := range recent {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}
		mt, ok := matchItem(article.Title+" "+article.CleanText, org, m)
		if !ok {
			continue
		}
		if isSpamHit(article.URL, article.Title, article.CleanText) {
			continue
		}
		snippet := article.Summary
		if snippet == "" {
			snippet = article.CleanText
		}
		var pub time.Time
		if article.PublishedAt != nil {
			pub = *article.PublishedAt
		}
		if saveHit(ctx, deps, org, "local", mt, article.Title, article.URL, snippet, pub) {
			hits++
		}
	}
	if hits > 0 {
		slog.Info("watchlist/local: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
