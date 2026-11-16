package watch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/Saul-Punybz/folio/internal/models"
)

// ScanYouTube lee el RSS de cada canal (sin llave). En una marca, el canal es
// suyo: entran todos sus videos. En temas y personas, solo los que la nombran.
func ScanYouTube(ctx context.Context, org models.WatchlistOrg, m Matcher, deps Deps) int {
	hits := 0
	for _, channelID := range youtubeChannels(org) {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}
		feedURL := fmt.Sprintf("https://www.youtube.com/feeds/videos.xml?channel_id=%s", url.QueryEscape(channelID))
		items, err := parseFeed(ctx, feedURL)
		if err != nil {
			slog.Warn("watchlist/youtube: parse feed", "channel", channelID, "err", err)
			continue
		}
		for _, item := range items {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.Link == "" || Excluded(item.Title+" "+item.Description, org) {
				continue
			}
			if !org.IsBrand() {
				if _, ok := m.Match(item.Title+" "+item.Description, org); !ok {
					continue
				}
			}
			tag := ""
			for _, h := range org.Hashtags {
				if hasHashtag(item.Title+" "+item.Description, h) {
					tag = normTag(h)
					break
				}
			}
			if saveHit(ctx, deps, org, "youtube", match{Tag: tag}, item.Title, item.Link, item.Description, item.Published) {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("watchlist/youtube: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
