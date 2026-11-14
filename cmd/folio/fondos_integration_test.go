package main

// Pruebas contra un Postgres embebido de verdad (en una carpeta temporal).
// Necesitan los binarios de Postgres: FOLIO_PG_BIN, o los de build/Folio.app
// (bash scripts/build-macos.sh). Sin ellos se saltan.

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/embeddb"
	"github.com/Saul-Punybz/folio/internal/models"
)

func testPG(t *testing.T) *pgxpool.Pool {
	t.Helper()
	bin := os.Getenv("FOLIO_PG_BIN")
	if bin == "" {
		bin, _ = filepath.Abs("../../build/Folio.app/Contents/Resources/pg")
	}
	if _, err := os.Stat(filepath.Join(bin, "bin", "pg_ctl")); err != nil {
		t.Skip("sin binarios de Postgres (FOLIO_PG_BIN o build/Folio.app)")
	}
	root, err := os.MkdirTemp("", "folio-it-")
	if err != nil {
		t.Fatal(err)
	}
	pg, err := embeddb.Start(embeddb.Options{Root: root, BinariesPath: bin})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := connect(ctx, pg.DB())
	if err != nil {
		_ = pg.Stop()
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		if err := pg.Stop(); err != nil {
			t.Errorf("parar postgres: %v", err)
		}
		os.RemoveAll(root)
	})
	return pool
}

func TestFondosDescartadoYPut(t *testing.T) {
	pool := testPG(t)
	ctx := context.Background()
	st := newStores(pool, t.TempDir())
	opps := st.opportunities

	// Sesión de administradora para la API.
	u := &models.User{Email: "prueba@folio.local", PasswordHash: "x", Role: "admin"}
	if err := st.users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	sess := &models.Session{ID: "token-de-prueba", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := st.sessions.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	h := newRouter(st, fstest.MapFS{}, routerHooks{})
	call := func(method, path string, body any) (int, []byte) {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code, rec.Body.Bytes()
	}

	dl := time.Date(2027, 1, 14, 23, 59, 0, 0, time.FixedZone("AST", -4*3600))
	newOpp := func(confidence int) *models.Opportunity {
		o := &models.Opportunity{
			Title: "Construction of first responders safe room", Agency: "Autoridad de Edificios Públicos (AEP)",
			Reference: "SUBASTA FORMAL 2026-27-53", Amount: "$250,000", AmountCents: 25_000_000,
			Deadline: &dl, Category: "rfp", SourceURL: "https://subastas.pr.gov/Pages/aviso.aspx?itemID=1",
			Confidence: confidence, Level: "estatal", SourceKind: "pagina", ContactPhone: "787-722-0590",
			Evidence: map[string]string{"reference": "SUBASTA FORMAL NÚM.2026-27-53"},
		}
		o.DedupeKey = crawler.DedupeKey(o.Reference, o.Agency, o.Title, o.Deadline, o.SourceURL)
		return o
	}

	// 1. La misma convocatoria vista dos veces (otra URL) = una fila.
	a := newOpp(60)
	if created, err := opps.Upsert(ctx, a); err != nil || !created {
		t.Fatalf("primer upsert: created=%v err=%v", created, err)
	}
	b := newOpp(70)
	b.SourceURL = "https://subastas.pr.gov/Pages/subastas.aspx"
	if created, err := opps.Upsert(ctx, b); err != nil || created {
		t.Fatalf("segundo upsert debió actualizar: created=%v err=%v", created, err)
	}
	if b.ID != a.ID {
		t.Fatalf("la misma huella creó dos filas: %s y %s", a.ID, b.ID)
	}

	// 2. PUT que vacía el monto, la referencia y la fecha (claves presentes,
	//    vacías o null) y no toca lo que no viene.
	code, body := call("PUT", "/api/opportunities/"+a.ID.String(), map[string]any{
		"amount": "", "reference": "", "deadline": nil, "contact_phone": "787-000-1111",
	})
	if code != http.StatusOK {
		t.Fatalf("PUT: %d %s", code, body)
	}
	got, err := opps.GetByID(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Amount != "" || got.AmountCents != 0 || got.Reference != "" || got.Deadline != nil {
		t.Fatalf("PUT no vació: amount=%q cents=%d ref=%q deadline=%v", got.Amount, got.AmountCents, got.Reference, got.Deadline)
	}
	if got.ContactPhone != "787-000-1111" || got.Title != a.Title || got.Status != "unknown" {
		t.Fatalf("PUT tocó de más: phone=%q title=%q status=%q", got.ContactPhone, got.Title, got.Status)
	}
	code, _ = call("PUT", "/api/opportunities/"+a.ID.String(), map[string]any{"deadline": "14-01-2027"})
	if code != http.StatusBadRequest {
		t.Fatalf("fecha mal escrita: %d, quería 400", code)
	}

	// 3. El siguiente crawl no pisa la corrección manual.
	c := newOpp(90)
	if _, err := opps.Upsert(ctx, c); err != nil {
		t.Fatal(err)
	}
	got, _ = opps.GetByID(ctx, a.ID)
	if got.Amount != "" || got.Reference != "" || got.Deadline != nil || got.ContactPhone != "787-000-1111" {
		t.Fatalf("el crawl pisó la corrección: amount=%q ref=%q deadline=%v phone=%q",
			got.Amount, got.Reference, got.Deadline, got.ContactPhone)
	}

	// 4. Descartar: DELETE guarda la huella; el próximo crawl no lo recrea.
	if code, body := call("DELETE", "/api/opportunities/"+a.ID.String(), nil); code != http.StatusOK {
		t.Fatalf("DELETE: %d %s", code, body)
	}
	if _, err := opps.Upsert(ctx, newOpp(95)); !errors.Is(err, models.ErrDismissed) {
		t.Fatalf("el descartado volvió: err=%v", err)
	}
	n, _ := opps.Count(ctx, models.ListFilter{})
	if n != 0 {
		t.Fatalf("quedan %d fondos, quería 0", n)
	}

	// 5. Lista y stats con reviewed/pending.
	d := newOpp(50)
	d.Reference, d.DedupeKey = "RFP 2027-9", crawler.DedupeKey("RFP 2027-9", "AEP", "", nil, "")
	if _, err := opps.Upsert(ctx, d); err != nil {
		t.Fatal(err)
	}
	code, body = call("GET", "/api/opportunities/stats", nil)
	var stats struct {
		Pending int `json:"pending"`
	}
	_ = json.Unmarshal(body, &stats)
	if code != http.StatusOK || stats.Pending != 1 {
		t.Fatalf("stats: %d %s", code, body)
	}
	if code, body := call("POST", "/api/opportunities/"+d.ID.String()+"/review", nil); code != http.StatusOK {
		t.Fatalf("review: %d %s", code, body)
	}
	code, body = call("GET", "/api/opportunities?reviewed=false", nil)
	var list struct {
		Count int `json:"count"`
	}
	_ = json.Unmarshal(body, &list)
	if code != http.StatusOK || list.Count != 0 {
		t.Fatalf("reviewed=false tras confirmar: %d %s", code, body)
	}
	code, _ = call("GET", "/no-existe", nil)
	if code != http.StatusNotFound {
		t.Fatalf("ruta inexistente: %d, quería 404", code)
	}
	// Logout sin sesión: 200 y la cookie limpia.
	req := httptest.NewRequest("POST", "/api/logout", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || rec.Header().Get("Set-Cookie") == "" {
		t.Fatalf("logout sin sesión: %d cookie=%q", rec.Code, rec.Header().Get("Set-Cookie"))
	}
}
