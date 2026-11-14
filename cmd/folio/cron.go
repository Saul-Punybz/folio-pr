package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/scraper"
	"github.com/Saul-Punybz/folio/internal/storage"
	"github.com/Saul-Punybz/folio/internal/watch"
)

// jobTracker cuenta los trabajos en curso. start() hace el wg.Add(1) bajo un
// mutex y se niega una vez que empezó el apagado, así ningún Add compite con
// el Wait de closeAndWait (regla de sync.WaitGroup).
type jobTracker struct {
	mu     sync.Mutex
	closed bool
	wg     sync.WaitGroup
}

func (t *jobTracker) start() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return false
	}
	t.wg.Add(1)
	return true
}

func (t *jobTracker) done() { t.wg.Done() }

func (t *jobTracker) closeAndWait() {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	t.wg.Wait()
}

// cronLocation: hora local de la organización. FOLIO_TZ la cambia.
func cronLocation() *time.Location {
	name := os.Getenv("FOLIO_TZ")
	if name == "" {
		name = "America/Puerto_Rico"
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		slog.Warn("cron: bad FOLIO_TZ, using UTC", "tz", name, "err", err)
		return time.UTC
	}
	return loc
}

// startCron registra y arranca las tareas programadas. Ninguna usa IA.
func startCron(ctx context.Context, t *jobTracker, st *stores, storageClient *storage.Client) (*cron.Cron, error) {
	loc := cronLocation()
	c := cron.New(
		cron.WithLocation(loc),
		// Si una corrida sigue cuando toca la próxima, se salta (no se apilan).
		cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)),
	)

	ss := st.scraperStores()
	sc := scraper.NewScraper()

	// job envuelve cada tarea: cuenta en el tracker y le da su timeout.
	job := func(name string, timeout time.Duration, fn func(context.Context)) func() {
		return func() {
			if !t.start() {
				return
			}
			defer t.done()
			jobCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			slog.Info("cron: start", "job", name)
			fn(jobCtx)
		}
	}

	// La corrida inicial y la del cron no deben pisarse.
	var ingestMu sync.Mutex
	ingest := func(jobCtx context.Context) {
		if !ingestMu.TryLock() {
			slog.Info("cron: ingestion already running, skipping")
			return
		}
		defer ingestMu.Unlock()
		n := scraper.RunIngestion(jobCtx, ss, sc, storageClient)
		slog.Info("cron: ingestion done", "articles", n)
	}

	specs := []struct {
		spec string
		name string
		fn   func()
	}{
		{"0 */4 * * *", "ingestion", job("ingestion", 3*time.Hour, ingest)},
		{"0 1,7,13,19 * * *", "watchlist-scan", job("watchlist-scan", 2*time.Hour, func(jobCtx context.Context) {
			watch.RunWatchlistScan(jobCtx, watch.Deps{Orgs: st.watchOrgs, Hits: st.watchHits, Articles: st.articles})
		})},
		{"0 */2 * * *", "crawler", job("crawler", 110*time.Minute, func(jobCtx context.Context) {
			crawler.RunCrawl(jobCtx, st.crawlDeps, 500)
		})},
		{"0 2 * * *", "recrawl", job("recrawl", 30*time.Minute, func(jobCtx context.Context) {
			crawler.RunRecrawl(jobCtx, st.crawlDeps)
		})},
		{"0 6 * * *", "opportunity-statuses", job("opportunity-statuses", 5*time.Minute, func(jobCtx context.Context) {
			n, err := st.opportunities.RefreshStatuses(jobCtx)
			if err != nil {
				slog.Error("cron: refresh opportunity statuses", "err", err)
				return
			}
			slog.Info("cron: opportunity statuses refreshed", "updated", n)
		})},
		{"0 4 * * *", "session-cleanup", job("session-cleanup", 5*time.Minute, func(jobCtx context.Context) {
			scraper.RunSessionCleanup(jobCtx, st.sessions)
		})},
	}
	if storageClient.Configured() {
		specs = append(specs, struct {
			spec string
			name string
			fn   func()
		}{"0 3 * * *", "evidence-cleanup", job("evidence-cleanup", 30*time.Minute, func(jobCtx context.Context) {
			scraper.RunEvidenceCleanup(jobCtx, ss, storageClient)
		})})
	}

	for _, s := range specs {
		if _, err := c.AddFunc(s.spec, s.fn); err != nil {
			return nil, fmt.Errorf("cron %s: %w", s.name, err)
		}
	}
	c.Start()
	slog.Info("cron: started", "jobs", len(c.Entries()), "tz", loc.String(), "evidence_cleanup", storageClient.Configured())

	// Corrida inicial de ingesta a los 10 s. El Add se hace AQUÍ, antes de
	// lanzar la goroutine, para que no compita con el Wait del apagado.
	if t.start() {
		go func() {
			defer t.done()
			select {
			case <-time.After(10 * time.Second):
			case <-ctx.Done():
				return
			}
			jobCtx, cancel := context.WithTimeout(ctx, 3*time.Hour)
			defer cancel()
			slog.Info("cron: initial ingestion")
			ingest(jobCtx)
		}()
	}

	return c, nil
}
