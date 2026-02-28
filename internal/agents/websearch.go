package agents

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// ScanWeb uses DuckDuckGo web search for each query.
func ScanWeb(ctx context.Context, org models.WatchlistOrg, queries []string, deps Deps) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		results, err := scraper.WebSearch(ctx, query, maxResultsPerAgent)
		if err != nil {
			slog.Warn("watchlist/web: search", "query", query, "err", err)
			continue
		}

		for _, result := range results {
			if hits >= maxResultsPerAgent {
				break
			}
			if result.URL == "" {
				continue
			}
			if isSpamHit(result.URL, result.Title, result.Snippet) {
				continue
			}

			urlHash := scraper.HashURL(result.URL)
			hit := &models.WatchlistHit{
				ID:         uuid.New(),
				OrgID:      org.ID,
				SourceType: "web",
				Title:      result.Title,
				URL:        result.URL,
				URLHash:    urlHash,
				Snippet:    truncateStr(result.Snippet, 500),
				Sentiment:  "unknown",
			}

			if err := deps.Hits.Create(ctx, hit); err != nil {
				slog.Error("watchlist/web: create hit", "err", err)
				continue
			}
			if hit.ID != uuid.Nil {
				hits++
			}
		}
	}

	if hits > 0 {
		slog.Info("watchlist/web: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
