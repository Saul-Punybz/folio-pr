package main

import (
	"io"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/handlers"
	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// stores agrupa todos los accesos a la base que usan el router y el cron.
type stores struct {
	articles      *models.ArticleStore
	users         *models.UserStore
	sessions      *models.SessionStore
	sources       *models.SourceStore
	notes         *models.NoteStore
	fingerprints  *models.FingerprintStore
	watchOrgs     *models.WatchlistOrgStore
	watchHits     *models.WatchlistHitStore
	opportunities *models.OpportunityStore
	settings      *models.SettingsStore
	crawlDeps     crawler.Deps
	dataDir       string
}

func newStores(pool *pgxpool.Pool, dataDir string) *stores {
	opps := models.NewOpportunityStore(pool)
	return &stores{
		dataDir:       dataDir,
		settings:      models.NewSettingsStore(pool),
		articles:      models.NewArticleStore(pool),
		users:         models.NewUserStore(pool),
		sessions:      models.NewSessionStore(pool),
		sources:       models.NewSourceStore(pool),
		notes:         models.NewNoteStore(pool),
		fingerprints:  models.NewFingerprintStore(pool),
		watchOrgs:     models.NewWatchlistOrgStore(pool),
		watchHits:     models.NewWatchlistHitStore(pool),
		opportunities: opps,
		crawlDeps: crawler.Deps{
			Domains:       models.NewCrawlDomainStore(pool),
			Queue:         models.NewCrawlQueueStore(pool),
			Pages:         models.NewCrawledPageStore(pool),
			Links:         models.NewCrawlLinkStore(pool),
			Runs:          models.NewCrawlRunStore(pool),
			Opportunities: opps,
			Documents:     models.NewDocumentStore(pool),
			DataDir:       dataDir,
		},
	}
}

// routerHooks: lo que el router necesita del proceso (cron y apagado).
type routerHooks struct {
	trigger  func() bool // cadena de fondos en segundo plano
	shutdown func()      // apagado ordenado (igual que SIGTERM)
}

func (s *stores) scraperStores() scraper.Stores {
	return scraper.Stores{Articles: s.articles, Sources: s.sources, Fingerprints: s.fingerprints}
}

// newRouter arma el router: API bajo /api (con sesión salvo health y login),
// el feed RSS público por token, y el frontend embebido para todo lo demás.
func newRouter(st *stores, frontendFS fs.FS, hooks routerHooks) http.Handler {
	sc := scraper.NewScraper()

	authH := &handlers.AuthHandler{Users: st.users, Sessions: st.sessions}
	setupH := &handlers.SetupHandler{Users: st.users}
	itemsH := &handlers.ItemsHandler{Articles: st.articles, Scraper: sc}
	searchH := &handlers.SearchHandler{Articles: st.articles}
	sourcesH := &handlers.SourcesHandler{Sources: st.sources, Scraper: sc}
	notesH := &handlers.NotesHandler{Notes: st.notes, Articles: st.articles}
	watchH := &handlers.WatchlistHandler{Orgs: st.watchOrgs, Hits: st.watchHits, Articles: st.articles}
	exportH := &handlers.ExportHandler{Articles: st.articles, Notes: st.notes}
	feedH := &handlers.FeedHandler{Users: st.users, Hits: st.watchHits}
	oppH := &handlers.OpportunityHandler{Opportunities: st.opportunities, Documents: st.crawlDeps.Documents, DataDir: st.dataDir}
	adminH := &handlers.AdminHandler{
		Articles: st.articles, Sources: st.sources, Fingerprints: st.fingerprints,
		Scraper: sc,
	}
	cd := st.crawlDeps
	crawlerH := &handlers.CrawlerHandler{
		Domains: cd.Domains, Queue: cd.Queue, Pages: cd.Pages, Links: cd.Links, Runs: cd.Runs,
		CrawlDeps: cd, Trigger: hooks.trigger,
	}

	// 5 intentos de login por minuto por IP.
	loginLimiter := middleware.NewRateLimiter(5, time.Minute)

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(middleware.MaxBodySize(10 << 20))
	// Sin CORS: el frontend se sirve desde el mismo origen.

	// ── Públicas ──────────────────────────────────────────────────
	r.Get("/api/health", handlers.Health)
	r.With(middleware.RateLimit(loginLimiter)).Post("/api/login", authH.Login)
	// Primer arranque: crear el primer admin (409 cuando ya hay usuarios).
	r.Get("/api/setup", setupH.Status)
	r.With(middleware.RateLimit(loginLimiter)).Post("/api/setup", setupH.Create)
	r.Get("/feed/{token}.xml", feedH.ServeFeed)
	// Cerrar sesión siempre limpia la cookie y responde 200, aunque la
	// sesión esté vencida o falte.
	r.Post("/api/logout", authH.Logout)

	// ── Con sesión ────────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(chimw.Timeout(60 * time.Second))
		r.Use(middleware.SessionAuth(st.sessions, st.users))

		r.Get("/api/me", authH.Me)
		r.Post("/api/shutdown", handlers.Shutdown(hooks.shutdown))

		r.Get("/api/items", itemsH.ListItems)
		r.Post("/api/items/trash", itemsH.TrashMany)
		r.Post("/api/items/undo", itemsH.UndoMany)
		r.Post("/api/items/{id}/save", itemsH.SaveItem)
		r.Post("/api/items/{id}/trash", itemsH.TrashItem)
		r.Post("/api/items/{id}/pin", itemsH.PinItem)
		r.Post("/api/items/{id}/undo", itemsH.UndoItem)
		r.Get("/api/items/{id}/notes", notesH.ListNotes)
		r.Post("/api/items/{id}/notes", notesH.CreateNote)
		r.Get("/api/items/{id}/export", exportH.ExportArticle)
		r.Post("/api/collect", itemsH.CollectItem)
		r.Get("/api/search", searchH.Search)
		r.Delete("/api/notes/{noteId}", notesH.DeleteNote)
		r.Post("/api/export", exportH.ExportBulk)

		r.Route("/api/watchlist", func(r chi.Router) {
			r.Get("/orgs", watchH.ListOrgs)
			r.Post("/orgs", watchH.CreateOrg)
			r.Put("/orgs/{id}", watchH.UpdateOrg)
			r.Delete("/orgs/{id}", watchH.DeleteOrg)
			r.Patch("/orgs/{id}/toggle", watchH.ToggleOrg)
			r.Get("/hits", watchH.ListHits)
			r.Get("/hits/unseen", watchH.CountUnseen)
			r.Post("/hits/{id}/seen", watchH.MarkSeen)
			r.Post("/hits/seen-all", watchH.MarkAllSeen)
			r.Delete("/hits/{id}", watchH.DeleteHit)
			r.Post("/scan", watchH.TriggerScan)
			r.Get("/feed-url", feedH.GetFeedURL)
			r.Post("/feed-url/regenerate", feedH.RegenerateFeedURL)
		})

		r.Route("/api/crawler", func(r chi.Router) {
			r.Get("/stats", crawlerH.Stats)
			r.Get("/runs", crawlerH.ListRuns)
			r.Get("/queue", crawlerH.QueueStats)
			r.Get("/pages", crawlerH.ListPages)
			r.Get("/pages/search", crawlerH.SearchPages)
			r.Get("/pages/changes", crawlerH.ListChangedPages)
			r.Get("/pages/{id}", crawlerH.GetPage)
			r.Get("/domains", crawlerH.ListDomains)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Post("/domains", crawlerH.CreateDomain)
				r.Put("/domains/{id}", crawlerH.UpdateDomain)
				r.Patch("/domains/{id}/toggle", crawlerH.ToggleDomain)
				r.Delete("/domains/{id}", crawlerH.DeleteDomain)
				r.Post("/trigger", crawlerH.TriggerCrawl)
			})
		})

		r.Route("/api/documents", func(r chi.Router) {
			r.Get("/kept", oppH.KeptDocuments)
			r.Get("/{id}/file", oppH.DocumentFile)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Post("/{id}/unkeep", oppH.UnkeepDocumentByID)
			})
		})

		r.Route("/api/opportunities", func(r chi.Router) {
			r.Get("/", oppH.List)
			r.Get("/stats", oppH.Stats)
			r.Get("/search", oppH.Search)
			r.Get("/{id}", oppH.Get)
			r.Get("/{id}/document", oppH.Document)
			r.Group(func(r chi.Router) {
				r.Use(middleware.RequireAdmin)
				r.Post("/{id}/unreview", oppH.Unreview)
				r.Post("/{id}/document/keep", oppH.KeepDocument)
				r.Post("/{id}/document/unkeep", oppH.UnkeepDocument)
				r.Put("/{id}", oppH.Update)
				r.Delete("/{id}", oppH.Delete)
				r.Post("/{id}/review", oppH.Review)
				r.Post("/refresh-statuses", oppH.RefreshStatuses)
			})
		})

		r.Group(func(r chi.Router) {
			r.Use(middleware.RequireAdmin)
			r.Get("/api/sources", sourcesH.ListSources)
			r.Post("/api/sources", sourcesH.CreateSource)
			r.Post("/api/sources/quick", sourcesH.QuickCreateSource)
			r.Put("/api/sources/{id}", sourcesH.UpdateSource)
			r.Patch("/api/sources/{id}/toggle", sourcesH.ToggleSource)
			r.Delete("/api/sources/{id}", sourcesH.DeleteSource)
			r.Post("/api/sources/{id}/test", sourcesH.TestScrape)
			r.Post("/api/admin/ingest", adminH.TriggerIngest)
		})
	})

	// Todo lo que no es API: frontend embebido.
	static := staticHandler(frontendFS)
	r.NotFound(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/api" || strings.HasPrefix(req.URL.Path, "/api/") || strings.HasPrefix(req.URL.Path, "/feed/") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"not found"}`)
			return
		}
		static.ServeHTTP(w, req)
	})

	return r
}

// staticHandler sirve el build de Astro embebido con su convención de
// directorios: /inbox → inbox/index.html. La raíz redirige a /inbox.
// (Lógica traída del AssetServer de la vieja app de escritorio.)
func staticHandler(frontendFS fs.FS) http.Handler {
	fileServer := http.FileServer(http.FS(frontendFS))

	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != http.MethodGet && req.Method != http.MethodHead {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		trimmed := strings.Trim(req.URL.Path, "/")

		if trimmed == "" {
			http.Redirect(w, req, "/inbox", http.StatusFound)
			return
		}

		// Archivo exacto (assets: /_astro/foo.js, /favicon.svg).
		if info, err := fs.Stat(frontendFS, trimmed); err == nil && !info.IsDir() {
			if strings.HasPrefix(trimmed, "_astro/") {
				// Astro pone hash en el nombre: se pueden cachear para siempre.
				w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
			}
			fileServer.ServeHTTP(w, req)
			return
		}

		// Página de Astro: /inbox → inbox/index.html.
		if serveHTML(w, frontendFS, trimmed+"/index.html", http.StatusOK) {
			return
		}

		// No existe: 404 (el 404.html de Astro si lo hay). Antes caía en la
		// raíz con 200 y una ruta mal escrita parecía una página buena.
		if serveHTML(w, frontendFS, "404.html", http.StatusNotFound) {
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, notFoundPage)
	})
}

func serveHTML(w http.ResponseWriter, fsys fs.FS, name string, status int) bool {
	f, err := fsys.Open(name)
	if err != nil {
		return false
	}
	defer f.Close()
	if info, err := f.Stat(); err != nil || info.IsDir() {
		return false
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.WriteHeader(status)
	_, _ = io.Copy(w, f)
	return true
}

const notFoundPage = `<!doctype html><html lang="es"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1"><title>Folio · No existe</title>
<style>body{font-family:system-ui,sans-serif;margin:0;min-height:100vh;display:grid;place-items:center;background:#fff;color:#1c1b1f}
@media (prefers-color-scheme:dark){body{background:#141218;color:#e6e0e9}}a{color:inherit;font-weight:600}</style></head>
<body><main><h1>Esta página no existe</h1><p><a href="/inbox">Volver a Folio</a></p></main></body></html>`
