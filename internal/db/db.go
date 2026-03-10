// Package db manages the PostgreSQL connection pool and runs migrations.
package db

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Saul-Punybz/folio/internal/config"
)

// Connect creates a pgxpool connection pool and runs pending migrations
// from the filesystem (migrations/ directory).
func Connect(ctx context.Context, cfg config.DBConfig) (*pgxpool.Pool, error) {
	pool, err := connectPool(ctx, cfg)
	if err != nil {
		return nil, err
	}

	if err := runMigrationsFromDir(ctx, pool); err != nil {
		pool.Close()
		return nil, fmt.Errorf("db: migrations: %w", err)
	}

	return pool, nil
}

// ConnectWithFS creates a pgxpool connection pool and runs pending migrations
// from an embedded filesystem. Used by the macOS .app bundle.
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

// ConnectWithDSN creates a pool using a raw DSN string (for embedded PG).
func ConnectWithDSN(ctx context.Context, dsn string, migrationsFS fs.FS) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(dsn)
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

	slog.Info("database connected", "dsn", dsn[:strings.Index(dsn, "@")+1]+"...")

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

	for _, f := range files {
		var exists bool
		err := pool.QueryRow(ctx, "SELECT EXISTS(SELECT 1 FROM _migrations WHERE filename = $1)", f).Scan(&exists)
		if err != nil {
			return fmt.Errorf("check migration %s: %w", f, err)
		}
		if exists {
			continue
		}

		content, err := readFile(f)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", f, err)
		}

		slog.Info("applying migration", "file", f)

		if _, err := pool.Exec(ctx, string(content)); err != nil {
			return fmt.Errorf("exec migration %s: %w", f, err)
		}

		if _, err := pool.Exec(ctx, "INSERT INTO _migrations (filename) VALUES ($1)", f); err != nil {
			return fmt.Errorf("record migration %s: %w", f, err)
		}
	}

	slog.Info("migrations complete", "count", len(files))
	return nil
}

// runMigrationsFromDir reads SQL files from the migrations/ directory.
func runMigrationsFromDir(ctx context.Context, pool *pgxpool.Pool) error {
	migrationsDir := "migrations"
	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
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

	return applyMigrations(ctx, pool, files, func(name string) ([]byte, error) {
		return os.ReadFile(filepath.Join(migrationsDir, name))
	})
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
