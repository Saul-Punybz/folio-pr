package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

func TestNoticiasBorrarArchivarYPurgar(t *testing.T) {
	pool := testPG(t)
	ctx := context.Background()
	st := newStores(pool, t.TempDir())

	u := &models.User{Email: "noticias@folio.local", PasswordHash: "x", Role: "admin"}
	if err := st.users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	sess := &models.Session{ID: "token-noticias", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := st.sessions.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	h := newRouter(st, fstest.MapFS{}, routerHooks{})
	call := func(method, path string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.AddCookie(&http.Cookie{Name: "session_token", Value: sess.ID})
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		var out map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	mk := func(slug, status string) *models.Article {
		a := &models.Article{
			Title: "Noticia " + slug, Source: "Prueba", URL: "https://ejemplo.pr/" + slug,
			CanonicalURL: scraper.CanonicalizeURL("https://ejemplo.pr/" + slug), Region: "PR",
			Status: status, CleanText: "texto unico de " + slug + " zanahoria", EvidencePolicy: "keep",
		}
		if err := st.articles.Create(ctx, a); err != nil {
			t.Fatal(err)
		}
		return a
	}
	inList := func(path, key string, id uuid.UUID) bool {
		_, out := call("GET", path, nil)
		items, _ := out[key].([]any)
		for _, it := range items {
			if m, ok := it.(map[string]any); ok && m["id"] == id.String() {
				return true
			}
		}
		return false
	}

	a := mk("a", "inbox")
	b := mk("b", "inbox")
	c := mk("c", "saved")
	d := mk("d", "inbox")

	// Borrar oculta de listado y búsqueda.
	if code, _ := call("POST", "/api/items/"+a.ID.String()+"/trash", nil); code != 200 {
		t.Fatalf("trash: %d", code)
	}
	if inList("/api/items?status=inbox", "items", a.ID) || inList("/api/search?q=zanahoria", "results", a.ID) {
		t.Fatal("la borrada sigue visible")
	}
	if !inList("/api/search?q=zanahoria", "results", b.ID) {
		t.Fatal("la búsqueda no encuentra lo no borrado")
	}
	// Deshacer la devuelve.
	if code, _ := call("POST", "/api/items/undo", map[string]any{"ids": []string{a.ID.String()}}); code != 200 {
		t.Fatalf("undo: %d", code)
	}
	if !inList("/api/items?status=inbox", "items", a.ID) {
		t.Fatal("deshacer no la devolvió")
	}

	// Borrado múltiple (guardada incluida) y deshacer vuelve a su estado.
	ids := []string{a.ID.String(), c.ID.String()}
	if code, out := call("POST", "/api/items/trash", map[string]any{"ids": ids}); code != 200 || out["count"] != float64(2) {
		t.Fatalf("trash many: %d %v", code, out)
	}
	if code, _ := call("POST", "/api/items/trash", map[string]any{"ids": []string{}}); code != 400 {
		t.Fatalf("ids vacío: %d", code)
	}
	if inList("/api/items?status=saved", "items", c.ID) {
		t.Fatal("la guardada borrada sigue en Guardados")
	}
	call("POST", "/api/items/undo", map[string]any{"ids": []string{c.ID.String()}})
	if !inList("/api/items?status=saved", "items", c.ID) {
		t.Fatal("deshacer no devolvió la guardada a Guardados")
	}

	// Purga: no toca lo reciente; luego elimina, bloquea URL y contenido.
	n, err := st.articles.PurgeTrashed(ctx, time.Hour, scraper.HashURL, scraper.HashContent)
	if err != nil || n != 0 {
		t.Fatalf("purga temprana: n=%d err=%v", n, err)
	}
	if _, err := pool.Exec(ctx, `UPDATE articles SET trashed_at = now() - interval '2 hours' WHERE id = $1`, a.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := st.articles.PurgeTrashed(ctx, time.Hour, scraper.HashURL, scraper.HashContent); err != nil || n != 1 {
		t.Fatalf("purga: n=%d err=%v", n, err)
	}
	if _, err := st.articles.GetByID(ctx, a.ID); err == nil {
		t.Fatal("la fila purgada sigue en la base")
	}
	if code, _ := call("POST", "/api/items/"+a.ID.String()+"/undo", nil); code == 200 {
		// el deshacer de una fila inexistente no debe resucitarla
		t.Fatal("deshacer tras la purga debió fallar")
	}
	exists, blocked, err := st.fingerprints.ExistsOrBlocked(ctx, scraper.HashURL(a.URL))
	if err != nil || !exists || !blocked {
		t.Fatalf("URL no bloqueada: exists=%v blocked=%v err=%v", exists, blocked, err)
	}
	if ok, err := st.fingerprints.ContentBlocked(ctx, scraper.HashContent(a.CleanText)); err != nil || !ok {
		t.Fatalf("contenido no bloqueado: %v %v", ok, err)
	}

	// Pasa a archived a los 7 días; lo guardado no.
	if _, err := pool.Exec(ctx, `UPDATE articles SET created_at = now() - interval '8 days' WHERE id = ANY($1)`, []uuid.UUID{b.ID, c.ID}); err != nil {
		t.Fatal(err)
	}
	if inList("/api/items?status=inbox", "items", b.ID) {
		t.Fatal("el Inbox muestra algo de más de 7 días")
	}
	if n, err := st.articles.ArchiveOld(ctx); err != nil || n != 1 {
		t.Fatalf("archivar: n=%d err=%v", n, err)
	}
	got, _ := st.articles.GetByID(ctx, b.ID)
	if got.Status != "archived" {
		t.Fatalf("b: %s", got.Status)
	}
	if got, _ := st.articles.GetByID(ctx, c.ID); got.Status != "saved" {
		t.Fatalf("la guardada cambió: %s", got.Status)
	}
	if got, _ := st.articles.GetByID(ctx, d.ID); got.Status != "inbox" {
		t.Fatalf("d: %s", got.Status)
	}
	// Archivo busca en inbox, archived y saved; filtro por estado.
	if !inList("/api/search?q=zanahoria&status=archived", "results", b.ID) || inList("/api/search?q=zanahoria&status=archived", "results", c.ID) {
		t.Fatal("filtro archived mal")
	}
	if !inList("/api/search?q=zanahoria", "results", c.ID) {
		t.Fatal("Archivo no encuentra lo guardado")
	}
	// Lo archivado y guardado sobrevive a la purga y se puede borrar desde Archivo.
	call("POST", "/api/items/"+b.ID.String()+"/trash", nil)
	if inList("/api/search?q=zanahoria", "results", b.ID) {
		t.Fatal("borrada desde Archivo sigue en la búsqueda")
	}
}
