package agents

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// ScanGoogleNews fetches Google News RSS for each search query.
func ScanGoogleNews(ctx context.Context, org models.WatchlistOrg, queries []string, deps Deps) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		feedURL := fmt.Sprintf(
			"https://news.google.com/rss/search?q=%s&hl=es-419&gl=PR&ceid=PR:es-419",
			url.QueryEscape(query),
		)

		agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
		items, err := scraper.ParseFeed(agentCtx, feedURL)
		cancel()

		if err != nil {
			slog.Warn("watchlist/google_news: parse feed", "query", query, "err", err)
			continue
		}

		for _, item := range items {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.Link == "" {
				continue
			}
			if isSpamHit(item.Link, item.Title, item.Description) {
				continue
			}

			urlHash := scraper.HashURL(item.Link)
			hit := &models.WatchlistHit{
				ID:         uuid.New(),
				OrgID:      org.ID,
				SourceType: "google_news",
				Title:      item.Title,
				URL:        item.Link,
				URLHash:    urlHash,
				Snippet:    truncateStr(item.Description, 500),
				Sentiment:  "unknown",
			}

			if err := deps.Hits.Create(ctx, hit); err != nil {
				slog.Error("watchlist/google_news: create hit", "err", err)
				continue
			}
			if hit.ID != uuid.Nil {
				hits++
			}
		}
	}

	if hits > 0 {
		slog.Info("watchlist/google_news: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
