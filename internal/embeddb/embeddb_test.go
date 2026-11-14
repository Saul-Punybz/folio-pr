package embeddb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestPruneKeepsNewest(t *testing.T) {
	dir := t.TempDir()
	for d := 1; d <= 20; d++ {
		name := filepath.Join(dir, fmt.Sprintf("folio-2026-01-%02d.sql.gz", d))
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := prune(dir, 14); err != nil {
		t.Fatal(err)
	}
	left, _ := backups(dir)
	if len(left) != 14 {
		t.Fatalf("quedaron %d, se esperaban 14", len(left))
	}
	if filepath.Base(left[0]) != "folio-2026-01-07.sql.gz" {
		t.Fatalf("el más viejo debería ser el 07, es %s", filepath.Base(left[0]))
	}
}

func TestRecoverStaleRemovesDeadPID(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "pgdata")
	if err := os.MkdirAll(data, 0o700); err != nil {
		t.Fatal(err)
	}
	pidFile := filepath.Join(data, "postmaster.pid")
	// PID casi seguro inexistente.
	if err := os.WriteFile(pidFile, []byte("999999\n/x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := recoverStale(Options{Root: root, BinariesPath: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatal("postmaster.pid huérfano no se borró")
	}
}

func TestRecoverStaleIgnoresNonPostgresPID(t *testing.T) {
	root := t.TempDir()
	data := filepath.Join(root, "pgdata")
	_ = os.MkdirAll(data, 0o700)
	pidFile := filepath.Join(data, "postmaster.pid")
	// El PID de la propia prueba: vivo, pero no es postgres.
	_ = os.WriteFile(pidFile, []byte(fmt.Sprintf("%d\n", os.Getpid())), 0o600)
	if err := recoverStale(Options{Root: root, BinariesPath: root}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatal("postmaster.pid con PID reciclado no se borró")
	}
}
