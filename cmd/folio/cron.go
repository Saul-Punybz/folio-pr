package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/grants"
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

// jobs son las tareas de Folio, cada una con su candado. Las usan el cron,
// la puesta al día del arranque y el botón «Buscar ahora» (TriggerCrawl).
type jobs struct {
	ingestion, watchScan, crawl, recrawl, extraction, grants, oppStatuses, sessionCleanup *task
	// docCleanup: retención de los PDF guardados (diaria y al arrancar).
	docCleanup *task
	// noticias: lo del Inbox con más de 7 días pasa a Archivo y lo borrado
	// hace más de 1 h se elimina (diaria y al arrancar).
	noticias *task
	// archivo: copia de respaldo, cita y enlace en Internet Archive de las
	// menciones de marca (tandas de 150; al arrancar rellena las que ya había).
	archivo *task
	// fondos: la cadena crawl → extracción → estados (cada 2 h).
	fondos *task
	// fondosFull: la cadena completa con Grants.gov (manual y arranque).
	fondosFull *task
}

// Presupuesto de páginas/PDF por corrida del crawler.
const (
	cronCrawlBudget   = 500
	manualCrawlBudget = 400
)

// crawlBudget: FOLIO_CRAWL_BUDGET cambia el presupuesto (pruebas).
func crawlBudget(def int) int {
	if v, err := strconv.Atoi(os.Getenv("FOLIO_CRAWL_BUDGET")); err == nil && v > 0 {
		return v
	}
	return def
}

func newJobs(st *stores) *jobs {
	ss := st.scraperStores()
	sc := scraper.NewScraper()
	j := &jobs{}
	j.ingestion = &task{name: "ingestion", timeout: 3 * time.Hour, fn: func(jobCtx context.Context) {
		n := scraper.RunIngestion(jobCtx, ss, sc)
		slog.Info("cron: ingestion done", "articles", n)
	}}
	j.watchScan = &task{name: "watchlist-scan", timeout: 2 * time.Hour, fn: func(jobCtx context.Context) {
		watch.RunWatchlistScan(jobCtx, watch.Deps{Orgs: st.watchOrgs, Hits: st.watchHits, Articles: st.articles})
	}}
	crawlWith := func(budget int, force bool) func(context.Context) {
		return func(jobCtx context.Context) {
			deps := st.crawlDeps
			deps.ForceEntries = force
			crawler.RunCrawl(jobCtx, deps, budget)
		}
	}
	j.crawl = &task{name: "crawler", timeout: 110 * time.Minute, fn: crawlWith(crawlBudget(cronCrawlBudget), false)}
	j.recrawl = &task{name: "recrawl", timeout: 30 * time.Minute, fn: func(jobCtx context.Context) {
		crawler.RunRecrawl(jobCtx, st.crawlDeps)
	}}
	j.extraction = &task{name: "extraction", timeout: 60 * time.Minute, fn: func(jobCtx context.Context) {
		crawler.RunExtraction(jobCtx, st.crawlDeps)
	}}
	j.grants = &task{name: "grants-gov", timeout: 15 * time.Minute, fn: func(jobCtx context.Context) {
		crawler.RunGrants(jobCtx, st.opportunities, grants.New(nil), crawler.GrantsKeywords(jobCtx, st.settings))
	}}
	j.oppStatuses = &task{name: "opportunity-statuses", timeout: 5 * time.Minute, fn: func(jobCtx context.Context) {
		n, err := st.opportunities.RefreshStatuses(jobCtx)
		if err != nil {
			slog.Error("cron: refresh opportunity statuses", "err", err)
			return
		}
		slog.Info("cron: opportunity statuses refreshed", "updated", n)
	}}
	j.docCleanup = &task{name: "document-cleanup", timeout: 30 * time.Minute, fn: func(jobCtx context.Context) {
		// Comparte el candado de la extracción: no se borra un PDF mientras
		// la extracción lo está leyendo (ni al revés).
		j.extraction.mu.Lock()
		defer j.extraction.mu.Unlock()
		crawler.RunDocumentCleanup(jobCtx, st.crawlDeps, st.settings)
	}}
	j.noticias = &task{name: "noticias-mantenimiento", timeout: 10 * time.Minute, fn: func(jobCtx context.Context) {
		runNoticiasMaintenance(jobCtx, st.articles)
	}}
	j.archivo = &task{name: "archivo-menciones", timeout: 2 * time.Hour, fn: func(jobCtx context.Context) {
		watch.RunArchive(jobCtx, st.watchHits, watch.ArchiveRunCap)
	}}
	j.sessionCleanup = &task{name: "session-cleanup", timeout: 5 * time.Minute, fn: func(jobCtx context.Context) {
		scraper.RunSessionCleanup(jobCtx, st.sessions)
	}}
	// Las cadenas esperan el candado de cada paso: nunca se encima un crawl
	// con otro ni una extracción con otra, venga de donde venga.
	j.fondos = &task{name: "fondos", timeout: 3 * time.Hour, fn: func(jobCtx context.Context) {
		j.crawl.run(jobCtx, true)
		j.extraction.run(jobCtx, true)
		j.oppStatuses.run(jobCtx, true)
	}}
	manualCrawl := &task{name: "crawler-manual", timeout: 110 * time.Minute, fn: crawlWith(crawlBudget(manualCrawlBudget), true)}
	j.fondosFull = &task{name: "fondos-completo", timeout: 4 * time.Hour, fn: func(jobCtx context.Context) {
		// El manual comparte el candado del crawl del cron.
		j.crawl.mu.Lock()
		manualCrawl.run(jobCtx, true)
		j.crawl.mu.Unlock()
		j.extraction.run(jobCtx, true)
		j.grants.run(jobCtx, true)
		j.oppStatuses.run(jobCtx, true)
	}}
	return j
}

// trigger lanza la cadena completa de fondos en segundo plano (botón
// «Buscar ahora»). false si ya hay una corriendo o Folio se está apagando.
func (j *jobs) trigger(ctx context.Context, t *jobTracker) bool {
	if !j.fondosFull.mu.TryLock() {
		return false
	}
	j.fondosFull.mu.Unlock()
	if !t.start() {
		return false
	}
	go func() {
		defer t.done()
		j.fondosFull.run(ctx, false)
	}()
	return true
}

// startCron registra y arranca las tareas programadas. Ninguna usa IA.
func startCron(ctx context.Context, t *jobTracker, j *jobs, opts cronOptions) (*cron.Cron, error) {
	loc := cronLocation()
	c := cron.New(
		cron.WithLocation(loc),
		// Si una corrida sigue cuando toca la próxima, se salta (no se apilan).
		cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)),
	)

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
		{"0 */4 * * *", j.ingestion},
		{"0 1,7,13,19 * * *", j.watchScan},
		{"0 */2 * * *", j.fondos},
		{"0 2 * * *", j.recrawl},
		{"30 5 * * *", j.grants},
		{"0 6 * * *", j.oppStatuses},
		{"0 4 * * *", j.sessionCleanup},
		{"30 3 * * *", j.docCleanup},
		{"45 3 * * *", j.noticias},
		{"30 2,8,14,20 * * *", j.archivo},
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
		// Lo que tocaba mientras estuvo cerrada, una vez y en este orden:
		// noticias, recrawl, crawl → extracción → Grants.gov → estados, vigilancias.
		startup = append(startup, j.noticias, j.ingestion, j.recrawl, j.crawl, j.extraction, j.docCleanup, j.grants, j.oppStatuses, j.watchScan, j.archivo)
	} else {
		startup = append(startup, j.noticias, j.ingestion, j.archivo)
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
