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

// ScanReddit fetches Reddit search RSS for each query.
func ScanReddit(ctx context.Context, org models.WatchlistOrg, queries []string, deps Deps) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		feedURL := fmt.Sprintf(
			"https://www.reddit.com/search.rss?q=%s&sort=new",
			url.QueryEscape(query),
		)

		agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
		items, err := scraper.ParseFeed(agentCtx, feedURL)
		cancel()

		if err != nil {
			slog.Warn("watchlist/reddit: parse feed", "query", query, "err", err)
			continue
		}

		for _, item := range items {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.Link == "" {
				continue
			}
			if isSpamHit(item.Link, item.Title, item.Description, append([]string{org.Name}, org.Keywords...)...) {
				continue
			}

			urlHash := scraper.HashURL(item.Link)
			hit := &models.WatchlistHit{
				ID:         uuid.New(),
				OrgID:      org.ID,
				SourceType: "reddit",
				Title:      item.Title,
				URL:        item.Link,
				URLHash:    urlHash,
				Snippet:    truncateStr(item.Description, 500),
				Sentiment:  "unknown",
			}

			if err := deps.Hits.Create(ctx, hit); err != nil {
				slog.Error("watchlist/reddit: create hit", "err", err)
				continue
			}
			if hit.ID != uuid.Nil {
				hits++
			}
		}
	}

	if hits > 0 {
		slog.Info("watchlist/reddit: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
