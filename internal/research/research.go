// Package research implements the Deep Research (Investigacion Profunda) engine.
// It runs a 3-phase pipeline: broad search, deep scrape, AI synthesis.
package research

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
)

// Deps groups dependencies needed by the research engine.
type Deps struct {
	Projects     *models.ResearchProjectStore
	Findings     *models.ResearchFindingStore
	Articles     *models.ArticleStore
	CrawledPages *models.CrawledPageStore
	AI           *ai.OllamaClient
}

// RunProject orchestrates all 3 research phases for a single project.
func RunProject(ctx context.Context, deps Deps, projectID uuid.UUID) {
	slog.Info("research: starting project", "id", projectID)

	project, err := deps.Projects.GetByID(ctx, projectID)
	if err != nil {
		slog.Error("research: get project", "id", projectID, "err", err)
		return
	}

	// Mark as started
	if err := deps.Projects.SetStarted(ctx, project.ID); err != nil {
		slog.Error("research: set started", "id", projectID, "err", err)
		return
	}

	// Expand keywords if empty
	if len(project.Keywords) == 0 && deps.AI != nil {
		keywords, kwErr := expandTopicKeywords(ctx, deps.AI, project.Topic)
		if kwErr == nil && len(keywords) > 0 {
			project.Keywords = keywords
		}
	}

	// Build search queries
	queries := buildResearchQueries(project.Topic, project.Keywords)

	// ── Phase 1: Broad Search ──
	slog.Info("research: phase 1 — broad search", "topic", project.Topic, "queries", len(queries))
	phase1Hits, err := RunPhase1(ctx, deps, project, queries)
	if err != nil {
		slog.Error("research: phase 1 failed", "err", err)
		_ = deps.Projects.SetFailed(ctx, project.ID, "Phase 1 failed: "+err.Error())
		return
	}

	if ctx.Err() != nil {
		slog.Warn("research: cancelled during phase 1")
		return
	}

	slog.Info("research: phase 1 complete", "hits", phase1Hits)

	// ── Phase 2: Deep Scrape ──
	if err := deps.Projects.UpdateStatus(ctx, project.ID, "scraping", 2); err != nil {
		slog.Error("research: update status to scraping", "err", err)
	}

	slog.Info("research: phase 2 — deep scrape", "topic", project.Topic)
	phase2Scraped, err := RunPhase2(ctx, deps, project)
	if err != nil {
		slog.Error("research: phase 2 failed", "err", err)
		_ = deps.Projects.SetFailed(ctx, project.ID, "Phase 2 failed: "+err.Error())
		return
	}

	if ctx.Err() != nil {
		slog.Warn("research: cancelled during phase 2")
		return
	}

	slog.Info("research: phase 2 complete", "scraped", phase2Scraped)

	// ── Phase 3: AI Synthesis ──
	if err := deps.Projects.UpdateStatus(ctx, project.ID, "synthesizing", 3); err != nil {
		slog.Error("research: update status to synthesizing", "err", err)
	}

	slog.Info("research: phase 3 — synthesis", "topic", project.Topic)
	if err := RunPhase3(ctx, deps, project); err != nil {
		slog.Error("research: phase 3 failed", "err", err)
		_ = deps.Projects.SetFailed(ctx, project.ID, "Phase 3 failed: "+err.Error())
		return
	}

	slog.Info("research: project complete",
		"topic", project.Topic,
		"phase1_hits", phase1Hits,
		"phase2_scraped", phase2Scraped,
		"duration", time.Since(project.CreatedAt).Round(time.Second),
	)
}
