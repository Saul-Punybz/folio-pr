package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
)

// SearchCrawlIndex searches the crawled pages using full-text search and returns
// results as ResearchFinding records for integration with the ResearchDesk.
func SearchCrawlIndex(ctx context.Context, pages *models.CrawledPageStore, query string, limit int) ([]models.ResearchFinding, error) {
	if limit <= 0 {
		limit = 20
	}

	results, err := pages.SearchFTS(ctx, query, limit)
	if err != nil {
		slog.Warn("crawler/search: FTS search", "query", query, "err", err)
		return nil, fmt.Errorf("search crawl index: %w", err)
	}

	var findings []models.ResearchFinding
	for _, page := range results {
		snippet := page.Summary
		if snippet == "" && len(page.CleanText) > 500 {
			snippet = page.CleanText[:500]
		}

		f := models.ResearchFinding{
			ID:         uuid.New(),
			URL:        page.URL,
			URLHash:    page.URLHash,
			Title:      page.Title,
			Snippet:    snippet,
			CleanText:  page.CleanText,
			SourceType: "crawl_index",
			Sentiment:  page.Sentiment,
			Scraped:    true,
			Tags:       page.Tags,
			Entities:   page.Entities,
		}
		if f.Tags == nil {
			f.Tags = json.RawMessage("[]")
		}
		if f.Entities == nil {
			f.Entities = json.RawMessage("{}")
		}

		findings = append(findings, f)
	}

	slog.Info("crawler/search: results", "query", query, "count", len(findings))
	return findings, nil
}
