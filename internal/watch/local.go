package watch

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// ScanLocalArticles scans recently ingested articles for keyword matches.
func ScanLocalArticles(ctx context.Context, org models.WatchlistOrg, deps Deps) int {
	recent, err := deps.Articles.ListRecent(ctx, 50)
	if err != nil {
		slog.Error("watchlist/local: list recent articles", "err", err)
		return 0
	}

	hits := 0
	for _, article := range recent {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		if !MatchesOrg(article.Title+" "+article.CleanText, org) {
			continue
		}

		if isSpamHit(article.URL, article.Title, article.CleanText) {
			continue
		}

		urlHash := scraper.HashURL(article.URL)
		snippet := article.Summary
		if snippet == "" {
			snippet = article.CleanText
		}

		hit := &models.WatchlistHit{
			ID:         uuid.New(),
			OrgID:      org.ID,
			SourceType: "local",
			Title:      article.Title,
			URL:        article.URL,
			URLHash:    urlHash,
			Snippet:    truncateStr(snippet, 500),
			Sentiment:  "unknown",
		}

		if err := deps.Hits.Create(ctx, hit); err != nil {
			slog.Error("watchlist/local: create hit", "err", err)
			continue
		}
		if hit.ID != uuid.Nil {
			hits++
		}
	}

	if hits > 0 {
		slog.Info("watchlist/local: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
