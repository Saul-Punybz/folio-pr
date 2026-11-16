package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/models"
)

// La retención contra Postgres de verdad: qué PDF se queda, cuál se borra, y
// que un fondo cuyo PDF se borró vuelve al enlace remoto.
func TestRetencionDeDocumentos(t *testing.T) {
	pool := testPG(t)
	ctx := context.Background()
	dataDir := t.TempDir()
	st := newStores(pool, dataDir)
	if err := os.MkdirAll(crawler.DocumentsDir(dataDir), 0o700); err != nil {
		t.Fatal(err)
	}

	var domID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM crawl_domains WHERE domain = 'subastas.pr.gov'`).Scan(&domID); err != nil {
		t.Fatal(err)
	}
	// El tope por defecto está sembrado por la migración 028.
	if v, ok, err := st.settings.Get(ctx, "documents_max_bytes"); err != nil || !ok || v != "1073741824" {
		t.Fatalf("documents_max_bytes = %q %v %v", v, ok, err)
	}
	// Los municipios de la 028.
	for _, d := range []string{"www.municipiodebayamon.com", "portalsuplidores.guaynabocity.gov.pr"} {
		var level, start string
		if err := pool.QueryRow(ctx, `SELECT level, start_url FROM crawl_domains WHERE domain = $1 AND active`, d).Scan(&level, &start); err != nil || level != "municipal" || start == "" {
			t.Fatalf("dominio %s: level=%q start=%q err=%v", d, level, start, err)
		}
	}

	newDoc := func(name string, ageDays int) *models.Document {
		rel := filepath.Join("documentos", name+".pdf")
		if err := os.WriteFile(filepath.Join(dataDir, rel), []byte("%PDF-1.4 "+name), 0o600); err != nil {
			t.Fatal(err)
		}
		d := &models.Document{SHA256: name, Path: rel, SizeBytes: 1000, SourceURL: "https://x.pr.gov/" + name + ".pdf"}
		if err := st.crawlDeps.Documents.Upsert(ctx, d); err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE documents SET created_at = NOW() - make_interval(days => $2) WHERE id = $1`, d.ID, ageDays); err != nil {
			t.Fatal(err)
		}
		return d
	}
	opp := func(d *models.Document, ref string, dl time.Time) *models.Opportunity {
		o := &models.Opportunity{
			Title: "Subasta " + ref, Agency: "Municipio de Prueba", Reference: ref, Deadline: &dl,
			Status: models.OpportunityStatus(&dl, time.Now()), Category: "rfp", SourceURL: "https://x.pr.gov/lista",
			Confidence: 50, Level: "municipal", SourceKind: "pdf", DocumentID: &d.ID, DocumentRemoteURL: d.SourceURL,
		}
		o.DedupeKey = crawler.DedupeKey(o.Reference, o.Agency, o.Title, o.Deadline, o.SourceURL)
		if _, err := st.opportunities.Upsert(ctx, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	today := time.Now()
	sinFondo := newDoc("sin-fondo", 5)
	vivo := newDoc("fondo-por-revisar", 5)
	opp(vivo, "A-1", today.AddDate(0, 0, 20))
	cerrado100 := newDoc("fondo-cerrado-100d", 200)
	o100 := opp(cerrado100, "B-2", today.AddDate(0, 0, -100))
	cerrado10 := newDoc("fondo-cerrado-10d", 200)
	opp(cerrado10, "C-3", today.AddDate(0, 0, -10))
	reciente := newDoc("recien-bajado", 0)
	pendiente := newDoc("esperando-extraccion", 5)
	if _, err := pool.Exec(ctx, `
		INSERT INTO crawled_pages (url, url_hash, domain_id, document_id, opportunity_checked)
		VALUES ('https://x.pr.gov/p.pdf', 'hash-pendiente', $1, $2, false)`, domID, pendiente.ID); err != nil {
		t.Fatal(err)
	}
	// Un archivo suelto viejo y uno suelto de hoy.
	huerfano := filepath.Join(dataDir, "documentos", "huerfano.pdf")
	nuevoSuelto := filepath.Join(dataDir, "documentos", "suelto-de-hoy.pdf")
	for _, p := range []string{huerfano, nuevoSuelto} {
		if err := os.WriteFile(p, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	old := time.Now().Add(-48 * time.Hour)
	_ = os.Chtimes(huerfano, old, old)

	stats := crawler.RunDocumentCleanup(ctx, st.crawlDeps, st.settings)
	if stats.Deleted != 2 || stats.Orphans != 1 {
		t.Fatalf("stats = %+v (esperaba 2 borrados y 1 huérfano)", stats)
	}
	exists := func(d *models.Document) bool {
		_, err := os.Stat(filepath.Join(dataDir, d.Path))
		return err == nil
	}
	for name, want := range map[string]struct {
		d    *models.Document
		kept bool
	}{
		"sin fondo":            {sinFondo, false},
		"fondo por revisar":    {vivo, true},
		"cerrado hace 100 d":   {cerrado100, false},
		"cerrado hace 10 d":    {cerrado10, true},
		"recién bajado":        {reciente, true},
		"esperando extracción": {pendiente, true},
	} {
		if exists(want.d) != want.kept {
			t.Errorf("%s: archivo existe=%v, quería %v", name, exists(want.d), want.kept)
		}
	}
	if _, err := os.Stat(huerfano); err == nil {
		t.Error("el archivo huérfano viejo siguió ahí")
	}
	if _, err := os.Stat(nuevoSuelto); err != nil {
		t.Error("el archivo suelto de hoy no debía borrarse")
	}

	// Las filas y el texto siguen; el fondo cerrado vuelve al enlace remoto.
	var rows int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM documents`).Scan(&rows); err != nil || rows != 6 {
		t.Fatalf("filas de documents = %d (%v), ninguna debía borrarse", rows, err)
	}
	got, err := st.opportunities.GetByID(ctx, o100.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.DocumentURL != "https://x.pr.gov/fondo-cerrado-100d.pdf" {
		t.Errorf("document_url = %q, quería el enlace remoto", got.DocumentURL)
	}
	// Y la segunda pasada no hace nada.
	if again := crawler.RunDocumentCleanup(ctx, st.crawlDeps, st.settings); again.Deleted != 0 || again.Orphans != 0 {
		t.Errorf("segunda pasada: %+v", again)
	}
}
