// Command folio es Folio entero en un solo binario: API HTTP + frontend
// embebido + tareas programadas (cron), sin IA.
//
//	folio                              servir (HTTP + cron)
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
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
	_ "time/tzdata" // zonas horarias dentro del binario (servidor sin tzdata)

	"github.com/jackc/pgx/v5/pgxpool"

	folio "github.com/Saul-Punybz/folio"
	"github.com/Saul-Punybz/folio/internal/config"
	"github.com/Saul-Punybz/folio/internal/db"
	"github.com/Saul-Punybz/folio/internal/storage"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	args := os.Args[1:]
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
  folio user add <email> [admin|member] crear/actualizar usuario (contraseña en FOLIO_PASSWORD o stdin)
  folio user list                       listar usuarios`

// connect abre el pool y aplica las migraciones embebidas.
func connect(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	migrationsFS, err := fs.Sub(folio.Migrations, "migrations")
	if err != nil {
		return nil, fmt.Errorf("embedded migrations: %w", err)
	}
	return db.ConnectWithFS(ctx, cfg.DB, migrationsFS)
}

func serve() error {
	cfg := config.Load()

	// Contexto raíz: se cancela con SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	connectCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	pool, err := connect(connectCtx, cfg)
	cancel()
	if err != nil {
		return err
	}
	defer pool.Close()

	storageClient, err := storage.NewClient(ctx, cfg.S3)
	if err != nil {
		slog.Warn("storage: not available, evidence archive disabled", "err", err)
		storageClient = nil
	}

	frontendFS, err := fs.Sub(folio.FrontendDist, "frontend/dist")
	if err != nil {
		return fmt.Errorf("embedded frontend: %w", err)
	}

	st := newStores(pool)
	handler := newRouter(st, storageClient, frontendFS)

	// Cron. jobCtx se cancela al apagar para que los trabajos en curso paren.
	jobCtx, jobCancel := context.WithCancel(context.Background())
	defer jobCancel()
	tracker := &jobTracker{}
	scheduler, err := startCron(jobCtx, tracker, st, storageClient)
	if err != nil {
		return err
	}

	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("folio: listening", "addr", srv.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
		close(serverErr)
	}()

	var runErr error
	select {
	case <-ctx.Done():
		slog.Info("folio: shutdown signal received")
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
