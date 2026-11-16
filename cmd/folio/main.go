// Command folio es Folio entero en un solo binario: API HTTP + frontend
// embebido + tareas programadas (cron), sin IA.
//
//	folio                              servir (HTTP + cron)
//	folio app                          modo app: Postgres embebido + navegador
//	folio user add <email> [admin|member]   crear o actualizar un usuario
//	folio user list                    listar usuarios
//
// La contraseña de `user add` se lee de FOLIO_PASSWORD o, si no está, de stdin
// (sin eco cuando es una terminal).
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	_ "time/tzdata" // zonas horarias dentro del binario (servidor sin tzdata)

	"github.com/jackc/pgx/v5/pgxpool"

	folio "github.com/Saul-Punybz/folio"
	"github.com/Saul-Punybz/folio/internal/config"
	"github.com/Saul-Punybz/folio/internal/db"
	"github.com/Saul-Punybz/folio/internal/watch"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	args := os.Args[1:]
	// Doble clic en Folio.app: lanzador que arranca `folio app` en segundo
	// plano (o abre el que ya corre) y sale.
	if insideAppBundle() && (len(args) == 0 || strings.HasPrefix(args[0], "-psn")) {
		if err := launch(); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}
	if len(args) > 0 && args[0] == "app" {
		if err := runApp(); err != nil {
			slog.Error("folio: fatal", "err", err)
			os.Exit(1)
		}
		return
	}
	if len(args) == 0 || args[0] == "serve" {
		if err := serve(); err != nil {
			slog.Error("folio: fatal", "err", err)
			os.Exit(1)
		}
		return
	}

	if args[0] == "user" {
		if err := userCommand(args[1:]); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintln(os.Stderr, usage)
	os.Exit(2)
}

const usage = `uso:
  folio                                 servir (HTTP + cron)
  folio app                             modo app (Postgres embebido, 127.0.0.1:47615, abre el navegador)
  folio user add <email> [admin|member] crear/actualizar usuario (contraseña en FOLIO_PASSWORD o stdin)
  folio user list                       listar usuarios`

// connect abre el pool y aplica las migraciones embebidas.
func connect(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	migrationsFS, err := fs.Sub(folio.Migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("embedded migrations: %w", err)
	}
	return db.ConnectWithFS(ctx, cfg, migrationsFS)
}

// serverOptions: lo que cambia entre `folio` (servidor) y `folio app`.
type serverOptions struct {
	db       config.DBConfig
	listener net.Listener
	cron     cronOptions
	onReady  func() // tras quedar escuchando (abrir el navegador)
	// dataDir: carpeta de datos (PDF en dataDir/documentos).
	dataDir string
}

// localDataDir: FOLIO_DATA_DIR, o ./data junto a donde corre `folio`.
func localDataDir() string {
	d := os.Getenv("FOLIO_DATA_DIR")
	if d == "" {
		d = "data"
	}
	if abs, err := filepath.Abs(d); err == nil {
		d = abs
	}
	return d
}

func serve() error {
	cfg := config.Load()

	// Contexto raíz: se cancela con SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ln, err := net.Listen("tcp", cfg.Server.Addr())
	if err != nil {
		return fmt.Errorf("listen %s: %w", cfg.Server.Addr(), err)
	}
	return runServer(ctx, serverOptions{db: cfg.DB, listener: ln, dataDir: localDataDir()})
}

// runServer conecta la base, aplica migraciones, sirve HTTP en o.listener y
// corre el cron hasta que ctx se cancele; luego apaga en orden y cierra el pool.
func runServer(ctx context.Context, o serverOptions) error {
	connectCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	pool, err := connect(connectCtx, o.db)
	cancel()
	if err != nil {
		o.listener.Close()
		return err
	}
	defer pool.Close()

	frontendFS, err := fs.Sub(folio.FrontendDist, "frontend/dist")
	if err != nil {
		o.listener.Close()
		return fmt.Errorf("embedded frontend: %w", err)
	}

	if err := os.MkdirAll(o.dataDir, 0o700); err != nil {
		o.listener.Close()
		return fmt.Errorf("carpeta de datos %s: %w", o.dataDir, err)
	}
	st := newStores(pool, o.dataDir)
	// Plantilla de marcas (idempotente; si aún no hay admin, entra luego).
	watch.SeedBrandTemplates(context.Background(), st.watchOrgs)

	// Cron. jobCtx se cancela al apagar para que los trabajos en curso paren.
	jobCtx, jobCancel := context.WithCancel(context.Background())
	defer jobCancel()
	tracker := &jobTracker{}
	j := newJobs(st)

	// POST /api/shutdown: el mismo apagado que SIGTERM.
	shutdownReq := make(chan struct{})
	var shutdownOnce sync.Once
	requestShutdown := func() { shutdownOnce.Do(func() { close(shutdownReq) }) }

	handler := newRouter(st, frontendFS, routerHooks{
		trigger:  func() bool { return j.trigger(jobCtx, tracker) },
		shutdown: requestShutdown,
	})

	scheduler, err := startCron(jobCtx, tracker, j, o.cron)
	if err != nil {
		o.listener.Close()
		return err
	}

	srv := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("folio: listening", "addr", o.listener.Addr().String())
		if err := srv.Serve(o.listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()
	if o.onReady != nil {
		o.onReady()
	}

	var runErr error
	select {
	case <-ctx.Done():
		slog.Info("folio: shutdown signal received")
	case <-shutdownReq:
		slog.Info("folio: shutdown requested from the app")
	case err, ok := <-serverErr:
		if ok && err != nil {
			runErr = fmt.Errorf("http server: %w", err)
		}
	}

	// ── Apagado ordenado ──────────────────────────────────────────
	// 1. Deja de aceptar HTTP y espera las peticiones en curso.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("folio: http shutdown", "err", err)
	}
	shutdownCancel()

	// 2. Para el cron (no arranca trabajos nuevos), cancela los que corren y
	//    espera a que terminen (con tope).
	cronDone := scheduler.Stop()
	jobCancel()
	waitDone := make(chan struct{})
	go func() {
		<-cronDone.Done()
		tracker.closeAndWait()
		close(waitDone)
	}()
	select {
	case <-waitDone:
		slog.Info("folio: background jobs finished")
	case <-time.After(60 * time.Second):
		slog.Warn("folio: timed out waiting for background jobs")
	}

	slog.Info("folio: stopped")
	return runErr
}
