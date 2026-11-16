package crawler

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
)

type fakeSettings map[string]string

func (f fakeSettings) Get(_ context.Context, k string) (string, bool, error) {
	v, ok := f[k]
	return v, ok, nil
}

func TestPlanRetentionSinTope(t *testing.T) {
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, ast)
	old := now.AddDate(0, 0, -5)
	mk := func(name string, active, pending bool, age time.Time, size int64) models.DocState {
		return models.DocState{ID: uuid.NewSHA1(uuid.NameSpaceURL, []byte(name)), Path: "documentos/" + name + ".pdf",
			SizeBytes: size, CreatedAt: age, HasActive: active, Pending: pending}
	}
	id := func(n string) uuid.UUID { return uuid.NewSHA1(uuid.NameSpaceURL, []byte(n)) }
	docs := []models.DocState{
		mk("sin-fondo", false, false, old, 5<<20),                            // no produjo fondo: se borra
		mk("fondo-vivo", true, false, old, 5<<20),                            // por revisar/confirmado: se queda
		mk("recien-bajado", false, false, now.Add(-time.Minute), 5<<20),      // la extracción aún no lo ve
		mk("esperando-extraccion", false, true, old, 5<<20),                  // pendiente de extraer
		{ID: id("ya-borrado"), Path: "", SizeBytes: 9 << 20, CreatedAt: old}, // sin archivo: nada que hacer
	}
	got := PlanRetention(docs, now, 1<<30)
	if len(got) != 1 || got[0] != id("sin-fondo") {
		t.Fatalf("borrar = %v, solo debía ser sin-fondo", got)
	}
}

func TestPlanRetentionTopeTotal(t *testing.T) {
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, ast)
	id := func(n string) uuid.UUID { return uuid.NewSHA1(uuid.NameSpaceURL, []byte(n)) }
	mk := func(name string, active, pending bool, daysAgo int, mb int64) models.DocState {
		return models.DocState{ID: id(name), Path: "documentos/" + name + ".pdf", SizeBytes: mb << 20,
			CreatedAt: now.AddDate(0, 0, -daysAgo), HasActive: active, Pending: pending}
	}
	docs := []models.DocState{
		mk("fondo-viejo", true, false, 60, 40),
		mk("pend-viejo", false, true, 30, 30), // sin fondo todavía, pero espera extracción
		mk("pend-nuevo", false, true, 2, 30),
		mk("fondo-nuevo", true, false, 1, 40),
	}
	// 140 MB con tope de 100: sale primero lo más viejo SIN fondo vivo; los
	// fondos vivos no se tocan aunque el total siga pasado.
	got := PlanRetention(docs, now, 100<<20)
	if len(got) != 2 || got[0] != id("pend-viejo") || got[1] != id("pend-nuevo") {
		t.Fatalf("borrar = %v", got)
	}
	// Con tope de 1 MB y solo fondos vivos no se borra nada.
	got = PlanRetention([]models.DocState{mk("a", true, false, 9, 50), mk("b", true, false, 1, 50)}, now, 1<<20)
	if len(got) != 0 {
		t.Fatalf("un fondo vivo se iba a borrar: %v", got)
	}
}

func TestDocumentsMaxBytes(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		s    SettingsReader
		want int64
	}{
		{nil, DefaultDocumentsMaxBytes},
		{fakeSettings{}, DefaultDocumentsMaxBytes},
		{fakeSettings{"documents_max_bytes": "2147483648"}, 2 << 30},
		{fakeSettings{"documents_max_bytes": "basura"}, DefaultDocumentsMaxBytes},
		{fakeSettings{"documents_max_bytes": "-5"}, DefaultDocumentsMaxBytes},
	} {
		if got := DocumentsMaxBytes(ctx, tc.s); got != tc.want {
			t.Errorf("DocumentsMaxBytes = %d, quería %d", got, tc.want)
		}
	}
}

func TestPlanRetentionNuncaBorraGuardados(t *testing.T) {
	now := time.Date(2026, 11, 16, 9, 0, 0, 0, ast)
	id := func(n string) uuid.UUID { return uuid.NewSHA1(uuid.NameSpaceURL, []byte(n)) }
	mk := func(name string, kept, saved bool, mb int64) models.DocState {
		return models.DocState{ID: id(name), Path: "documentos/" + name + ".pdf", SizeBytes: mb << 20,
			CreatedAt: now.AddDate(0, 0, -300), Kept: kept, Saved: saved}
	}
	docs := []models.DocState{
		mk("kept-suelto", true, false, 50),            // PDF guardado sin fondo vivo
		mk("fondo-guardado-cerrado", false, true, 50), // fondo guardado, cerrado hace meses
		mk("suelto", false, false, 50),                // nada lo protege
	}
	// Con y sin tope: solo cae el que nada protege, y los guardados no cuentan para el tope.
	for _, cap := range []int64{1 << 30, 60 << 20, 1} {
		got := PlanRetention(docs, now, cap)
		if len(got) != 1 || got[0] != id("suelto") {
			t.Fatalf("tope %d: borrar = %v, solo debía ser suelto", cap, got)
		}
	}
	// Soltado el PDF (Kept=false) y sin fondo, vuelve a quedar expuesto.
	docs[0].Kept = false
	if got := PlanRetention(docs[:1], now, 1<<30); len(got) != 1 {
		t.Fatalf("un PDF soltado debía quedar expuesto: %v", got)
	}
}
