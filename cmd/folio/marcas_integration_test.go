package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/watch"
)

// Siembra de marcas idempotente, sin pisar ediciones ni resucitar borradas,
// tope de 10 marcas activas y deduplicado de menciones.
func TestMarcasSiembraYTope(t *testing.T) {
	pool := testPG(t)
	ctx := context.Background()
	st := newStores(pool, t.TempDir())

	// Sin admin, la siembra no hace nada.
	if n, err := st.watchOrgs.SeedTemplates(ctx, watch.BrandTemplates()); err != nil || n != 0 {
		t.Fatalf("sin admin: n=%d err=%v", n, err)
	}

	u := &models.User{Email: "marcas@folio.local", PasswordHash: "x", Role: "admin"}
	if err := st.users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	sess := &models.Session{ID: "token-marcas", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}
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
	list := func() []models.WatchlistOrg {
		code, body := call("GET", "/api/watchlist/orgs", nil)
		if code != 200 {
			t.Fatalf("GET orgs: %d %s", code, body)
		}
		var r struct {
			Orgs []models.WatchlistOrg `json:"orgs"`
		}
		if err := json.Unmarshal(body, &r); err != nil {
			t.Fatal(err)
		}
		return r.Orgs
	}

	// 1. El GET siembra las 4 marcas una vez.
	orgs := list()
	if len(orgs) != 4 {
		t.Fatalf("marcas sembradas = %d, want 4", len(orgs))
	}
	byName := map[string]models.WatchlistOrg{}
	for _, o := range orgs {
		if o.Kind != models.KindMarca || !o.Active || o.ToConfirm || len(o.SourcesEvidence) == 0 {
			t.Errorf("marca mal sembrada: %+v", o)
		}
		byName[o.Name] = o
	}
	if psi := byName["Platform for Social Impact (PSI)"]; len(psi.ContextAliases) == 0 || len(psi.People) != 1 || len(psi.Aliases) == 0 {
		t.Fatalf("PSI sin alias con contexto o sin personas: %+v", psi)
	}
	active, err := st.watchOrgs.ListActive(ctx)
	if err != nil || len(active) != 4 || len(active[0].Hashtags)+len(active[0].ContextAliases) == 0 {
		t.Fatalf("ListActive: %v %+v", err, active)
	}
	vim := byName["Vimenti"]
	if len(vim.Socials) == 0 || len(vim.Hashtags) == 0 || len(vim.Aliases) == 0 {
		t.Fatalf("Vimenti sin datos: %+v", vim)
	}

	// 2. La usuaria edita Vimenti y borra Oasis 360: la siembra no pisa ni resucita.
	code, body := call("PUT", "/api/watchlist/orgs/"+vim.ID.String(), map[string]any{
		"name": "Vimenti", "kind": "marca", "website": vim.Website, "aliases": []string{"Vimenti"},
		"hashtags": []string{"#VimentiPR"}, "exclude_terms": []string{}, "socials": vim.Socials,
		"alert_feeds": []string{"https://www.google.com/alerts/feeds/1/2"},
	})
	if code != 200 {
		t.Fatalf("PUT: %d %s", code, body)
	}
	oasis := byName["Oasis 360"]
	if code, body := call("DELETE", "/api/watchlist/orgs/"+oasis.ID.String(), nil); code != 200 {
		t.Fatalf("DELETE: %d %s", code, body)
	}
	for i := 0; i < 2; i++ {
		if n, err := st.watchOrgs.SeedTemplates(ctx, watch.BrandTemplates()); err != nil || n != 0 {
			t.Fatalf("resiembra %d: n=%d err=%v", i, n, err)
		}
	}
	orgs = list()
	if len(orgs) != 3 {
		t.Fatalf("tras borrar y resembrar = %d, want 3", len(orgs))
	}
	for _, o := range orgs {
		if o.Name == "Vimenti" && (len(o.Aliases) != 1 || o.Hashtags[0] != "VimentiPR" || len(o.AlertFeeds) != 1) {
			t.Errorf("se pisó la edición: %+v", o)
		}
	}

	// 3. Tope de 10 marcas activas (hay 3).
	for i := 0; i < 7; i++ {
		if code, body := call("POST", "/api/watchlist/orgs", map[string]any{"name": fmt.Sprintf("Marca %d", i), "kind": "marca"}); code != 201 {
			t.Fatalf("crear marca %d: %d %s", i, code, body)
		}
	}
	if code, _ := call("POST", "/api/watchlist/orgs", map[string]any{"name": "Marca 11", "kind": "marca"}); code != http.StatusConflict {
		t.Fatalf("la 11.ª marca debió dar 409, dio %d", code)
	}
	// Temas y personas no cuentan.
	if code, body := call("POST", "/api/watchlist/orgs", map[string]any{"name": "Vivienda", "kind": "tema", "keywords": []string{"vivienda"}}); code != 201 {
		t.Fatalf("tema: %d %s", code, body)
	}
	// Pausar una y activar otra pausada.
	if code, _ := call("PATCH", "/api/watchlist/orgs/"+vim.ID.String()+"/toggle", map[string]bool{"active": false}); code != 200 {
		t.Fatal("pausar")
	}
	code, body = call("POST", "/api/watchlist/orgs", map[string]any{"name": "Pausada", "kind": "marca", "active": false})
	if code != 201 {
		t.Fatalf("pausada: %d %s", code, body)
	}
	var paused models.WatchlistOrg
	_ = json.Unmarshal(body, &paused)
	if code, _ := call("PATCH", "/api/watchlist/orgs/"+vim.ID.String()+"/toggle", map[string]bool{"active": true}); code != 200 {
		t.Fatal("reactivar Vimenti (hay 9 activas) debió poder")
	}
	if code, _ := call("PATCH", "/api/watchlist/orgs/"+paused.ID.String()+"/toggle", map[string]bool{"active": true}); code != http.StatusConflict {
		t.Fatalf("activar la 11.ª debió dar 409, dio %d", code)
	}

	// 4. Deduplicado: misma URL o mismo título normalizado = una mención.
	mk := func(src, title, link string) bool {
		hit := &models.WatchlistHit{OrgID: vim.ID, SourceType: src, Title: title, URL: link, URLHash: link, TitleKey: "vimenti y starbucks lanzan recaudacion de 5"}
		if err := st.watchHits.Create(ctx, hit); err != nil {
			t.Fatal(err)
		}
		return hit.ID.String() != "00000000-0000-0000-0000-000000000000"
	}
	if !mk("google_news", "Vimenti y Starbucks… - NIMB", "https://news.google.com/a") {
		t.Fatal("la primera debió entrar")
	}
	if mk("bing_news", "Vimenti y Starbucks…", "https://newsismybusiness.com/a") {
		t.Fatal("mismo título por Bing debió deduplicarse")
	}
	code, body = call("GET", "/api/watchlist/hits?org_id="+vim.ID.String(), nil)
	var hr struct {
		Hits []models.WatchlistHit `json:"hits"`
	}
	_ = json.Unmarshal(body, &hr)
	if code != 200 || len(hr.Hits) != 1 || hr.Hits[0].Network != "noticias" {
		t.Fatalf("hits: %d %s", code, body)
	}
	if code, body := call("POST", "/api/watchlist/orgs/"+vim.ID.String()+"/seen-all", nil); code != 200 {
		t.Fatalf("seen-all: %d %s", code, body)
	}
}
