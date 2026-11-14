package main

// Modo app: Folio en la Mac de una sola persona, sin nada instalado.
//
//	Folio.app (doble clic) → launch(): si ya corre, abre el navegador; si no,
//	                         arranca `Folio app` en segundo plano y sale.
//	folio app              → runApp(): Postgres embebido → migraciones →
//	                         HTTP en 127.0.0.1:47615 → navegador por defecto.
//
// El lanzador existe porque macOS, al hacer doble clic en una app que ya está
// abierta, solo la "activa" (no lanza otro proceso); un binario sin Cocoa no
// puede responder a eso. Con el lanzador efímero cada doble clic abre Folio.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/Saul-Punybz/folio/internal/embeddb"
)

const (
	appAddr = "127.0.0.1:47615"
	appURL  = "http://" + appAddr + "/"

	backupMaxAge = 20 * time.Hour
	backupKeep   = 14
)

// appRoot: ~/Library/Application Support/Folio2 (carpeta nueva; la vieja
// "Folio" no se toca).
func appRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return "", fmt.Errorf("sin HOME: %w", err)
	}
	return filepath.Join(home, "Library", "Application Support", "Folio2"), nil
}

func executablePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		return real
	}
	return exe
}

func insideAppBundle() bool {
	return strings.Contains(executablePath(), ".app/Contents/MacOS/")
}

// pgBinaries: FOLIO_PG_BIN, o Contents/Resources/pg dentro del .app.
func pgBinaries() string {
	if p := os.Getenv("FOLIO_PG_BIN"); p != "" {
		return p
	}
	return filepath.Join(filepath.Dir(executablePath()), "..", "Resources", "pg")
}

// noGUI: FOLIO_NO_BROWSER=1 (pruebas) no abre navegador ni diálogos.
func noGUI() bool { return os.Getenv("FOLIO_NO_BROWSER") != "" }

func openBrowser() {
	if noGUI() {
		slog.Info("app: FOLIO_NO_BROWSER set, not opening browser", "url", appURL)
		return
	}
	if err := exec.Command("/usr/bin/open", appURL).Run(); err != nil {
		slog.Error("app: open browser", "err", err)
	}
}

// alert muestra un diálogo de macOS (osascript viene con el sistema).
func alert(msg string) {
	if noGUI() {
		return
	}
	script := fmt.Sprintf(`display alert "Folio no pudo abrir" message %q as critical`, msg)
	_ = exec.Command("/usr/bin/osascript", "-e", script).Run()
}

// lockInstance toma el candado de instancia única (flock: se suelta solo si
// el proceso muere, incluso con kill -9). ok=false si otro Folio lo tiene.
func lockInstance(root string) (f *os.File, ok bool, err error) {
	f, err = os.OpenFile(filepath.Join(root, "folio.lock"), os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, false, nil
		}
		return nil, false, err
	}
	_ = f.Truncate(0)
	_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	return f, true, nil
}

// openLog: logs/folio.log (se rota a folio.log.1 al pasar de 10 MB).
func openLog(logDir string) (*os.File, error) {
	p := filepath.Join(logDir, "folio.log")
	if info, err := os.Stat(p); err == nil && info.Size() > 10<<20 {
		_ = os.Rename(p, p+".1")
	}
	return os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
}

// launch es lo que corre el doble clic.
func launch() error {
	root, err := appRoot()
	if err != nil {
		alert(err.Error())
		return err
	}
	if err := os.MkdirAll(filepath.Join(root, "logs"), 0o700); err != nil {
		alert(err.Error())
		return err
	}
	f, ok, err := lockInstance(root)
	if err != nil {
		alert(err.Error())
		return err
	}
	if !ok {
		// Ya hay un Folio abierto: abrirlo en el navegador.
		openBrowser()
		return nil
	}
	f.Close() // lo toma el proceso de fondo

	cmd := exec.Command(executablePath(), "app")
	cmd.Stdin, cmd.Stdout, cmd.Stderr = nil, nil, nil // su log va a logs/folio.log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		alert("No se pudo arrancar Folio: " + err.Error())
		return err
	}
	exited := make(chan struct{})
	go func() { _ = cmd.Wait(); close(exited) }()

	// Esperar a que responda (el de fondo abre el navegador) o a que falle
	// (el de fondo ya mostró su diálogo).
	deadline := time.After(3 * time.Minute)
	tick := time.NewTicker(500 * time.Millisecond)
	defer tick.Stop()
	for {
		select {
		case <-exited:
			return errors.New("folio app terminó al arrancar; ver logs/folio.log")
		case <-deadline:
			return nil
		case <-tick.C:
			if healthy() {
				return nil
			}
		}
	}
}

func healthy() bool {
	c := http.Client{Timeout: time.Second}
	resp, err := c.Get(appURL + "api/health")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// runApp: `folio app`, en primer plano.
func runApp() (retErr error) {
	root, err := appRoot()
	if err != nil {
		return err
	}
	for _, d := range []string{"logs", "respaldos"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o700); err != nil {
			return err
		}
	}

	logFile, err := openLog(filepath.Join(root, "logs"))
	if err == nil {
		defer logFile.Close()
		slog.SetDefault(slog.New(slog.NewTextHandler(io.MultiWriter(os.Stdout, logFile),
			&slog.HandlerOptions{Level: slog.LevelInfo})))
	}
	defer func() {
		if retErr != nil {
			slog.Error("app: fatal", "err", retErr)
			alert(retErr.Error())
		}
	}()

	lock, ok, err := lockInstance(root)
	if err != nil {
		return fmt.Errorf("candado de instancia: %w", err)
	}
	if !ok {
		slog.Info("app: another Folio is already running, opening it")
		openBrowser()
		return nil
	}
	defer lock.Close()

	// El puerto HTTP primero: si está ocupado no tiene sentido arrancar nada.
	ln, err := net.Listen("tcp", appAddr)
	if err != nil {
		return fmt.Errorf("el puerto 47615 de 127.0.0.1 lo está usando otro programa (%v). Cierra ese programa y vuelve a abrir Folio", err)
	}

	pg, err := embeddb.Start(embeddb.Options{Root: root, BinariesPath: pgBinaries()})
	if err != nil {
		ln.Close()
		return fmt.Errorf("no arrancó la base de datos: %w", err)
	}
	defer func() {
		if err := pg.Stop(); err != nil {
			slog.Error("app: stop postgres", "err", err)
		}
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer stop()

	backup := func(context.Context) {
		path, err := pg.BackupIfDue(backupMaxAge, backupKeep, time.Now())
		switch {
		case err != nil:
			slog.Error("backup: failed", "err", err)
		case path != "":
			slog.Info("backup: done", "file", path)
		default:
			slog.Info("backup: not due")
		}
	}

	return runServer(ctx, serverOptions{
		db:       pg.DB(),
		listener: ln,
		cron: cronOptions{
			catchUp: true,
			extra: []extraJob{
				// Al arrancar y luego cada día a mediodía (si lleva días abierta).
				{spec: "0 12 * * *", name: "backup", timeout: 30 * time.Minute, fn: backup, atStartup: true},
			},
		},
		onReady: openBrowser,
		dataDir: root,
	})
}
