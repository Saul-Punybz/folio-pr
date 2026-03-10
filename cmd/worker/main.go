// Command worker runs the Folio background ingestion pipeline. It periodically
// fetches articles from configured sources, scrapes content, runs AI
// enrichment, and manages evidence lifecycle.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Saul-Punybz/folio/internal/agents"
	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/config"
	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/db"
	"github.com/Saul-Punybz/folio/internal/generator"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/research"
	"github.com/Saul-Punybz/folio/internal/scraper"
	"github.com/Saul-Punybz/folio/internal/storage"
)

func main() {
	// Structured JSON logging.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("worker: starting folio worker")

	// Load configuration.
	cfg := config.Load()

	// Create a root context that is cancelled on shutdown.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Connect to the database.
	pool, err := db.Connect(ctx, cfg.DB)
	if err != nil {
		slog.Error("worker: database connection failed", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Create stores.
	articleStore := models.NewArticleStore(pool)
	sourceStore := models.NewSourceStore(pool)
	fingerprintStore := models.NewFingerprintStore(pool)
	sessionStore := models.NewSessionStore(pool)
	briefStore := models.NewBriefStore(pool)
	watchlistOrgStore := models.NewWatchlistOrgStore(pool)
	watchlistHitStore := models.NewWatchlistHitStore(pool)
	entityStore := models.NewEntityStore(pool)
	telegramUserStore := models.NewTelegramUserStore(pool)
	notificationStore := models.NewNotificationStore(pool)
	researchProjectStore := models.NewResearchProjectStore(pool)
	researchFindingStore := models.NewResearchFindingStore(pool)
	escritoStore := models.NewEscritoStore(pool)
	escritoSourceStore := models.NewEscritoSourceStore(pool)

	// Crawler stores.
	crawlDomainStore := models.NewCrawlDomainStore(pool)
	crawlQueueStore := models.NewCrawlQueueStore(pool)
	crawledPageStore := models.NewCrawledPageStore(pool)
	crawlLinkStore := models.NewCrawlLinkStore(pool)
	crawlRunStore := models.NewCrawlRunStore(pool)
	pageEntityStore := models.NewPageEntityStore(pool)
	entityRelStore := models.NewEntityRelationshipStore(pool)

	stores := scraper.Stores{
		Articles:     articleStore,
		Sources:      sourceStore,
		Fingerprints: fingerprintStore,
		Entities:     entityStore,
	}

	// Create scraper.
	sc := scraper.NewScraper()

	// Create AI client.
	aiClient := ai.NewClient(
		cfg.Ollama.Host,
		cfg.Ollama.InstructModel,
		cfg.Ollama.EmbedModel,
	)

	// Create S3 storage client.
	storageClient, err := storage.NewClient(ctx, cfg.S3)
	if err != nil {
		slog.Error("worker: storage client creation failed", "err", err)
		os.Exit(1)
	}

	// Track in-flight jobs for graceful shutdown.
	var wg sync.WaitGroup

	// Set up cron scheduler (standard 5-field cron expressions).
	c := cron.New()

	// Ingestion: every 4 hours (6 times/day).
	_, err = c.AddFunc("0 */4 * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 3*time.Hour)
		defer jobCancel()

		slog.Info("cron: ingestion job triggered")
		scraper.RunIngestion(jobCtx, stores, sc, aiClient, storageClient)
	})
	if err != nil {
		slog.Error("worker: add ingestion cron", "err", err)
		os.Exit(1)
	}

	// Evidence cleanup: daily at 3am.
	_, err = c.AddFunc("0 3 * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 30*time.Minute)
		defer jobCancel()

		slog.Info("cron: evidence cleanup job triggered")
		scraper.RunEvidenceCleanup(jobCtx, stores, storageClient)
	})
	if err != nil {
		slog.Error("worker: add evidence cleanup cron", "err", err)
		os.Exit(1)
	}

	// Session cleanup: daily at 4am.
	_, err = c.AddFunc("0 4 * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 5*time.Minute)
		defer jobCancel()

		slog.Info("cron: session cleanup job triggered")
		scraper.RunSessionCleanup(jobCtx, sessionStore)
	})
	if err != nil {
		slog.Error("worker: add session cleanup cron", "err", err)
		os.Exit(1)
	}

	// Daily brief generation: daily at 5am.
	_, err = c.AddFunc("0 5 * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 10*time.Minute)
		defer jobCancel()

		slog.Info("cron: daily brief generation triggered")
		scraper.GenerateDailyBrief(jobCtx, articleStore, briefStore, aiClient)

		// Create digest notification for all telegram-linked users.
		brief, briefErr := briefStore.GetLatest(jobCtx)
		if briefErr == nil && brief != nil {
			tUsers, _ := telegramUserStore.ListAll(jobCtx)
			for _, tu := range tUsers {
				_ = notificationStore.CreateDigest(jobCtx, tu.UserID, brief.Summary)
			}
		}
	})
	if err != nil {
		slog.Error("worker: add daily brief cron", "err", err)
		os.Exit(1)
	}

	// Watchlist scan: 4 times/day (1am, 7am, 1pm, 7pm).
	_, err = c.AddFunc("0 1,7,13,19 * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 2*time.Hour)
		defer jobCancel()

		slog.Info("cron: watchlist scan triggered")
		agents.RunWatchlistScan(jobCtx, agents.Deps{
			Orgs:     watchlistOrgStore,
			Hits:     watchlistHitStore,
			Articles: articleStore,
			AI:       aiClient,
		})
	})
	if err != nil {
		slog.Error("worker: add watchlist scan cron", "err", err)
		os.Exit(1)
	}

	// Research: every 2 minutes, pick up queued deep research projects.
	_, err = c.AddFunc("*/2 * * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 2*time.Hour)
		defer jobCancel()

		queued, qErr := researchProjectStore.ListQueued(jobCtx)
		if qErr != nil || len(queued) == 0 {
			return
		}

		slog.Info("cron: research project picked up", "id", queued[0].ID, "topic", queued[0].Topic)
		research.RunProject(jobCtx, research.Deps{
			Projects:     researchProjectStore,
			Findings:     researchFindingStore,
			Articles:     articleStore,
			CrawledPages: crawledPageStore,
			AI:           aiClient,
		}, queued[0].ID)
	})
	if err != nil {
		slog.Error("worker: add research cron", "err", err)
		os.Exit(1)
	}

	// Escritos: every 2 minutes, pick up queued SEO article generation.
	_, err = c.AddFunc("*/2 * * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 30*time.Minute)
		defer jobCancel()

		queued, qErr := escritoStore.ListQueued(jobCtx)
		if qErr != nil || len(queued) == 0 {
			return
		}

		slog.Info("cron: escrito picked up", "id", queued[0].ID, "topic", queued[0].Topic)
		generator.RunGeneration(jobCtx, generator.Deps{
			Escritos: escritoStore,
			Sources:  escritoSourceStore,
			Articles: articleStore,
			AI:       aiClient,
		}, queued[0].ID, nil)
	})
	if err != nil {
		slog.Error("worker: add escritos cron", "err", err)
		os.Exit(1)
	}

	// Crawler: every 2 hours — fetch from queue, extract links, detect changes.
	crawlerDeps := crawler.Deps{
		Domains:  crawlDomainStore,
		Queue:    crawlQueueStore,
		Pages:    crawledPageStore,
		Links:    crawlLinkStore,
		Runs:     crawlRunStore,
		Entities: entityStore,
		PageEnts: pageEntityStore,
		Rels:     entityRelStore,
		AI:       aiClient,
	}

	_, err = c.AddFunc("0 */2 * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 2*time.Hour)
		defer jobCancel()

		slog.Info("cron: crawler job triggered")
		crawler.RunCrawl(jobCtx, crawlerDeps, 500)
	})
	if err != nil {
		slog.Error("worker: add crawler cron", "err", err)
		os.Exit(1)
	}

	// Recrawl: daily at 2am — re-enqueue pages past next_crawl_at.
	_, err = c.AddFunc("0 2 * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 30*time.Minute)
		defer jobCancel()

		slog.Info("cron: recrawl job triggered")
		crawler.RunRecrawl(jobCtx, crawlerDeps)
	})
	if err != nil {
		slog.Error("worker: add recrawl cron", "err", err)
		os.Exit(1)
	}

	// Crawler enrichment: every 30 minutes — AI enrichment for un-enriched pages.
	_, err = c.AddFunc("*/30 * * * *", func() {
		wg.Add(1)
		defer wg.Done()

		jobCtx, jobCancel := context.WithTimeout(ctx, 1*time.Hour)
		defer jobCancel()

		slog.Info("cron: crawler enrichment job triggered")
		crawler.RunEnrichment(jobCtx, crawlerDeps, 50)
	})
	if err != nil {
		slog.Error("worker: add crawler enrichment cron", "err", err)
		os.Exit(1)
	}

	// Start the cron scheduler.
	c.Start()
	slog.Info("worker: cron scheduler started",
		"jobs", len(c.Entries()),
	)

	// Run an initial ingestion on startup so we don't wait 4 hours for the
	// first run.
	wg.Add(1)
	go func() {
		defer wg.Done()

		// Small delay to let everything settle.
		select {
		case <-time.After(5 * time.Second):
		case <-ctx.Done():
			return
		}

		jobCtx, jobCancel := context.WithTimeout(ctx, 3*time.Hour)
		defer jobCancel()

		slog.Info("worker: running initial ingestion on startup")
		scraper.RunIngestion(jobCtx, stores, sc, aiClient, storageClient)
	}()

	// ── Graceful Shutdown ──────────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	sig := <-sigCh
	slog.Info("worker: received shutdown signal", "signal", sig.String())

	// Stop accepting new cron jobs.
	slog.Info("worker: stopping cron scheduler")
	cronCtx := c.Stop()

	// Cancel the root context to signal all in-flight jobs to stop.
	cancel()

	// Wait for the cron scheduler to finish its currently running jobs.
	select {
	case <-cronCtx.Done():
		slog.Info("worker: cron scheduler stopped")
	case <-time.After(30 * time.Second):
		slog.Warn("worker: cron scheduler stop timed out")
	}

	// Wait for all in-flight goroutines.
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("worker: all in-flight jobs complete")
	case <-time.After(60 * time.Second):
		slog.Warn("worker: timed out waiting for in-flight jobs")
	}

	// Close the database pool.
	pool.Close()
	slog.Info("worker: shutdown complete")
}
