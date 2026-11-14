// Package embeddb arranca y para el Postgres que va dentro de Folio.app.
//
// Usa github.com/fergusstrange/embedded-postgres con los binarios ya
// extraídos dentro del .app (BinariesPath): nunca descarga nada. Los datos
// viven en <Root>/pgdata; el socket y archivos temporales en <Root>/run.
package embeddb

import (
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"

	"github.com/Saul-Punybz/folio/internal/config"
)

// Version es la versión de los binarios empaquetados (zonky/EDB).
const Version = embeddedpostgres.V18

const (
	dbName = "folio"
	dbUser = "folio"
)

// Options configura la instancia.
type Options struct {
	Root         string // carpeta de datos de Folio (…/Folio2)
	BinariesPath string // carpeta con bin/ lib/ share/ de Postgres
}

// Instance es un Postgres embebido en marcha.
type Instance struct {
	opts     Options
	port     uint32
	password string
	pg       *embeddedpostgres.EmbeddedPostgres
	logFile  *os.File
}

func (o Options) dataDir() string { return filepath.Join(o.Root, "pgdata") }
func (o Options) runDir() string  { return filepath.Join(o.Root, "run") }
func (o Options) logDir() string  { return filepath.Join(o.Root, "logs") }

// Start recupera un cierre abrupto anterior si hace falta, elige un puerto
// libre en 127.0.0.1 y arranca Postgres.
func Start(opts Options) (*Instance, error) {
	if _, err := os.Stat(filepath.Join(opts.BinariesPath, "bin", "pg_ctl")); err != nil {
		// Sin esto la librería intentaría bajar los binarios de Maven.
		return nil, fmt.Errorf("binarios de Postgres no encontrados en %s", opts.BinariesPath)
	}
	for _, d := range []string{opts.Root, opts.logDir()} {
		if err := os.MkdirAll(d, 0o700); err != nil {
			return nil, err
		}
	}

	if err := recoverStale(opts); err != nil {
		return nil, err
	}

	password, err := loadOrCreateSecret(filepath.Join(opts.Root, "db.secret"))
	if err != nil {
		return nil, err
	}
	port, err := freePort()
	if err != nil {
		return nil, err
	}

	logFile, err := os.OpenFile(filepath.Join(opts.logDir(), "postgres-arranque.log"),
		os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, err
	}

	params := map[string]string{
		"listen_addresses":         "127.0.0.1",
		"logging_collector":        "on",
		"log_directory":            opts.logDir(),
		"log_filename":             "postgres-%a.log",
		"log_truncate_on_rotation": "on",
		"log_rotation_age":         "1d",
		"log_rotation_size":        "0",
	}
	// Socket Unix dentro de la carpeta de datos. macOS limita la ruta a 103
	// bytes; si no cabe se apaga el socket y queda solo TCP en 127.0.0.1.
	sock := filepath.Join(opts.runDir(), ".s.PGSQL."+strconv.Itoa(int(port)))
	if len(sock) < 100 {
		params["unix_socket_directories"] = opts.runDir()
	} else {
		params["unix_socket_directories"] = ""
		slog.Warn("embeddb: socket path too long, unix socket disabled", "path", sock)
	}

	cfg := embeddedpostgres.DefaultConfig().
		Version(Version).
		Port(port).
		Database(dbName).
		Username(dbUser).
		Password(password).
		BinariesPath(opts.BinariesPath).
		RuntimePath(opts.runDir()). // la librería la borra al arrancar
		CachePath(filepath.Join(opts.runDir(), "cache")).
		DataPath(opts.dataDir()).
		Locale("en_US.UTF-8").
		Encoding("UTF8").
		StartParameters(params).
		StartTimeout(60 * time.Second).
		Logger(logFile)

	pg := embeddedpostgres.NewDatabase(cfg)
	if err := pg.Start(); err != nil {
		logFile.Close()
		return nil, fmt.Errorf("postgres: %w", err)
	}
	slog.Info("embeddb: postgres started", "port", port, "data", opts.dataDir())
	return &Instance{opts: opts, port: port, password: password, pg: pg, logFile: logFile}, nil
}

// DB devuelve la configuración de conexión para internal/db.
func (i *Instance) DB() config.DBConfig {
	return config.DBConfig{
		Host: "127.0.0.1", Port: int(i.port), User: dbUser, Pass: i.password,
		DBName: dbName, SSLMode: "disable",
	}
}

// Port es el puerto TCP de Postgres.
func (i *Instance) Port() uint32 { return i.port }

// Stop para Postgres (pg_ctl stop -m fast -w).
func (i *Instance) Stop() error {
	err := i.pg.Stop()
	i.logFile.Close()
	if err == nil {
		slog.Info("embeddb: postgres stopped")
	}
	return err
}

// ── Recuperación tras un cierre abrupto ────────────────────────────

// recoverStale mira pgdata/postmaster.pid. Si el postgres de un Folio
// anterior sigue vivo (el proceso principal murió con kill -9 o un crash),
// lo para; si el PID ya no existe o es de otro programa, borra el archivo.
func recoverStale(opts Options) error {
	pidFile := filepath.Join(opts.dataDir(), "postmaster.pid")
	data, err := os.ReadFile(pidFile)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	lines := strings.SplitN(string(data), "\n", 2)
	pid, err := strconv.Atoi(strings.TrimSpace(lines[0]))
	if err != nil || pid <= 0 {
		slog.Warn("embeddb: unreadable postmaster.pid, removing")
		return os.Remove(pidFile)
	}

	if !isOurPostgres(pid) {
		slog.Warn("embeddb: stale postmaster.pid (process gone), removing", "pid", pid)
		return os.Remove(pidFile)
	}

	slog.Warn("embeddb: postgres from a previous run still alive, stopping it", "pid", pid)
	pgCtl := filepath.Join(opts.BinariesPath, "bin", "pg_ctl")
	cmd := exec.Command(pgCtl, "stop", "-D", opts.dataDir(), "-m", "fast", "-w", "-t", "30")
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Warn("embeddb: pg_ctl stop failed, signalling", "err", err, "out", strings.TrimSpace(string(out)))
		_ = syscall.Kill(pid, syscall.SIGQUIT) // immediate shutdown
		deadline := time.Now().Add(15 * time.Second)
		for isAlive(pid) && time.Now().Before(deadline) {
			time.Sleep(200 * time.Millisecond)
		}
		if isAlive(pid) {
			_ = syscall.Kill(pid, syscall.SIGKILL)
			time.Sleep(time.Second)
		}
		_ = os.Remove(pidFile)
	}
	return nil
}

func isAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}

// isOurPostgres: el PID vive y es un proceso "postgres" (no un PID reciclado).
func isOurPostgres(pid int) bool {
	if !isAlive(pid) {
		return false
	}
	out, err := exec.Command("/bin/ps", "-p", strconv.Itoa(pid), "-o", "comm=").Output()
	if err != nil {
		return false
	}
	return filepath.Base(strings.TrimSpace(string(out))) == "postgres"
}

func freePort() (uint32, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("puerto libre para postgres: %w", err)
	}
	defer l.Close()
	return uint32(l.Addr().(*net.TCPAddr).Port), nil
}

// loadOrCreateSecret: contraseña aleatoria de la base, creada una sola vez.
// Debe existir antes del initdb y no cambiar después.
func loadOrCreateSecret(path string) (string, error) {
	if b, err := os.ReadFile(path); err == nil {
		if s := strings.TrimSpace(string(b)); s != "" {
			return s, nil
		}
	}
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	s := hex.EncodeToString(buf)
	if err := os.WriteFile(path, []byte(s+"\n"), 0o600); err != nil {
		return "", err
	}
	return s, nil
}

// ── Respaldos ───────────────────────────────────────────────────────

// BackupDir es la carpeta de respaldos.
func (i *Instance) BackupDir() string { return filepath.Join(i.opts.Root, "respaldos") }

// BackupIfDue hace pg_dump a respaldos/folio-AAAA-MM-DD.sql.gz si el último
// respaldo tiene más de maxAge, y conserva solo los últimos keep.
// Devuelve la ruta creada ("" si no tocaba).
func (i *Instance) BackupIfDue(maxAge time.Duration, keep int, now time.Time) (string, error) {
	dir := i.BackupDir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	files, err := backups(dir)
	if err != nil {
		return "", err
	}
	if len(files) > 0 {
		last := files[len(files)-1]
		if info, err := os.Stat(last); err == nil && now.Sub(info.ModTime()) < maxAge {
			return "", nil
		}
	}

	dest := filepath.Join(dir, "folio-"+now.Format("2006-01-02")+".sql.gz")
	if err := i.dump(dest); err != nil {
		return "", err
	}
	if err := prune(dir, keep); err != nil {
		slog.Warn("backup: prune", "err", err)
	}
	return dest, nil
}

// backups lista respaldos ordenados del más viejo al más nuevo (por nombre).
func backups(dir string) ([]string, error) {
	m, err := filepath.Glob(filepath.Join(dir, "folio-*.sql.gz"))
	if err != nil {
		return nil, err
	}
	sort.Strings(m)
	return m, nil
}

func prune(dir string, keep int) error {
	files, err := backups(dir)
	if err != nil {
		return err
	}
	for len(files) > keep {
		if err := os.Remove(files[0]); err != nil {
			return err
		}
		files = files[1:]
	}
	return nil
}

func (i *Instance) dump(dest string) error {
	tmp := dest + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer os.Remove(tmp) // no-op tras el rename

	gz := gzip.NewWriter(f)
	cmd := exec.Command(filepath.Join(i.opts.BinariesPath, "bin", "pg_dump"),
		"-h", "127.0.0.1", "-p", strconv.Itoa(int(i.port)), "-U", dbUser,
		"--no-owner", "--no-privileges", dbName)
	cmd.Env = []string{"PGPASSWORD=" + i.password, "PATH=/usr/bin:/bin"}
	cmd.Stdout = gz
	var stderr strings.Builder
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if err := gz.Close(); err != nil && runErr == nil {
		runErr = err
	}
	if err := f.Close(); err != nil && runErr == nil {
		runErr = err
	}
	if runErr != nil {
		return fmt.Errorf("pg_dump: %w: %s", runErr, strings.TrimSpace(stderr.String()))
	}
	return os.Rename(tmp, dest)
}
