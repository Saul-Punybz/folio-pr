package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/agents"
	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

const (
	maxResultsPerAgent = 20
	agentTimeout       = 30 * time.Second
)

// RunPhase1 performs broad search across 7 agents. Returns the total number of new findings.
func RunPhase1(ctx context.Context, deps Deps, project *models.ResearchProject, queries []string) (int, error) {
	slog.Info("research/phase1: starting broad search", "topic", project.Topic, "queries", len(queries))
	total := 0

	// 1. Google News RSS
	n := searchGoogleNews(ctx, deps, project.ID, queries)
	total += n

	// 2. Bing News RSS
	n = searchBingNews(ctx, deps, project.ID, queries)
	total += n

	// 3. DuckDuckGo Web Search
	n = searchWeb(ctx, deps, project.ID, queries)
	total += n

	// 4. Local DB articles
	n = searchLocal(ctx, deps, project)
	total += n

	// 5. YouTube
	n = searchYouTube(ctx, deps, project.ID, queries)
	total += n

	// 6. Reddit
	n = searchReddit(ctx, deps, project.ID, queries)
	total += n

	// 7. Crawled government pages (crawl index)
	n = searchCrawlIndex(ctx, deps, project, queries)
	total += n

	// Update progress
	progress, _ := json.Marshal(map[string]int{"phase1_hits": total})
	_ = deps.Projects.UpdateProgress(ctx, project.ID, progress)

	slog.Info("research/phase1: complete", "topic", project.Topic, "total_hits", total)
	return total, nil
}

func searchGoogleNews(ctx context.Context, deps Deps, projectID uuid.UUID, queries []string) int {
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
			slog.Warn("research/phase1/google_news: parse feed", "query", query, "err", err)
			continue
		}

		for _, item := range items {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.Link == "" {
				continue
			}
			if agents.IsSpamHit(item.Link, item.Title, item.Description) {
				continue
			}

			f := &models.ResearchFinding{
				ID:         uuid.New(),
				ProjectID:  projectID,
				URL:        item.Link,
				URLHash:    scraper.HashURL(item.Link),
				Title:      item.Title,
				Snippet:    truncateStr(item.Description, 500),
				SourceType: "google_news",
				Sentiment:  "unknown",
			}
			if !item.Published.IsZero() {
				t := item.Published
				f.PublishedAt = &t
			}

			if err := deps.Findings.Create(ctx, f); err != nil {
				slog.Error("research/phase1/google_news: create finding", "err", err)
				continue
			}
			if f.ID != uuid.Nil {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("research/phase1/google_news: done", "new_hits", hits)
	}
	return hits
}

func searchBingNews(ctx context.Context, deps Deps, projectID uuid.UUID, queries []string) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
		results, err := scraper.BingNewsSearch(agentCtx, query, maxResultsPerAgent)
		cancel()

		if err != nil {
			slog.Warn("research/phase1/bing_news: search", "query", query, "err", err)
			continue
		}

		for _, item := range results {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.URL == "" {
				continue
			}
			if agents.IsSpamHit(item.URL, item.Title, item.Snippet) {
				continue
			}

			f := &models.ResearchFinding{
				ID:         uuid.New(),
				ProjectID:  projectID,
				URL:        item.URL,
				URLHash:    scraper.HashURL(item.URL),
				Title:      item.Title,
				Snippet:    truncateStr(item.Snippet, 500),
				SourceType: "bing_news",
				Sentiment:  "unknown",
			}

			if err := deps.Findings.Create(ctx, f); err != nil {
				slog.Error("research/phase1/bing_news: create finding", "err", err)
				continue
			}
			if f.ID != uuid.Nil {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("research/phase1/bing_news: done", "new_hits", hits)
	}
	return hits
}

func searchWeb(ctx context.Context, deps Deps, projectID uuid.UUID, queries []string) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		results, err := scraper.WebSearch(ctx, query, maxResultsPerAgent)
		if err != nil {
			slog.Warn("research/phase1/web: search", "query", query, "err", err)
			continue
		}

		for _, item := range results {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.URL == "" {
				continue
			}
			if agents.IsSpamHit(item.URL, item.Title, item.Snippet) {
				continue
			}

			f := &models.ResearchFinding{
				ID:         uuid.New(),
				ProjectID:  projectID,
				URL:        item.URL,
				URLHash:    scraper.HashURL(item.URL),
				Title:      item.Title,
				Snippet:    truncateStr(item.Snippet, 500),
				SourceType: "web",
				Sentiment:  "unknown",
			}

			if err := deps.Findings.Create(ctx, f); err != nil {
				slog.Error("research/phase1/web: create finding", "err", err)
				continue
			}
			if f.ID != uuid.Nil {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("research/phase1/web: done", "new_hits", hits)
	}
	return hits
}

func searchLocal(ctx context.Context, deps Deps, project *models.ResearchProject) int {
	recent, err := deps.Articles.ListRecent(ctx, 100)
	if err != nil {
		slog.Error("research/phase1/local: list recent", "err", err)
		return 0
	}

	hits := 0
	searchTerms := append([]string{project.Topic}, project.Keywords...)

	for _, article := range recent {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		text := strings.ToLower(article.Title + " " + article.CleanText)
		matched := false
		for _, term := range searchTerms {
			if len(term) > 0 && strings.Contains(text, strings.ToLower(term)) {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}

		if agents.IsSpamHit(article.URL, article.Title, article.CleanText) {
			continue
		}

		snippet := article.Summary
		if snippet == "" && len(article.CleanText) > 500 {
			snippet = article.CleanText[:500]
		}

		f := &models.ResearchFinding{
			ID:         uuid.New(),
			ProjectID:  project.ID,
			URL:        article.URL,
			URLHash:    scraper.HashURL(article.URL),
			Title:      article.Title,
			Snippet:    truncateStr(snippet, 500),
			CleanText:  article.CleanText,
			SourceType: "local",
			Sentiment:  "unknown",
			Scraped:    true, // Already have full text
		}
		if article.PublishedAt != nil {
			f.PublishedAt = article.PublishedAt
		}

		if err := deps.Findings.Create(ctx, f); err != nil {
			slog.Error("research/phase1/local: create finding", "err", err)
			continue
		}
		if f.ID != uuid.Nil {
			hits++
		}
	}

	if hits > 0 {
		slog.Info("research/phase1/local: done", "new_hits", hits)
	}
	return hits
}

func searchYouTube(ctx context.Context, deps Deps, projectID uuid.UUID, queries []string) int {
	hits := 0
	// Search YouTube via RSS search (limited but free)
	for _, query := range queries[:min(3, len(queries))] {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		feedURL := fmt.Sprintf(
			"https://www.youtube.com/feeds/videos.xml?search_query=%s",
			url.QueryEscape(query),
		)

		agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
		items, err := scraper.ParseFeed(agentCtx, feedURL)
		cancel()

		if err != nil {
			slog.Warn("research/phase1/youtube: parse feed", "query", query, "err", err)
			continue
		}

		for _, item := range items {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.Link == "" {
				continue
			}

			f := &models.ResearchFinding{
				ID:         uuid.New(),
				ProjectID:  projectID,
				URL:        item.Link,
				URLHash:    scraper.HashURL(item.Link),
				Title:      item.Title,
				Snippet:    truncateStr(item.Description, 500),
				SourceType: "youtube",
				Sentiment:  "unknown",
			}
			if !item.Published.IsZero() {
				t := item.Published
				f.PublishedAt = &t
			}

			if err := deps.Findings.Create(ctx, f); err != nil {
				slog.Error("research/phase1/youtube: create finding", "err", err)
				continue
			}
			if f.ID != uuid.Nil {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("research/phase1/youtube: done", "new_hits", hits)
	}
	return hits
}

func searchReddit(ctx context.Context, deps Deps, projectID uuid.UUID, queries []string) int {
	hits := 0
	for _, query := range queries[:min(3, len(queries))] {
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
			slog.Warn("research/phase1/reddit: parse feed", "query", query, "err", err)
			continue
		}

		for _, item := range items {
			if hits >= maxResultsPerAgent {
				break
			}
			if item.Link == "" {
				continue
			}
			if agents.IsSpamHit(item.Link, item.Title, item.Description) {
				continue
			}

			f := &models.ResearchFinding{
				ID:         uuid.New(),
				ProjectID:  projectID,
				URL:        item.Link,
				URLHash:    scraper.HashURL(item.Link),
				Title:      item.Title,
				Snippet:    truncateStr(item.Description, 500),
				SourceType: "reddit",
				Sentiment:  "unknown",
			}

			if err := deps.Findings.Create(ctx, f); err != nil {
				slog.Error("research/phase1/reddit: create finding", "err", err)
				continue
			}
			if f.ID != uuid.Nil {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("research/phase1/reddit: done", "new_hits", hits)
	}
	return hits
}

func searchCrawlIndex(ctx context.Context, deps Deps, project *models.ResearchProject, queries []string) int {
	if deps.CrawledPages == nil {
		return 0
	}

	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}

		findings, err := crawler.SearchCrawlIndex(ctx, deps.CrawledPages, query, maxResultsPerAgent-hits)
		if err != nil {
			slog.Warn("research/phase1/crawl_index: search", "query", query, "err", err)
			continue
		}

		for _, f := range findings {
			if hits >= maxResultsPerAgent {
				break
			}

			f.ProjectID = project.ID
			if err := deps.Findings.Create(ctx, &f); err != nil {
				slog.Error("research/phase1/crawl_index: create finding", "err", err)
				continue
			}
			if f.ID != uuid.Nil {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("research/phase1/crawl_index: done", "new_hits", hits)
	}
	return hits
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
