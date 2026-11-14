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

// task es un trabajo con nombre, timeout y su propio candado: la corrida de
// arranque y la del cron nunca se pisan.
type task struct {
	name    string
	timeout time.Duration
	fn      func(context.Context)
	mu      sync.Mutex
}

// run ejecuta la tarea. wait=false (cron): si ya corre, se salta.
// wait=true (puesta al día del arranque): espera su turno.
func (tk *task) run(ctx context.Context, wait bool) {
	if wait {
		tk.mu.Lock()
	} else if !tk.mu.TryLock() {
		slog.Info("cron: already running, skipping", "job", tk.name)
		return
	}
	defer tk.mu.Unlock()
	if ctx.Err() != nil {
		return
	}
	jobCtx, cancel := context.WithTimeout(ctx, tk.timeout)
	defer cancel()
	slog.Info("cron: start", "job", tk.name)
	tk.fn(jobCtx)
}

type cronOptions struct {
	// catchUp: al arrancar, correr una vez y en orden lo que tocaba mientras
	// la app estuvo cerrada (modo app). Si es false, solo la ingesta inicial.
	catchUp bool
	// extra: tareas adicionales (p. ej. el respaldo) con su horario.
	extra []extraJob
}

type extraJob struct {
	spec    string
	name    string
	timeout time.Duration
	fn      func(context.Context)
	// atStartup: corre también al arrancar, antes de la puesta al día.
	atStartup bool
}

// startCron registra y arranca las tareas programadas. Ninguna usa IA.
func startCron(ctx context.Context, t *jobTracker, st *stores, opts cronOptions) (*cron.Cron, error) {
	loc := cronLocation()
	c := cron.New(
		cron.WithLocation(loc),
		// Si una corrida sigue cuando toca la próxima, se salta (no se apilan).
		cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)),
	)

	ss := st.scraperStores()
	sc := scraper.NewScraper()

	ingestion := &task{name: "ingestion", timeout: 3 * time.Hour, fn: func(jobCtx context.Context) {
		n := scraper.RunIngestion(jobCtx, ss, sc)
		slog.Info("cron: ingestion done", "articles", n)
	}}
	watchScan := &task{name: "watchlist-scan", timeout: 2 * time.Hour, fn: func(jobCtx context.Context) {
		watch.RunWatchlistScan(jobCtx, watch.Deps{Orgs: st.watchOrgs, Hits: st.watchHits, Articles: st.articles})
	}}
	crawl := &task{name: "crawler", timeout: 110 * time.Minute, fn: func(jobCtx context.Context) {
		crawler.RunCrawl(jobCtx, st.crawlDeps, 500)
	}}
	recrawl := &task{name: "recrawl", timeout: 30 * time.Minute, fn: func(jobCtx context.Context) {
		crawler.RunRecrawl(jobCtx, st.crawlDeps)
	}}
	oppStatuses := &task{name: "opportunity-statuses", timeout: 5 * time.Minute, fn: func(jobCtx context.Context) {
		n, err := st.opportunities.RefreshStatuses(jobCtx)
		if err != nil {
			slog.Error("cron: refresh opportunity statuses", "err", err)
			return
		}
		slog.Info("cron: opportunity statuses refreshed", "updated", n)
	}}
	sessionCleanup := &task{name: "session-cleanup", timeout: 5 * time.Minute, fn: func(jobCtx context.Context) {
		scraper.RunSessionCleanup(jobCtx, st.sessions)
	}}

	// fromCron envuelve una tarea para el cron: cuenta en el tracker.
	fromCron := func(tk *task) func() {
		return func() {
			if !t.start() {
				return
			}
			defer t.done()
			tk.run(ctx, false)
		}
	}

	specs := []struct {
		spec string
		tk   *task
	}{
		{"0 */4 * * *", ingestion},
		{"0 1,7,13,19 * * *", watchScan},
		{"0 */2 * * *", crawl},
		{"0 2 * * *", recrawl},
		{"0 6 * * *", oppStatuses},
		{"0 4 * * *", sessionCleanup},
	}
	var startup []*task
	for _, e := range opts.extra {
		tk := &task{name: e.name, timeout: e.timeout, fn: e.fn}
		specs = append(specs, struct {
			spec string
			tk   *task
		}{e.spec, tk})
		if e.atStartup {
			startup = append(startup, tk)
		}
	}
	for _, s := range specs {
		if _, err := c.AddFunc(s.spec, fromCron(s.tk)); err != nil {
			return nil, fmt.Errorf("cron %s: %w", s.tk.name, err)
		}
	}
	c.Start()
	slog.Info("cron: started", "jobs", len(c.Entries()), "tz", loc.String(), "catch_up", opts.catchUp)

	// Corrida de arranque a los 10 s. El Add se hace AQUÍ, antes de lanzar la
	// goroutine, para que no compita con el Wait del apagado.
	if opts.catchUp {
		// Lo que tocaba mientras estuvo cerrada, una vez y en este orden.
		startup = append(startup, ingestion, crawl, recrawl, watchScan, oppStatuses)
	} else {
		startup = append(startup, ingestion)
	}
	if t.start() {
		go func() {
			defer t.done()
			select {
			case <-time.After(10 * time.Second):
			case <-ctx.Done():
				return
			}
			slog.Info("cron: startup run", "jobs", len(startup))
			for _, tk := range startup {
				if ctx.Err() != nil {
					return
				}
				tk.run(ctx, true)
			}
			slog.Info("cron: startup run done")
		}()
	}

	return c, nil
}
