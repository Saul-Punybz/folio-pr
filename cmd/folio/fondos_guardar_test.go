package main

// Guardar fondos y PDF, contra un Postgres embebido (se salta sin binarios).

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/models"
)

func TestGuardarFondoYPDF(t *testing.T) {
	pool := testPG(t)
	ctx := context.Background()
	dataDir := t.TempDir()
	st := newStores(pool, dataDir)

	pdf, err := os.ReadFile("../../internal/pdftext/testdata/dtop_aviso_publico_reglamento.pdf")
	if err != nil {
		t.Fatal(err)
	}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/roto.pdf") {
			http.NotFound(w, r)
			return
		}
		hits.Add(1)
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdf)
	}))
	defer srv.Close()

	u := &models.User{Email: "guardar@folio.local", PasswordHash: "x", Role: "admin"}
	if err := st.users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	sess := &models.Session{ID: "token-guardar", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := st.sessions.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	h := newRouter(st, fstest.MapFS{}, routerHooks{})
	call := func(method, path string) (int, http.Header, []byte) {
		req := httptest.NewRequest(method, path, &bytes.Buffer{})
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, rec.Header(), rec.Body.Bytes()
	}
	newOpp := func(ref, remote string) *models.Opportunity {
		o := &models.Opportunity{Title: "Servicios " + ref, Agency: "AEP", Reference: ref, Category: "rfp",
			SourceURL: "https://subastas.pr.gov/" + ref, Confidence: 60, Level: "estatal", SourceKind: "pagina",
			DocumentRemoteURL: remote}
		o.DedupeKey = crawler.DedupeKey(ref, o.Agency, o.Title, nil, o.SourceURL)
		if _, err := st.opportunities.Upsert(ctx, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	docFile := func(sha string) string { return filepath.Join(dataDir, "documentos", sha+".pdf") }
	cleanup := func(maxBytes string) {
		_, _ = pool.Exec(ctx, `INSERT INTO settings (key, value) VALUES ('documents_max_bytes', $1)
			ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value`, maxBytes)
		crawler.RunDocumentCleanup(ctx, st.crawlDeps, st.settings)
	}

	// 1. Guardar sin archivo local: lo baja y lo marca kept. «Descargar» da attachment.
	a := newOpp("SUBASTA FORMAL 2026-27-53", srv.URL+"/aviso.pdf")
	code, _, body := call("POST", "/api/opportunities/"+a.ID.String()+"/review")
	if code != 200 || !strings.Contains(string(body), `"pdf":"kept"`) || hits.Load() != 1 {
		t.Fatalf("review: %d %s hits=%d", code, body, hits.Load())
	}
	got, _ := st.opportunities.GetByID(ctx, a.ID)
	if !got.Reviewed || !got.DocKept || !got.DocLocal {
		t.Fatalf("tras guardar: reviewed=%v kept=%v local=%v", got.Reviewed, got.DocKept, got.DocLocal)
	}
	_, sha, _ := st.opportunities.DocumentFor(ctx, a.ID)
	if _, err := os.Stat(docFile(sha)); err != nil {
		t.Fatalf("el PDF no quedó en disco: %v", err)
	}
	code, hdr, _ := call("GET", "/api/opportunities/"+a.ID.String()+"/document?download=1")
	if want := `attachment; filename="SUBASTA-FORMAL-2026-27-53.pdf"`; code != 200 || hdr.Get("Content-Disposition") != want {
		t.Fatalf("download: %d %q", code, hdr.Get("Content-Disposition"))
	}
	_, hdr, _ = call("GET", "/api/opportunities/"+a.ID.String()+"/document")
	if !strings.HasPrefix(hdr.Get("Content-Disposition"), "inline;") {
		t.Fatalf("sin download debe ser inline: %q", hdr.Get("Content-Disposition"))
	}

	// 2. Lista de PDF guardados con su espacio.
	code, _, body = call("GET", "/api/documents/kept")
	var kept struct {
		Documents  []models.KeptDoc `json:"documents"`
		KeptBytes  int64            `json:"kept_bytes"`
		TotalBytes int64            `json:"total_bytes"`
	}
	_ = json.Unmarshal(body, &kept)
	if code != 200 || len(kept.Documents) != 1 || kept.KeptBytes != int64(len(pdf)) || kept.TotalBytes < kept.KeptBytes ||
		kept.Documents[0].Reference != a.Reference || !kept.Documents[0].Saved {
		t.Fatalf("kept: %d %s", code, body)
	}

	// 3. La retención no lo borra aunque pase el tope, ni aunque el fondo esté cerrado hace meses.
	_, _ = pool.Exec(ctx, `UPDATE opportunities SET status='closed', deadline = CURRENT_DATE - 200 WHERE id = $1`, a.ID)
	_, _ = pool.Exec(ctx, `UPDATE documents SET created_at = NOW() - interval '30 days'`)
	cleanup("1")
	if _, err := os.Stat(docFile(sha)); err != nil {
		t.Fatalf("la retención borró un PDF guardado: %v", err)
	}

	// 4. Quitar de guardados: vuelve a Por revisar.
	if code, _, body := call("POST", "/api/opportunities/"+a.ID.String()+"/unreview"); code != 200 {
		t.Fatalf("unreview: %d %s", code, body)
	}
	got, _ = st.opportunities.GetByID(ctx, a.ID)
	if got.Reviewed {
		t.Fatal("unreview no devolvió el fondo a Por revisar")
	}
	if code, _, _ := call("POST", "/api/opportunities/"+uuid.NewString()+"/unreview"); code != http.StatusNotFound {
		t.Fatalf("unreview inexistente: %d", code)
	}

	// 5. Aun sin fondo guardado, el PDF kept sobrevive; al soltarlo la retención ya puede borrarlo.
	cleanup("1")
	if _, err := os.Stat(docFile(sha)); err != nil {
		t.Fatalf("el PDF kept se borró: %v", err)
	}
	if code, _, body := call("POST", "/api/opportunities/"+a.ID.String()+"/document/unkeep"); code != 200 {
		t.Fatalf("unkeep: %d %s", code, body)
	}
	cleanup("1")
	if _, err := os.Stat(docFile(sha)); !os.IsNotExist(err) {
		t.Fatalf("tras unkeep la retención debía poder borrarlo (err=%v)", err)
	}

	// 6. «Guardar PDF» suelto: el fondo sigue sin guardar y el PDF se baja y queda kept.
	b := newOpp("RFP 2027-9", srv.URL+"/otro.pdf")
	code, _, body = call("POST", "/api/opportunities/"+b.ID.String()+"/document/keep")
	got, _ = st.opportunities.GetByID(ctx, b.ID)
	if code != 200 || got.Reviewed || !got.DocKept {
		t.Fatalf("keep suelto: %d %s reviewed=%v kept=%v", code, body, got.Reviewed, got.DocKept)
	}

	// 7. Si la descarga falla el fondo se guarda igual y queda el enlace.
	c := newOpp("RFP 2027-10", srv.URL+"/roto.pdf")
	code, _, body = call("POST", "/api/opportunities/"+c.ID.String()+"/review")
	got, _ = st.opportunities.GetByID(ctx, c.ID)
	if code != 200 || !strings.Contains(string(body), `"pdf":"remote"`) || !got.Reviewed || got.DocKept || got.DocLocal {
		t.Fatalf("descarga fallida: %d %s reviewed=%v kept=%v", code, body, got.Reviewed, got.DocKept)
	}
	if got.DocumentURL != srv.URL+"/roto.pdf" {
		t.Fatalf("debía quedar el enlace remoto: %q", got.DocumentURL)
	}
}
