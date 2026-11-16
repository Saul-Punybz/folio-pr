package watch

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/Saul-Punybz/folio/internal/models"
)

// ScanBingNews lee Bing News RSS para cada búsqueda (con la fecha real de
// publicación y la URL real, sin el apiclick.aspx de Bing).
func ScanBingNews(ctx context.Context, org models.WatchlistOrg, queries []string, m Matcher, deps Deps) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}
		feedURL := fmt.Sprintf("https://www.bing.com/news/search?q=%s&format=rss", url.QueryEscape(query))
		items, err := parseFeed(ctx, feedURL)
		if err != nil {
			slog.Warn("watchlist/bing_news: search", "query", query, "err", err)
			continue
		}
		for _, item := range items {
			if hits >= maxResultsPerAgent {
				break
			}
			link := unwrapRedirect(item.Link)
			desc := stripHTML(item.Description)
			if link == "" || isSpamHit(link, item.Title, desc) {
				continue
			}
			mt, ok := matchItem(item.Title+" "+desc, org, m)
			if !ok {
				continue
			}
			if saveHit(ctx, deps, org, "bing_news", mt, item.Title, link, desc, item.Published) {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("watchlist/bing_news: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}
