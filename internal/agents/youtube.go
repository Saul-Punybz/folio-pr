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

// ScanYouTube fetches YouTube RSS feeds for configured channel IDs.
func ScanYouTube(ctx context.Context, org models.WatchlistOrg, deps Deps) int {
	hits := 0
	for _, channelID := range org.YouTubeChannels {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		feedURL := fmt.Sprintf(
			"https://www.youtube.com/feeds/videos.xml?channel_id=%s",
			url.QueryEscape(channelID),
		)

		agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
		items, err := scraper.ParseFeed(agentCtx, feedURL)
		cancel()

		if err != nil {
			slog.Warn("watchlist/youtube: parse feed", "channel", channelID, "err", err)
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

			// Only include videos that mention org keywords.
			if !containsAnyKeyword(item.Title+" "+item.Description, org) {
				continue
			}

			urlHash := scraper.HashURL(item.Link)
			hit := &models.WatchlistHit{
				ID:         uuid.New(),
				OrgID:      org.ID,
				SourceType: "youtube",
				Title:      item.Title,
				URL:        item.Link,
				URLHash:    urlHash,
				Snippet:    truncateStr(item.Description, 500),
				Sentiment:  "unknown",
			}

			if err := deps.Hits.Create(ctx, hit); err != nil {
				slog.Error("watchlist/youtube: create hit", "err", err)
				continue
			}
			if hit.ID != uuid.Nil {
				hits++
			}
		}
	}

	if hits > 0 {
		slog.Info("watchlist/youtube: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
