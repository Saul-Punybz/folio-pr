// Package db manages the PostgreSQL connection pool and runs migrations.
package db

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Saul-Punybz/folio/internal/config"
)

// ConnectWithFS creates a pgxpool connection pool and runs pending migrations
// from an embedded filesystem (folio.Migrations, rooted at the .sql files).
func ConnectWithFS(ctx context.Context, cfg config.DBConfig, migrationsFS fs.FS) (*pgxpool.Pool, error) {
	pool, err := connectPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := runMigrationsFromFS(ctx, pool, migrationsFS); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: migrations: %w", err)
	}

	return pool, nil
}

func connectPool(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("db: parse config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("db: connect: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: ping: %w", err)
	}

	slog.Info("database connected", "host", cfg.Host, "port", cfg.Port, "db", cfg.DBName)
	return pool, nil
}

// applyMigrations executes migration files against the pool.
func applyMigrations(ctx context.Context, pool *pgxpool.Pool, files []string, readFile func(string) ([]byte, error)) error {
	const createTracker = `
		CREATE TABLE IF NOT EXISTS _migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`
	if _, err := pool.Exec(ctx, createTracker); err != nil {
		return fmt.Errorf("create tracker table: %w", err)
	}

	applied := 0
	for _, f := range files {
		ok, err := applyOne(ctx, pool, f, readFile)
		if err != nil {
			return err
		}
		if ok {
			applied++
		}
	}

	slog.Info("migrations complete", "files", len(files), "applied", applied)
	return nil
}

// applyOne runs a single migration and records it in _migrations inside ONE
// transaction: either both happen or neither does. A transaction-scoped
// advisory lock keeps two processes from applying the same file twice.
// Returns true if the migration was applied now.
func applyOne(ctx context.Context, pool *pgxpool.Pool, f string, readFile func(string) ([]byte, error)) (bool, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return false, fmt.Errorf("begin migration %s: %w", f, err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	// Serializa migraciones entre procesos (clave arbitraria fija).
	if _, err := tx.Exec(ctx, "SELECT pg_advisory_xact_lock(7461636)"); err != nil {
		return false, fmt.Errorf("lock migration %s: %w", f, err)
	}

	var exists bool
	if err := tx.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM _migrations WHERE filename = $1)", f).Scan(&exists); err != nil {
		return false, fmt.Errorf("check migration %s: %w", f, err)
	}
	if exists {
		return false, nil
	}

	content, err := readFile(f)
	if err != nil {
		return false, fmt.Errorf("read migration %s: %w", f, err)
	}

	slog.Info("applying migration", "file", f)
	if _, err := tx.Exec(ctx, string(content)); err != nil {
		return false, fmt.Errorf("exec migration %s: %w", f, err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO _migrations (filename) VALUES ($1)", f); err != nil {
		return false, fmt.Errorf("record migration %s: %w", f, err)
	}
	if err := tx.Commit(ctx); err != nil {
		return false, fmt.Errorf("commit migration %s: %w", f, err)
	}
	return true, nil
}

// runMigrationsFromFS reads SQL files from an embedded filesystem.
func runMigrationsFromFS(ctx context.Context, pool *pgxpool.Pool, fsys fs.FS) error {
	entries, err := fs.ReadDir(fsys, ".")
	if err != nil {
		return fmt.Errorf("read embedded migrations: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	return applyMigrations(ctx, pool, files, func(name string) ([]byte, error) {
		return fs.ReadFile(fsys, name)
	})
}
