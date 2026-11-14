// Command app is the self-contained macOS Folio application.
// It starts an embedded PostgreSQL, runs migrations, serves the API + frontend
// in a native Wails window, and runs background worker cron jobs — all in one process.
package main

import (
	"context"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/robfig/cron/v3"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	folio "github.com/Saul-Punybz/folio"
	"github.com/Saul-Punybz/folio/internal/agents"
	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/config"
	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/db"
	"github.com/Saul-Punybz/folio/internal/embedded"
	"github.com/Saul-Punybz/folio/internal/generator"
	"github.com/Saul-Punybz/folio/internal/handlers"
	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/research"
	"github.com/Saul-Punybz/folio/internal/scraper"
	"github.com/Saul-Punybz/folio/internal/storage"
)

// splashHTML is the branded loading screen shown when the app starts.
const splashHTML = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Folio</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{background:#0a0a0f;color:#fff;font-family:system-ui,-apple-system,BlinkMacSystemFont,sans-serif;
  display:flex;align-items:center;justify-content:center;height:100vh;overflow:hidden;
  background:radial-gradient(ellipse at 50% 40%,#1a1530 0%,#0a0a0f 70%)}
.splash{text-align:center;animation:fadeIn .6s ease}
.logo{width:80px;height:80px;margin:0 auto 24px;position:relative}
.logo svg{width:80px;height:80px;filter:drop-shadow(0 0 30px rgba(99,102,241,.4))}
.logo::after{content:'';position:absolute;inset:-10px;border-radius:20px;
  background:radial-gradient(circle,rgba(99,102,241,.15) 0%,transparent 70%);
  animation:pulse 2s ease-in-out infinite}
h1{font-size:32px;font-weight:700;letter-spacing:-.5px;margin-bottom:8px;
  background:linear-gradient(135deg,#c4b5fd 0%,#818cf8 50%,#6366f1 100%);
  -webkit-background-clip:text;-webkit-text-fill-color:transparent}
.subtitle{font-size:13px;color:#71717a;letter-spacing:1.5px;text-transform:uppercase;margin-bottom:40px}
.loader{display:flex;gap:6px;justify-content:center;margin-bottom:32px}
.loader span{width:6px;height:6px;border-radius:50%;background:#6366f1;
  animation:bounce .8s ease-in-out infinite}
.loader span:nth-child(2){animation-delay:.15s}
.loader span:nth-child(3){animation-delay:.3s}
.status{font-size:12px;color:#52525b;transition:opacity .3s}
@keyframes fadeIn{from{opacity:0;transform:translateY(10px)}to{opacity:1;transform:none}}
@keyframes pulse{0%,100%{opacity:.5;transform:scale(1)}50%{opacity:1;transform:scale(1.05)}}
@keyframes bounce{0%,100%{transform:translateY(0);opacity:.4}50%{transform:translateY(-8px);opacity:1}}
@keyframes fadeOut{to{opacity:0;transform:scale(1.02)}}
</style>
</head>
<body>
<div class="splash" id="splash">
  <div class="logo">
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">
      <rect width="32" height="32" rx="6" fill="#6366f1"/>
      <text x="16" y="22" font-family="system-ui,sans-serif" font-size="18" font-weight="700" fill="white" text-anchor="middle">F</text>
    </svg>
  </div>
  <h1>Folio</h1>
  <p class="subtitle">Political Intelligence</p>
  <div class="loader"><span></span><span></span><span></span></div>
  <p class="status" id="status">Connecting...</p>
</div>
<script>
(function(){
  var status=document.getElementById('status');
  var msgs=['Connecting...','Loading sources...','Preparing inbox...'];
  var i=0;
  var iv=setInterval(function(){i++;if(i<msgs.length)status.textContent=msgs[i]},600);
  function go(){
    clearInterval(iv);
    status.textContent='Ready';
    document.getElementById('splash').style.animation='fadeOut .4s ease forwards';
    setTimeout(function(){window.location.replace('/inbox')},400);
  }
  // Check API health then navigate
  function check(){
    fetch('/api/health').then(function(r){if(r.ok)go();else setTimeout(check,300)}).catch(function(){setTimeout(check,300)});
  }
  setTimeout(check,400);
})();
</script>
</body>
</html>`

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	slog.Info("Folio starting...", "version", "1.0.0")

	// ── Data Directory ───────────────────────────────────────────
	dataDir := embedded.DataDirectory()
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		slog.Error("failed to create data directory", "path", dataDir, "err", err)
		os.Exit(1)
	}
	slog.Info("data directory", "path", dataDir)

	// ── Find PostgreSQL ──────────────────────────────────────────
	pgBinDir, pgLibDir, err := embedded.FindPGBinDir()
	if err != nil {
		slog.Error("PostgreSQL not found", "err", err)
		fmt.Fprintf(os.Stderr, "\n  PostgreSQL is required. Install with:\n    brew install postgresql@17\n\n")
		os.Exit(1)
	}
	slog.Info("found PostgreSQL", "binDir", pgBinDir)

	// ── Start Embedded PostgreSQL ────────────────────────────────
	pgPort, err := embedded.FreePort()
	if err != nil {
		slog.Error("failed to find free port", "err", err)
		os.Exit(1)
	}

	pg := &embedded.Postgres{
		BinDir:  pgBinDir,
		LibDir:  pgLibDir,
		DataDir: filepath.Join(dataDir, "pgdata"),
		LogFile: filepath.Join(dataDir, "postgresql.log"),
		Port:    pgPort,
		DBName:  "folio",
		User:    "folio",
	}

	if err := pg.Start(); err != nil {
		slog.Error("failed to start PostgreSQL", "err", err)
		fmt.Fprintf(os.Stderr, "\n  Check %s for details.\n\n", pg.LogFile)
		os.Exit(1)
	}
	defer pg.Stop()

	// ── Connect to DB with Embedded Migrations ───────────────────
	migrationsFS, err := fs.Sub(folio.Migrations, "migrations")
	if err != nil {
		slog.Error("failed to access embedded migrations", "err", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	pool, err := db.ConnectWithDSN(ctx, pg.DSN(), migrationsFS)
	cancel()
	if err != nil {
		slog.Error("failed to connect to database", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	// ── Load Config (for Ollama, S3, Telegram) ───────────────────
	// Override DB config to point at our embedded PG
	os.Setenv("DB_HOST", "localhost")
	os.Setenv("DB_PORT", fmt.Sprintf("%d", pgPort))
	os.Setenv("DB_USER", "folio")
	os.Setenv("DB_PASS", "")
	os.Setenv("DB_NAME", "folio")
	os.Setenv("DB_SSLMODE", "disable")
	cfg := config.Load()

	// ── Check AI Provider ─────────────────────────────────────────
	if cfg.AI.Provider == "openai" {
		slog.Info("AI provider: OpenAI-compatible API",
			"host", cfg.AI.Host,
			"model", cfg.AI.InstructModel)
	} else {
		ollamaAvailable := checkOllama(cfg.AI.Host)
		if !ollamaAvailable {
			slog.Warn("Ollama not available — AI features disabled",
				"host", cfg.AI.Host,
				"fix", "Install Ollama: brew install ollama && ollama serve")
		}
	}

	// ── Create Stores ────────────────────────────────────────────
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
	crawlDomainStore := models.NewCrawlDomainStore(pool)
	crawlQueueStore := models.NewCrawlQueueStore(pool)
	crawledPageStore := models.NewCrawledPageStore(pool)
	crawlLinkStore := models.NewCrawlLinkStore(pool)
	crawlRunStore := models.NewCrawlRunStore(pool)
	pageEntityStore := models.NewPageEntityStore(pool)
	entityRelStore := models.NewEntityRelationshipStore(pool)
	escritoStore := models.NewEscritoStore(pool)
	escritoSourceStore := models.NewEscritoSourceStore(pool)
	opportunityStore := models.NewOpportunityStore(pool)

	// S3 storage (optional).
	storageCtx, storageCancel := context.WithTimeout(context.Background(), 5*time.Second)
	storageClient, storageErr := storage.NewClient(storageCtx, cfg.S3)
	storageCancel()
	if storageErr != nil {
		slog.Warn("S3 storage not available", "err", storageErr)
		storageClient = nil
	}

	// AI client — supports both Ollama (local) and OpenAI-compatible APIs (cloud).
	aiClient := ai.NewFromConfig(cfg.AI.Provider, cfg.AI.Host, cfg.AI.APIKey, cfg.AI.InstructModel, cfg.AI.EmbedModel)

	// ── Setup Router ─────────────────────────────────────────────
	r := setupRouter(
		cfg, aiClient, storageClient,
		articleStore, userStore, sessionStore, sourceStore, noteStore,
		briefStore, watchlistOrgStore, watchlistHitStore, fingerprintStore,
		chatSessionStore, researchProjectStore, researchFindingStore,
		entityStore, crawlDomainStore, crawlQueueStore, crawledPageStore,
		crawlLinkStore, crawlRunStore, pageEntityStore, entityRelStore,
		escritoStore, escritoSourceStore, opportunityStore, pool,
	)

	// ── Start Worker Cron Jobs (inline) ──────────────────────────
	var wg sync.WaitGroup
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()

	cronScheduler := startWorkerCron(workerCtx, &wg, cfg, aiClient, storageClient,
		articleStore, sourceStore, fingerprintStore, sessionStore,
		briefStore, watchlistOrgStore, watchlistHitStore, entityStore,
		researchProjectStore, researchFindingStore, crawlDomainStore,
		crawlQueueStore, crawledPageStore, crawlLinkStore, crawlRunStore,
		pageEntityStore, entityRelStore, escritoStore, escritoSourceStore,
		opportunityStore,
	)

	// ── Embedded Frontend Assets ─────────────────────────────────
	frontendFS, _ := fs.Sub(folio.FrontendDist, "frontend/dist")
	fileServer := http.FileServer(http.FS(frontendFS))

	// ── Launch Wails Native Window ───────────────────────────────
	slog.Info("Folio is ready — opening native window")

	if err := wails.Run(&options.App{
		Title:     "Folio",
		Width:     1400,
		Height:    900,
		MinWidth:  1024,
		MinHeight: 700,
		AssetServer: &assetserver.Options{
			// Handler serves all requests: API routes go to Chi, frontend
			// routes resolve Astro's directory/index.html convention.
			Handler: http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				path := req.URL.Path

				// API and feed routes → Chi router
				if strings.HasPrefix(path, "/api/") ||
					strings.HasPrefix(path, "/api") ||
					strings.HasPrefix(path, "/feed/") {
					r.ServeHTTP(w, req)
					return
				}

				// Normalize path
				trimmed := strings.TrimPrefix(path, "/")
				trimmed = strings.TrimSuffix(trimmed, "/")

				// Root → serve branded splash screen
				if trimmed == "" {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					w.Write([]byte(splashHTML))
					return
				}

				// Try exact file (static assets: /_astro/foo.js, /favicon.svg)
				if f, err := frontendFS.Open(trimmed); err == nil {
					info, statErr := f.Stat()
					f.Close()
					if statErr == nil && !info.IsDir() {
						fileServer.ServeHTTP(w, req)
						return
					}
				}

				// Try directory index (Astro pages: /inbox → inbox/index.html)
				indexPath := trimmed + "/index.html"
				if f, err := frontendFS.Open(indexPath); err == nil {
					w.Header().Set("Content-Type", "text/html; charset=utf-8")
					io.Copy(w, f)
					f.Close()
					return
				}

				// Fallback: serve root index.html
				req.URL.Path = "/index.html"
				fileServer.ServeHTTP(w, req)
			}),
		},
		OnShutdown: func(ctx context.Context) {
			slog.Info("shutting down Folio...")

			// Stop cron
			cronCtx := cronScheduler.Stop()
			workerCancel()

			// Wait for cron + goroutines
			select {
			case <-cronCtx.Done():
			case <-time.After(15 * time.Second):
			}
			wg.Wait()

			pool.Close()
			pg.Stop()

			slog.Info("Folio stopped. Goodbye!")
		},
		Mac: &mac.Options{
			TitleBar: mac.TitleBarDefault(),
			About: &mac.AboutInfo{
				Title:   "Folio",
				Message: "Political Intelligence & Evidence Archive\nVersion 1.0.0\n\nCopyright © 2026 Puny.bz",
			},
		},
	}); err != nil {
		slog.Error("wails app error", "err", err)
		os.Exit(1)
	}
}

// setupRouter creates the Chi router with all API routes.
func setupRouter(
	cfg config.Config,
	aiClient *ai.OllamaClient,
	storageClient *storage.Client,
	articleStore *models.ArticleStore,
	userStore *models.UserStore,
	sessionStore *models.SessionStore,
	sourceStore *models.SourceStore,
	noteStore *models.NoteStore,
	briefStore *models.BriefStore,
	watchlistOrgStore *models.WatchlistOrgStore,
	watchlistHitStore *models.WatchlistHitStore,
	fingerprintStore *models.FingerprintStore,
	chatSessionStore *models.ChatSessionStore,
	researchProjectStore *models.ResearchProjectStore,
	researchFindingStore *models.ResearchFindingStore,
	entityStore *models.EntityStore,
	crawlDomainStore *models.CrawlDomainStore,
	crawlQueueStore *models.CrawlQueueStore,
	crawledPageStore *models.CrawledPageStore,
	crawlLinkStore *models.CrawlLinkStore,
	crawlRunStore *models.CrawlRunStore,
	pageEntityStore *models.PageEntityStore,
	entityRelStore *models.EntityRelationshipStore,
	escritoStore *models.EscritoStore,
	escritoSourceStore *models.EscritoSourceStore,
	opportunityStore *models.OpportunityStore,
	pool *pgxpool.Pool,
) *chi.Mux {
	sc := scraper.NewScraper()

	authHandler := &handlers.AuthHandler{Users: userStore, Sessions: sessionStore}
	itemsHandler := &handlers.ItemsHandler{
		Articles: articleStore,
		Scraper:  sc,
		AI:       aiClient,
	}
	searchHandler := &handlers.SearchHandler{Articles: articleStore}
	sourcesHandler := &handlers.SourcesHandler{Sources: sourceStore, Scraper: sc, AI: aiClient}
	notesHandler := &handlers.NotesHandler{Notes: noteStore, Articles: articleStore}
	briefHandler := &handlers.BriefHandler{Briefs: briefStore, Articles: articleStore, AI: aiClient}
	watchlistHandler := &handlers.WatchlistHandler{
		Orgs: watchlistOrgStore, Hits: watchlistHitStore,
		Articles: articleStore, AI: aiClient,
	}
	exportHandler := &handlers.ExportHandler{Articles: articleStore, Notes: noteStore, Storage: storageClient}
	chatHandler := &handlers.ChatHandler{Sessions: chatSessionStore}
	feedHandler := &handlers.FeedHandler{Users: userStore, Hits: watchlistHitStore}
	researchHandler := &handlers.ResearchHandler{
		Projects: researchProjectStore, Findings: researchFindingStore,
		Articles: articleStore, AI: aiClient,
	}

	crawlerDeps := crawler.Deps{
		Domains: crawlDomainStore, Queue: crawlQueueStore, Pages: crawledPageStore,
		Links: crawlLinkStore, Runs: crawlRunStore, Entities: entityStore,
		PageEnts: pageEntityStore, Rels: entityRelStore,
		Opportunities: opportunityStore, AI: aiClient,
	}
	crawlerHandler := &handlers.CrawlerHandler{
		Domains: crawlDomainStore, Queue: crawlQueueStore, Pages: crawledPageStore,
		Links: crawlLinkStore, Runs: crawlRunStore, Entities: entityStore,
		PageEnts: pageEntityStore, Rels: entityRelStore, CrawlDeps: crawlerDeps,
	}
	opportunityHandler := &handlers.OpportunityHandler{
		Opportunities: opportunityStore, CrawlDeps: crawlerDeps,
	}
	escritosHandler := &handlers.EscritosHandler{
		Escritos: escritoStore, Sources: escritoSourceStore,
		Articles: articleStore, AI: aiClient,
	}
	adminHandler := &handlers.AdminHandler{
		Articles: articleStore, Sources: sourceStore, Fingerprints: fingerprintStore,
		Entities: entityStore, AI: aiClient, Scraper: sc, Storage: storageClient,
	}

	r := chi.NewRouter()
	r.Use(chimw.RequestID, chimw.RealIP, chimw.Logger, chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(middleware.MaxBodySize(10 << 20)) // 10 MB max body
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:*", "https://localhost:*", "http://127.0.0.1:*", "wails://wails"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Public routes.
	r.Get("/api/health", handlers.Health)
	r.Get("/feed/{token}.xml", feedHandler.ServeFeed)

	// All routes auto-authenticated (local macOS app, no login needed).
	r.Group(func(r chi.Router) {
		r.Use(middleware.AutoAuth(userStore))

		r.Get("/api/me", authHandler.Me)

		r.Get("/api/items", itemsHandler.ListItems)
		r.Post("/api/items/{id}/save", itemsHandler.SaveItem)
		r.Post("/api/items/{id}/trash", itemsHandler.TrashItem)
		r.Post("/api/items/{id}/pin", itemsHandler.PinItem)
		r.Post("/api/items/{id}/undo", itemsHandler.UndoItem)
		r.Post("/api/collect", itemsHandler.CollectItem)

		r.Get("/api/search", searchHandler.Search)
		r.Get("/api/items/{id}/similar", searchHandler.Similar)

		r.Get("/api/items/{id}/notes", notesHandler.ListNotes)
		r.Post("/api/items/{id}/notes", notesHandler.CreateNote)
		r.Delete("/api/notes/{noteId}", notesHandler.DeleteNote)

		r.Get("/api/briefs/latest", briefHandler.GetLatestBrief)
		r.Get("/api/briefs", briefHandler.ListBriefs)
		r.Post("/api/briefs/generate", briefHandler.GenerateBrief)

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

		r.Get("/api/chat/sessions", chatHandler.ListSessions)
		r.Get("/api/chat/sessions/{id}", chatHandler.GetSession)
		r.Post("/api/chat/sessions", chatHandler.CreateSession)
		r.Put("/api/chat/sessions/{id}", chatHandler.UpdateSession)
		r.Delete("/api/chat/sessions/{id}", chatHandler.DeleteSession)

		r.Route("/api/research", func(r chi.Router) {
			r.Post("/", researchHandler.CreateProject)
			r.Get("/", researchHandler.ListProjects)
			r.Get("/{id}", researchHandler.GetProject)
			r.Get("/{id}/findings", researchHandler.GetFindings)
			r.Post("/{id}/stop", researchHandler.StopProject)
			r.Post("/{id}/run", researchHandler.TriggerResearch)
			r.Delete("/{id}", researchHandler.DeleteProject)
		})

		r.Route("/api/escritos", func(r chi.Router) {
			r.Post("/", escritosHandler.CreateEscrito)
			r.Get("/", escritosHandler.ListEscritos)
			r.Get("/{id}", escritosHandler.GetEscrito)
			r.Put("/{id}", escritosHandler.UpdateEscrito)
			r.Post("/{id}/regenerate", escritosHandler.RegenerateEscrito)
			r.Post("/{id}/improve", escritosHandler.ImproveEscrito)
			r.Post("/{id}/expand", escritosHandler.ExpandEscrito)
			r.Post("/{id}/seo-score", escritosHandler.RecalcSEO)
			r.Post("/{id}/seo-optimize", escritosHandler.OptimizeSEO)
			r.Delete("/{id}", escritosHandler.DeleteEscrito)
			r.Post("/{id}/export", escritosHandler.ExportEscrito)
		})

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
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Post("/domains", crawlerHandler.CreateDomain)
				r.Put("/domains/{id}", crawlerHandler.UpdateDomain)
				r.Patch("/domains/{id}/toggle", crawlerHandler.ToggleDomain)
				r.Delete("/domains/{id}", crawlerHandler.DeleteDomain)
				r.Post("/trigger", crawlerHandler.TriggerCrawl)
			})
		})

		r.Route("/api/opportunities", func(r chi.Router) {
			r.Get("/", opportunityHandler.List)
			r.Get("/stats", opportunityHandler.Stats)
			r.Get("/search", opportunityHandler.Search)
			r.Get("/{id}", opportunityHandler.Get)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Put("/{id}", opportunityHandler.Update)
				r.Delete("/{id}", opportunityHandler.Delete)
				r.Post("/{id}/review", opportunityHandler.Review)
				r.Post("/refresh-statuses", opportunityHandler.RefreshStatuses)
				r.Post("/extract", opportunityHandler.TriggerExtraction)
			})
		})

		// Open URL in system browser (macOS)
		r.Get("/api/open-url", handlers.OpenURL)

		analyticsHandler := &handlers.AnalyticsHandler{Pool: pool}
		r.Get("/api/analytics/tags", analyticsHandler.TagTrends)
		r.Get("/api/analytics/entities", analyticsHandler.TopEntities)
		r.Get("/api/analytics/co-occurrences", analyticsHandler.CoOccurrences)
		r.Get("/api/analytics/sentiment", analyticsHandler.SentimentDistribution)
		r.Get("/api/analytics/sources", analyticsHandler.SourceHealth)
		r.Get("/api/analytics/volume", analyticsHandler.ArticleVolume)

		r.Get("/api/items/{id}/export", exportHandler.ExportArticle)
		r.Post("/api/export", exportHandler.ExportBulk)
		r.Put("/api/items/{id}/retention", itemsHandler.UpdateRetention)

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

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin)
			r.Post("/api/admin/reenrich", adminHandler.Reenrich)
			r.Post("/api/admin/ingest", adminHandler.TriggerIngest)
			r.Post("/api/admin/chat", adminHandler.ChatWithNews)
		})
	})

	return r
}

// startWorkerCron starts the background cron jobs (ingestion, cleanup, etc).
func startWorkerCron(
	ctx context.Context, wg *sync.WaitGroup,
	cfg config.Config, aiClient *ai.OllamaClient, storageClient *storage.Client,
	articleStore *models.ArticleStore,
	sourceStore *models.SourceStore,
	fingerprintStore *models.FingerprintStore,
	sessionStore *models.SessionStore,
	briefStore *models.BriefStore,
	watchlistOrgStore *models.WatchlistOrgStore,
	watchlistHitStore *models.WatchlistHitStore,
	entityStore *models.EntityStore,
	researchProjectStore *models.ResearchProjectStore,
	researchFindingStore *models.ResearchFindingStore,
	crawlDomainStore *models.CrawlDomainStore,
	crawlQueueStore *models.CrawlQueueStore,
	crawledPageStore *models.CrawledPageStore,
	crawlLinkStore *models.CrawlLinkStore,
	crawlRunStore *models.CrawlRunStore,
	pageEntityStore *models.PageEntityStore,
	entityRelStore *models.EntityRelationshipStore,
	escritoStore *models.EscritoStore,
	escritoSourceStore *models.EscritoSourceStore,
	opportunityStore *models.OpportunityStore,
) *cron.Cron {
	sc := scraper.NewScraper()
	stores := scraper.Stores{
		Articles:     articleStore,
		Sources:      sourceStore,
		Fingerprints: fingerprintStore,
		Entities:     entityStore,
	}

	crawlerDeps := crawler.Deps{
		Domains: crawlDomainStore, Queue: crawlQueueStore, Pages: crawledPageStore,
		Links: crawlLinkStore, Runs: crawlRunStore, Entities: entityStore,
		PageEnts: pageEntityStore, Rels: entityRelStore,
		Opportunities: opportunityStore, AI: aiClient,
	}

	c := cron.New()

	// Ingestion: every 4 hours
	c.AddFunc("0 */4 * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 3*time.Hour)
		defer cancel()
		slog.Info("cron: ingestion")
		scraper.RunIngestion(jobCtx, stores, sc, aiClient, storageClient)
	})

	// Daily brief: 5am
	c.AddFunc("0 5 * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 10*time.Minute)
		defer cancel()
		slog.Info("cron: daily brief")
		scraper.GenerateDailyBrief(jobCtx, articleStore, briefStore, aiClient)
	})

	// Watchlist scan: 4x/day
	c.AddFunc("0 1,7,13,19 * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
		defer cancel()
		slog.Info("cron: watchlist scan")
		agents.RunWatchlistScan(jobCtx, agents.Deps{
			Orgs: watchlistOrgStore, Hits: watchlistHitStore,
			Articles: articleStore, AI: aiClient,
		})
	})

	// Research: every 2 min
	c.AddFunc("*/2 * * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
		defer cancel()
		queued, err := researchProjectStore.ListQueued(jobCtx)
		if err != nil || len(queued) == 0 {
			return
		}
		slog.Info("cron: research", "topic", queued[0].Topic)
		research.RunProject(jobCtx, research.Deps{
			Projects: researchProjectStore, Findings: researchFindingStore,
			Articles: articleStore, CrawledPages: crawledPageStore, AI: aiClient,
		}, queued[0].ID)
	})

	// Escritos: every 2 min
	c.AddFunc("*/2 * * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		queued, err := escritoStore.ListQueued(jobCtx)
		if err != nil || len(queued) == 0 {
			return
		}
		slog.Info("cron: escrito", "topic", queued[0].Topic)
		generator.RunGeneration(jobCtx, generator.Deps{
			Escritos: escritoStore, Sources: escritoSourceStore,
			Articles: articleStore, AI: aiClient,
		}, queued[0].ID, nil)
	})

	// Crawler: every 2 hours
	c.AddFunc("0 */2 * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
		defer cancel()
		slog.Info("cron: crawler")
		crawler.RunCrawl(jobCtx, crawlerDeps, 500)
	})

	// Opportunity extraction: every 30 min
	c.AddFunc("*/30 * * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		slog.Info("cron: opportunity extraction")
		crawler.RunOpportunityExtraction(jobCtx, crawlerDeps, 50)
	})

	// Opportunity status refresh: daily 6am
	c.AddFunc("0 6 * * *", func() {
		wg.Add(1)
		defer wg.Done()
		updated, err := opportunityStore.RefreshStatuses(ctx)
		if err != nil {
			slog.Error("cron: refresh opportunity statuses", "err", err)
		} else {
			slog.Info("cron: opportunity statuses refreshed", "updated", updated)
		}
	})

	// Evidence cleanup: 3am
	c.AddFunc("0 3 * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
		defer cancel()
		scraper.RunEvidenceCleanup(jobCtx, stores, storageClient)
	})

	// Session cleanup: 4am
	c.AddFunc("0 4 * * *", func() {
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
		defer cancel()
		scraper.RunSessionCleanup(jobCtx, sessionStore)
	})

	c.Start()
	slog.Info("worker cron started", "jobs", len(c.Entries()))

	// Initial ingestion after 10s
	go func() {
		select {
		case <-time.After(10 * time.Second):
		case <-ctx.Done():
			return
		}
		wg.Add(1)
		defer wg.Done()
		jobCtx, cancel := context.WithTimeout(ctx, 3*time.Hour)
		defer cancel()
		slog.Info("running initial ingestion")
		scraper.RunIngestion(jobCtx, stores, sc, aiClient, storageClient)
	}()

	return c
}

func checkOllama(host string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get(host + "/api/version")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}
