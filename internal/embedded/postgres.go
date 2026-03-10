// Package embedded manages an embedded PostgreSQL instance for the macOS app.
package embedded

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Postgres manages an embedded PostgreSQL server.
type Postgres struct {
	BinDir  string // path to pg bin/ directory (inside .app or Homebrew)
	LibDir  string // path to pg lib/ directory
	DataDir string // path to data directory (~/Library/Application Support/Folio/pgdata)
	LogFile string // path to pg log file
	Port    int
	DBName  string
	User    string
}

// FreePort finds an available TCP port.
func FreePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return port, nil
}

// DataDirectory returns the default data directory for Folio.
func DataDirectory() string {
	if runtime.GOOS == "darwin" {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Folio")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".folio")
}

// FindPGBinDir locates the PostgreSQL binaries.
// It checks: 1) .app bundle Resources/pg/bin, 2) Homebrew, 3) PATH
func FindPGBinDir() (string, string, error) {
	// Check if running inside a .app bundle
	exe, _ := os.Executable()
	macosDir := filepath.Dir(exe)
	contentsDir := filepath.Dir(macosDir)
	if filepath.Base(contentsDir) == "Contents" {
		appPG := filepath.Join(contentsDir, "Resources", "pg")
		binDir := filepath.Join(appPG, "bin")
		libDir := filepath.Join(appPG, "lib")
		if _, err := os.Stat(filepath.Join(binDir, "pg_ctl")); err == nil {
			return binDir, libDir, nil
		}
	}

	// Check Homebrew
	for _, ver := range []string{"17", "16", "15"} {
		prefix := fmt.Sprintf("/opt/homebrew/opt/postgresql@%s", ver)
		binDir := filepath.Join(prefix, "bin")
		libDir := filepath.Join(prefix, "lib")
		if _, err := os.Stat(filepath.Join(binDir, "pg_ctl")); err == nil {
			return binDir, libDir, nil
		}
		// Intel Mac
		prefix = fmt.Sprintf("/usr/local/opt/postgresql@%s", ver)
		binDir = filepath.Join(prefix, "bin")
		libDir = filepath.Join(prefix, "lib")
		if _, err := os.Stat(filepath.Join(binDir, "pg_ctl")); err == nil {
			return binDir, libDir, nil
		}
	}

	// Check PATH
	pgCtl, err := exec.LookPath("pg_ctl")
	if err == nil {
		binDir := filepath.Dir(pgCtl)
		libDir := filepath.Join(filepath.Dir(binDir), "lib")
		return binDir, libDir, nil
	}

	return "", "", fmt.Errorf("PostgreSQL not found. Install via: brew install postgresql@17")
}

// Start initializes and starts the embedded PostgreSQL server.
func (pg *Postgres) Start() error {
	if err := os.MkdirAll(pg.DataDir, 0700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	// Set library path for dynamic linking
	if pg.LibDir != "" {
		os.Setenv("DYLD_LIBRARY_PATH", pg.LibDir+":"+os.Getenv("DYLD_LIBRARY_PATH"))
	}

	// Initialize data directory if needed
	versionFile := filepath.Join(pg.DataDir, "PG_VERSION")
	if _, err := os.Stat(versionFile); os.IsNotExist(err) {
		slog.Info("initializing PostgreSQL data directory", "path", pg.DataDir)
		cmd := exec.Command(
			filepath.Join(pg.BinDir, "initdb"),
			"-D", pg.DataDir,
			"-U", pg.User,
			"-A", "trust",
			"--encoding=UTF-8",
			"--locale=C",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("initdb: %w", err)
		}

		// Configure postgresql.conf for our embedded use
		confPath := filepath.Join(pg.DataDir, "postgresql.conf")
		conf, _ := os.ReadFile(confPath)
		extra := fmt.Sprintf(`
# Folio embedded config
port = %d
listen_addresses = 'localhost'
unix_socket_directories = '%s'
shared_buffers = 128MB
work_mem = 16MB
max_connections = 20
log_destination = 'stderr'
logging_collector = off
`, pg.Port, pg.DataDir)
		os.WriteFile(confPath, append(conf, []byte(extra)...), 0600)
	}

	// Start PostgreSQL
	slog.Info("starting PostgreSQL", "port", pg.Port)
	cmd := exec.Command(
		filepath.Join(pg.BinDir, "pg_ctl"),
		"start",
		"-D", pg.DataDir,
		"-o", fmt.Sprintf("-p %d", pg.Port),
		"-l", pg.LogFile,
		"-w", // wait for startup
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_ctl start: %w", err)
	}

	// Wait for it to be ready
	for i := 0; i < 30; i++ {
		cmd := exec.Command(
			filepath.Join(pg.BinDir, "pg_isready"),
			"-h", "localhost",
			"-p", fmt.Sprintf("%d", pg.Port),
			"-U", pg.User,
		)
		if err := cmd.Run(); err == nil {
			slog.Info("PostgreSQL is ready", "port", pg.Port)
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	// Create database if it doesn't exist
	cmd = exec.Command(
		filepath.Join(pg.BinDir, "createdb"),
		"-h", "localhost",
		"-p", fmt.Sprintf("%d", pg.Port),
		"-U", pg.User,
		pg.DBName,
	)
	out, err := cmd.CombinedOutput()
	if err != nil && !strings.Contains(string(out), "already exists") {
		return fmt.Errorf("createdb: %s: %w", string(out), err)
	}

	return nil
}

// Stop gracefully shuts down the PostgreSQL server.
func (pg *Postgres) Stop() error {
	slog.Info("stopping PostgreSQL")
	cmd := exec.Command(
		filepath.Join(pg.BinDir, "pg_ctl"),
		"stop",
		"-D", pg.DataDir,
		"-m", "fast",
		"-w",
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// DSN returns the connection string for this embedded PostgreSQL.
func (pg *Postgres) DSN() string {
	return fmt.Sprintf("postgres://%s@localhost:%d/%s?sslmode=disable",
		pg.User, pg.Port, pg.DBName)
}
