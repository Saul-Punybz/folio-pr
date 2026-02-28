// Package db manages the PostgreSQL connection pool and runs migrations.
package db

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Saul-Punybz/folio/internal/config"
)

// Connect creates a pgxpool connection pool and runs pending migrations.
func Connect(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
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

	if err := runMigrations(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: migrations: %w", err)
	}

	return pool, nil
}

// runMigrations reads SQL files from the migrations/ directory and executes
// them in sorted order. It uses a simple migrations tracking table to avoid
// re-running migrations.
func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	// Create the tracking table if it doesn't exist.
	const createTracker = `
		CREATE TABLE IF NOT EXISTS _migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		);`
	if _, err := pool.Exec(ctx, createTracker); err != nil {
		return fmt.Errorf("create tracker table: %w", err)
	}

	// Find migration files.
	migrationsDir := "migrations"
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		// If the migrations directory doesn't exist, skip silently.
		if os.IsNotExist(err) {
			slog.Info("migrations directory not found, skipping")
			return nil
		}
		return fmt.Errorf("read migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		// Check if already applied.
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM _migrations WHERE filename = $1)", f).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", f, err)
		}
		if exists {
			continue
		}

		// Read and execute the migration.
		content, err := os.ReadFile(filepath.Join(migrationsDir, f))
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}

		slog.Info("applying migration", "file", f)

		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("exec migration %s: %w", f, err)
		}

		// Record it.
		if _, err := pool.Exec(ctx, "INSERT INTO _migrations (filename) VALUES ($1)", f); err != nil {
			return fmt.Errorf("record migration %s: %w", f, err)
		}
	}

	slog.Info("migrations complete", "count", len(files))
	return nil
}
