package agents

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
)

const (
	maxResultsPerAgent = 10
	agentTimeout       = 30 * time.Second
	scanTimeout        = 2 * time.Hour
)

// Deps groups dependencies needed by all agents.
type Deps struct {
	Orgs     *models.WatchlistOrgStore
	Hits     *models.WatchlistHitStore
	Articles *models.ArticleStore
	AI       *ai.OllamaClient
}

// RunWatchlistScan is the main entry point called by the worker cron.
// It processes orgs SEQUENTIALLY to keep resource usage low.
func RunWatchlistScan(ctx context.Context, deps Deps) {
	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()

	slog.Info("watchlist: starting scan")
	start := time.Now()

	orgs, err := deps.Orgs.ListActive(ctx)
	if err != nil {
		slog.Error("watchlist: list active orgs", "err", err)
		return
	}

	if len(orgs) == 0 {
		slog.Info("watchlist: no active orgs to scan")
		return
	}

	totalHits := 0
	for _, org := range orgs {
		if ctx.Err() != nil {
			break
		}
		hits := scanOrg(ctx, org, deps)
		totalHits += hits
	}

	// Classify sentiment and generate PR drafts for negative hits.
	classifyAndDraft(ctx, deps)

	slog.Info("watchlist: scan complete",
		"orgs", len(orgs),
		"new_hits", totalHits,
		"duration", time.Since(start).Round(time.Millisecond),
	)
}

// scanOrg runs all 5 agents sequentially for a single org.
func scanOrg(ctx context.Context, org models.WatchlistOrg, deps Deps) int {
	slog.Info("watchlist: scanning org", "name", org.Name, "keywords", org.Keywords)
	hits := 0

	queries := buildSearchQueries(org)

	hits += ScanGoogleNews(ctx, org, queries, deps)
	hits += ScanBingNews(ctx, org, queries, deps)
	hits += ScanWeb(ctx, org, queries, deps)
	hits += ScanLocalArticles(ctx, org, deps)

	if len(org.YouTubeChannels) > 0 {
		hits += ScanYouTube(ctx, org, deps)
	}

	hits += ScanReddit(ctx, org, queries, deps)

	slog.Info("watchlist: org scan complete", "name", org.Name, "new_hits", hits)
	return hits
}

// buildSearchQueries builds search queries from the org name and keywords.
// Returns at most 5 queries for broader coverage.
func buildSearchQueries(org models.WatchlistOrg) []string {
	queries := []string{org.Name + " Puerto Rico"}
	for i, kw := range org.Keywords {
		if i >= 4 {
			break
		}
		if !strings.EqualFold(kw, org.Name) {
			queries = append(queries, kw+" Puerto Rico")
		}
	}
	return queries
}

// containsAnyKeyword checks if text mentions the org name or any keyword.
func containsAnyKeyword(text string, org models.WatchlistOrg) bool {
	lower := strings.ToLower(text)
	if strings.Contains(lower, strings.ToLower(org.Name)) {
		return true
	}
	for _, kw := range org.Keywords {
		if len(kw) > 0 && strings.Contains(lower, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// truncateStr shortens a string to maxLen.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
