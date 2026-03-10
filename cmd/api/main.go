// Command api starts the Folio HTTP API server.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/config"
	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/db"
	"github.com/Saul-Punybz/folio/internal/handlers"
	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
	"github.com/Saul-Punybz/folio/internal/storage"
)

func main() {
	// Structured logging.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg := config.Load()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Database connection.
	pool, err := db.Connect(ctx, cfg.DB)
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Data stores.
	articleStore := models.NewArticleStore(pool)
	userStore := models.NewUserStore(pool)
	sessionStore := models.NewSessionStore(pool)
	sourceStore := models.NewSourceStore(pool)
	noteStore := models.NewNoteStore(pool)
	briefStore := models.NewBriefStore(pool)
	watchlistOrgStore := models.NewWatchlistOrgStore(pool)
	watchlistHitStore := models.NewWatchlistHitStore(pool)
	fingerprintStore := models.NewFingerprintStore(pool)
	chatSessionStore := models.NewChatSessionStore(pool)
	researchProjectStore := models.NewResearchProjectStore(pool)
	researchFindingStore := models.NewResearchFindingStore(pool)
	entityStore := models.NewEntityStore(pool)

	// Crawler stores.
	crawlDomainStore := models.NewCrawlDomainStore(pool)
	crawlQueueStore := models.NewCrawlQueueStore(pool)
	crawledPageStore := models.NewCrawledPageStore(pool)
	crawlLinkStore := models.NewCrawlLinkStore(pool)
	crawlRunStore := models.NewCrawlRunStore(pool)
	pageEntityStore := models.NewPageEntityStore(pool)
	entityRelStore := models.NewEntityRelationshipStore(pool)

	// S3 storage client (for export handler).
	storageClient, storageErr := storage.NewClient(ctx, cfg.S3)
	if storageErr != nil {
		slog.Warn("S3 storage not available for export", "err", storageErr)
		storageClient = nil
	}

	// Handlers.
	authHandler := &handlers.AuthHandler{
		Users:    userStore,
		Sessions: sessionStore,
	}
	itemsHandler := &handlers.ItemsHandler{
		Articles: articleStore,
		Scraper:  scraper.NewScraper(),
		AI:       ai.NewClient(cfg.Ollama.Host, cfg.Ollama.InstructModel, cfg.Ollama.EmbedModel),
	}
	searchHandler := &handlers.SearchHandler{
		Articles: articleStore,
	}
	sourcesHandler := &handlers.SourcesHandler{
		Sources: sourceStore,
		Scraper: scraper.NewScraper(),
		AI:      ai.NewClient(cfg.Ollama.Host, cfg.Ollama.InstructModel, cfg.Ollama.EmbedModel),
	}
	notesHandler := &handlers.NotesHandler{
		Notes:    noteStore,
		Articles: articleStore,
	}
	// AI client for manual brief generation.
	aiClient := ai.NewClient(
		cfg.Ollama.Host,
		cfg.Ollama.InstructModel,
		cfg.Ollama.EmbedModel,
	)

	briefHandler := &handlers.BriefHandler{
		Briefs:   briefStore,
		Articles: articleStore,
		AI:       aiClient,
	}
	watchlistHandler := &handlers.WatchlistHandler{
		Orgs:     watchlistOrgStore,
		Hits:     watchlistHitStore,
		Articles: articleStore,
		AI:       aiClient,
	}
	exportHandler := &handlers.ExportHandler{
		Articles: articleStore,
		Notes:    noteStore,
		Storage:  storageClient,
	}
	chatHandler := &handlers.ChatHandler{
		Sessions: chatSessionStore,
	}
	feedHandler := &handlers.FeedHandler{
		Users: userStore,
		Hits:  watchlistHitStore,
	}

	researchHandler := &handlers.ResearchHandler{
		Projects: researchProjectStore,
		Findings: researchFindingStore,
		Articles: articleStore,
		AI:       aiClient,
	}

	analyticsHandler := &handlers.AnalyticsHandler{
		Pool: pool,
	}

	sc := scraper.NewScraper()
	adminHandler := &handlers.AdminHandler{
		Articles:     articleStore,
		Sources:      sourceStore,
		Fingerprints: fingerprintStore,
		AI:           aiClient,
		Scraper:      sc,
		Storage:      storageClient,
	}

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
	// Escritos stores.
	escritoStore := models.NewEscritoStore(pool)
	escritoSourceStore := models.NewEscritoSourceStore(pool)

	escritosHandler := &handlers.EscritosHandler{
		Escritos: escritoStore,
		Sources:  escritoSourceStore,
		Articles: articleStore,
		AI:       aiClient,
	}

	crawlerHandler := &handlers.CrawlerHandler{
		Domains:   crawlDomainStore,
		Queue:     crawlQueueStore,
		Pages:     crawledPageStore,
		Links:     crawlLinkStore,
		Runs:      crawlRunStore,
		Entities:  entityStore,
		PageEnts:  pageEntityStore,
		Rels:      entityRelStore,
		CrawlDeps: crawlerDeps,
	}

	// Router.
	r := chi.NewRouter()

	// Global middleware.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(middleware.MaxBodySize(10 << 20)) // 10 MB max body
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "https://localhost:*", "http://127.0.0.1:*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Login rate limiter: 10 attempts per 15 minutes per IP.
	loginLimiter := middleware.NewRateLimiter(10, 15*time.Minute)

	// Public routes.
	r.Get("/api/health", handlers.Health)
	r.With(middleware.RateLimit(loginLimiter)).Post("/api/login", authHandler.Login)
	r.Get("/feed/{token}.xml", feedHandler.ServeFeed)

	// Authenticated routes.
	r.Group(func(r chi.Router) {
		r.Use(middleware.SessionAuth(sessionStore, userStore))

		r.Post("/api/logout", authHandler.Logout)
		r.Get("/api/me", authHandler.Me)

		// Items (articles).
		r.Get("/api/items", itemsHandler.ListItems)
		r.Post("/api/items/{id}/save", itemsHandler.SaveItem)
		r.Post("/api/items/{id}/trash", itemsHandler.TrashItem)
		r.Post("/api/items/{id}/pin", itemsHandler.PinItem)
		r.Post("/api/items/{id}/undo", itemsHandler.UndoItem)
		r.Post("/api/collect", itemsHandler.CollectItem)

		// Search.
		r.Get("/api/search", searchHandler.Search)
		r.Get("/api/items/{id}/similar", searchHandler.Similar)

		// Notes.
		r.Get("/api/items/{id}/notes", notesHandler.ListNotes)
		r.Post("/api/items/{id}/notes", notesHandler.CreateNote)
		r.Delete("/api/notes/{noteId}", notesHandler.DeleteNote)

		// Briefs.
		r.Get("/api/briefs/latest", briefHandler.GetLatestBrief)
		r.Get("/api/briefs", briefHandler.ListBriefs)
		r.Post("/api/briefs/generate", briefHandler.GenerateBrief)

		// Watchlist.
		r.Route("/api/watchlist", func(r chi.Router) {
			r.Get("/orgs", watchlistHandler.ListOrgs)
			r.Post("/orgs", watchlistHandler.CreateOrg)
			r.Put("/orgs/{id}", watchlistHandler.UpdateOrg)
			r.Delete("/orgs/{id}", watchlistHandler.DeleteOrg)
			r.Patch("/orgs/{id}/toggle", watchlistHandler.ToggleOrg)

			r.Get("/hits", watchlistHandler.ListHits)
			r.Get("/hits/unseen", watchlistHandler.CountUnseen)
			r.Post("/hits/{id}/seen", watchlistHandler.MarkSeen)
			r.Post("/hits/seen-all", watchlistHandler.MarkAllSeen)
			r.Delete("/hits/{id}", watchlistHandler.DeleteHit)

			r.Post("/scan", watchlistHandler.TriggerScan)
			r.Post("/orgs/{id}/enrich", watchlistHandler.EnrichOrg)

			r.Get("/feed-url", feedHandler.GetFeedURL)
			r.Post("/feed-url/regenerate", feedHandler.RegenerateFeedURL)
		})

		// Chat sessions.
		r.Get("/api/chat/sessions", chatHandler.ListSessions)
		r.Get("/api/chat/sessions/{id}", chatHandler.GetSession)
		r.Post("/api/chat/sessions", chatHandler.CreateSession)
		r.Put("/api/chat/sessions/{id}", chatHandler.UpdateSession)
		r.Delete("/api/chat/sessions/{id}", chatHandler.DeleteSession)

		// Research (deep investigation).
		r.Route("/api/research", func(r chi.Router) {
			r.Post("/", researchHandler.CreateProject)
			r.Get("/", researchHandler.ListProjects)
			r.Get("/{id}", researchHandler.GetProject)
			r.Get("/{id}/findings", researchHandler.GetFindings)
			r.Post("/{id}/stop", researchHandler.StopProject)
			r.Post("/{id}/run", researchHandler.TriggerResearch)
			r.Delete("/{id}", researchHandler.DeleteProject)
		})

		// Escritos (SEO article generator).
		r.Route("/api/escritos", func(r chi.Router) {
			r.Post("/", escritosHandler.CreateEscrito)
			r.Get("/", escritosHandler.ListEscritos)
			r.Get("/{id}", escritosHandler.GetEscrito)
			r.Put("/{id}", escritosHandler.UpdateEscrito)
			r.Post("/{id}/regenerate", escritosHandler.RegenerateEscrito)
			r.Post("/{id}/improve", escritosHandler.ImproveEscrito)
			r.Post("/{id}/seo-score", escritosHandler.RecalcSEO)
			r.Delete("/{id}", escritosHandler.DeleteEscrito)
			r.Post("/{id}/export", escritosHandler.ExportEscrito)
		})

		// Crawler.
		r.Route("/api/crawler", func(r chi.Router) {
			r.Get("/stats", crawlerHandler.Stats)
			r.Get("/runs", crawlerHandler.ListRuns)
			r.Get("/queue", crawlerHandler.QueueStats)
			r.Get("/pages", crawlerHandler.ListPages)
			r.Get("/pages/search", crawlerHandler.SearchPages)
			r.Get("/pages/changes", crawlerHandler.ListChangedPages)
			r.Get("/pages/{id}", crawlerHandler.GetPage)
			r.Get("/graph", crawlerHandler.GetGraph)
			r.Get("/graph/{entityId}/relations", crawlerHandler.GetEntityRelations)
			r.Get("/domains", crawlerHandler.ListDomains)

			// Admin-only crawler operations
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Post("/domains", crawlerHandler.CreateDomain)
				r.Put("/domains/{id}", crawlerHandler.UpdateDomain)
				r.Patch("/domains/{id}/toggle", crawlerHandler.ToggleDomain)
				r.Delete("/domains/{id}", crawlerHandler.DeleteDomain)
				r.Post("/trigger", crawlerHandler.TriggerCrawl)
			})
		})

		// Analytics (authenticated, not admin-only)
		r.Get("/api/analytics/tags", analyticsHandler.TagTrends)
		r.Get("/api/analytics/entities", analyticsHandler.TopEntities)
		r.Get("/api/analytics/co-occurrences", analyticsHandler.CoOccurrences)
		r.Get("/api/analytics/sentiment", analyticsHandler.SentimentDistribution)
		r.Get("/api/analytics/sources", analyticsHandler.SourceHealth)
		r.Get("/api/analytics/volume", analyticsHandler.ArticleVolume)

		// Export.
		r.Get("/api/items/{id}/export", exportHandler.ExportArticle)
		r.Post("/api/export", exportHandler.ExportBulk)

		// Retention.
		r.Put("/api/items/{id}/retention", itemsHandler.UpdateRetention)

		// Sources (admin only).
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin)
			r.Get("/api/sources", sourcesHandler.ListSources)
			r.Post("/api/sources", sourcesHandler.CreateSource)
			r.Post("/api/sources/quick", sourcesHandler.QuickCreateSource)
			r.Put("/api/sources/{id}", sourcesHandler.UpdateSource)
			r.Patch("/api/sources/{id}/toggle", sourcesHandler.ToggleSource)
			r.Delete("/api/sources/{id}", sourcesHandler.DeleteSource)
			r.Post("/api/sources/{id}/test", sourcesHandler.TestScrape)
		})

		// Admin actions.
		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin)
			r.Post("/api/admin/reenrich", adminHandler.Reenrich)
			r.Post("/api/admin/ingest", adminHandler.TriggerIngest)
			r.Post("/api/admin/chat", adminHandler.ChatWithNews)
		})
	})

	// Serve static frontend files if the directory exists.
	frontendDir := "./frontend/dist"
	if _, err := os.Stat(frontendDir); err == nil {
		fileServer := http.FileServer(http.Dir(frontendDir))
		r.Get("/*", func(w http.ResponseWriter, r *http.Request) {
			// If the path starts with /api, let Chi handle it (404).
			if strings.HasPrefix(r.URL.Path, "/api") {
				http.NotFound(w, r)
				return
			}
			// Try to serve the file; fall back to index.html for SPA routing.
			path := frontendDir + r.URL.Path
			if _, err := os.Stat(path); os.IsNotExist(err) {
				http.ServeFile(w, r, frontendDir+"/index.html")
				return
			}
			fileServer.ServeHTTP(w, r)
		})
	}

	// Start server.
	addr := cfg.Server.Addr()
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown.
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		slog.Info("server starting", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-done
	slog.Info("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}

	slog.Info("server stopped")
}
