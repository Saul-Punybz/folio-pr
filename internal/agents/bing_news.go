package agents

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// ScanBingNews fetches Bing News RSS for each search query.
func ScanBingNews(ctx context.Context, org models.WatchlistOrg, queries []string, deps Deps) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
		results, err := scraper.BingNewsSearch(agentCtx, query, maxResultsPerAgent)
		cancel()

		if err != nil {
			slog.Warn("watchlist/bing_news: search", "query", query, "err", err)
			continue
		}

		for _, item := range results {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.URL == "" {
				continue
			}
			if isSpamHit(item.URL, item.Title, item.Snippet) {
				continue
			}

			urlHash := scraper.HashURL(item.URL)
			hit := &models.WatchlistHit{
				ID:         uuid.New(),
				OrgID:      org.ID,
				SourceType: "bing_news",
				Title:      item.Title,
				URL:        item.URL,
				URLHash:    urlHash,
				Snippet:    truncateStr(item.Snippet, 500),
				Sentiment:  "unknown",
			}

			if err := deps.Hits.Create(ctx, hit); err != nil {
				slog.Error("watchlist/bing_news: create hit", "err", err)
				continue
			}
			if hit.ID != uuid.Nil {
				hits++
			}
		}
	}

	if hits > 0 {
		slog.Info("watchlist/bing_news: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
