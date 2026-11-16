package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
	"github.com/Saul-Punybz/folio/internal/watch"
)

type archivoEnv struct {
	t     *testing.T
	st    *stores
	ctx   context.Context
	call  func(method, path string, body any) (int, []byte)
	user  *models.User
	brand func(name string) *models.WatchlistOrg
}

func newArchivoEnv(t *testing.T) *archivoEnv {
	pool := testPG(t)
	ctx := context.Background()
	st := newStores(pool, t.TempDir())
	u := &models.User{Email: "archivo@folio.local", PasswordHash: "x", Role: "admin"}
	if err := st.users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	sess := &models.Session{ID: "token-archivo", UserID: u.ID, ExpiresAt: time.Now().Add(time.Hour)}
	if err := st.sessions.Create(ctx, sess); err != nil {
		t.Fatal(err)
	}
	h := newRouter(st, fstest.MapFS{}, routerHooks{})
	e := &archivoEnv{t: t, st: st, ctx: ctx, user: u}
	e.call = func(method, path string, body any) (int, []byte) {
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
	e.brand = func(name string) *models.WatchlistOrg {
		o := &models.WatchlistOrg{UserID: u.ID, Name: name, Kind: models.KindMarca, Aliases: []string{name}, Active: true}
		if err := st.watchOrgs.Create(ctx, o); err != nil {
			t.Fatal(err)
		}
		return o
	}
	return e
}

func (e *archivoEnv) hit(org *models.WatchlistOrg, src, title, link string) *models.WatchlistHit {
	h := &models.WatchlistHit{OrgID: org.ID, SourceType: src, Title: title, URL: link, URLHash: scraper.HashURL(link), Snippet: "Resumen del buscador de " + title}
	if err := e.st.watchHits.Create(e.ctx, h); err != nil {
		e.t.Fatal(err)
	}
	return h
}

func (e *archivoEnv) count(orgID uuid.UUID) int {
	var n int
	if err := e.st.watchHits.Pool().QueryRow(e.ctx, `SELECT count(*) FROM watchlist_hits WHERE org_id = $1`, orgID).Scan(&n); err != nil {
		e.t.Fatal(err)
	}
	return n
}

// Dedupe por marca: la misma URL entra una vez en cada marca, no una sola vez en total.
func TestDedupePorMarca(t *testing.T) {
	e := newArchivoEnv(t)
	pm, oasis := e.brand("Project Makers"), e.brand("Oasis 360")
	const link = "https://newsismybusiness.com/project-makers-y-oasis"
	if h := e.hit(pm, "web", "Project Makers y Oasis 360 abren el Oasis Hub", link); h.ID == uuid.Nil {
		t.Fatal("debía entrar en Project Makers")
	}
	if h := e.hit(oasis, "web", "Project Makers y Oasis 360 abren el Oasis Hub", link); h.ID == uuid.Nil {
		t.Fatal("la misma noticia debía entrar TAMBIÉN en Oasis 360")
	}
	if h := e.hit(pm, "bing_news", "Otro título distinto de la misma nota", link); h.ID != uuid.Nil {
		t.Fatal("la misma URL en la misma marca no debe repetirse")
	}
	if e.count(pm.ID) != 1 || e.count(oasis.ID) != 1 {
		t.Fatalf("conteos: pm=%d oasis=%d", e.count(pm.ID), e.count(oasis.ID))
	}
}

// Ninguna limpieza, purga, tope ni retención borra menciones; solo la usuaria, una por una.
func TestMencionesNoSeBorranSolas(t *testing.T) {
	e := newArchivoEnv(t)
	vim := e.brand("Vimenti")
	var ids []uuid.UUID
	for i, src := range []string{"google_news", "bing_news", "web", "local", "youtube", "reddit", "bluesky", "sitio", "mastodon", "alertas"} {
		h := e.hit(vim, src, "Mención número "+string(rune('A'+i))+" de Vimenti en el archivo", "https://ejemplo.pr/m/"+string(rune('a'+i)))
		ids = append(ids, h.ID)
	}
	// Una antigua, como las que borraría cualquier retención por edad.
	if _, err := e.st.watchHits.Pool().Exec(e.ctx, `UPDATE watchlist_hits SET created_at = now() - interval '5 years', seen = true`); err != nil {
		t.Fatal(err)
	}
	want := len(ids)

	// Todas las tareas de mantenimiento de Folio.
	runNoticiasMaintenance(e.ctx, e.st.articles)
	j := newJobs(e.st)
	j.docCleanup.fn(e.ctx)
	j.sessionCleanup.fn(e.ctx)
	j.noticias.fn(e.ctx)
	j.oppStatuses.fn(e.ctx)
	if n := e.count(vim.ID); n != want {
		t.Fatalf("tras la limpieza quedan %d de %d menciones", n, want)
	}
	// «Marcar visto» no archiva ni borra.
	for _, p := range []string{"/api/watchlist/orgs/" + vim.ID.String() + "/seen-all", "/api/watchlist/hits/seen-all"} {
		if code, b := e.call("POST", p, nil); code != 200 {
			t.Fatalf("%s: %d %s", p, code, b)
		}
	}
	if n := e.count(vim.ID); n != want {
		t.Fatalf("marcar visto dejó %d de %d", n, want)
	}
	// Ni siquiera un DELETE directo de SQL (otra limpieza futura) pasa el candado.
	if _, err := e.st.watchHits.Pool().Exec(e.ctx, `DELETE FROM watchlist_hits WHERE created_at < now() - interval '1 year'`); err == nil {
		t.Fatal("el candado debía impedir el DELETE directo")
	}
	if _, err := e.st.watchHits.Pool().Exec(e.ctx, `TRUNCATE watchlist_hits`); err == nil {
		t.Fatal("el candado debía impedir el TRUNCATE")
	}
	// Una marca con menciones no se borra de golpe.
	if code, _ := e.call("DELETE", "/api/watchlist/orgs/"+vim.ID.String(), nil); code != http.StatusConflict {
		t.Fatalf("borrar marca con menciones debía dar 409, dio %d", code)
	}
	// Una por una, a mano.
	if code, b := e.call("DELETE", "/api/watchlist/hits/"+ids[0].String(), nil); code != 200 {
		t.Fatalf("borrar una: %d %s", code, b)
	}
	if n := e.count(vim.ID); n != want-1 {
		t.Fatalf("tras borrar una quedan %d", n)
	}
	if code, _ := e.call("DELETE", "/api/watchlist/hits/"+ids[0].String(), nil); code != http.StatusNotFound {
		t.Fatalf("borrar otra vez debía dar 404, dio %d", code)
	}
}

func readFixture(t *testing.T, name string) []byte {
	b, err := os.ReadFile("../../internal/archivo/testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Relleno al arrancar, copia, reintentos, respaldo en Internet Archive, edición y exportación.
func TestArchivoRellenoYExportacion(t *testing.T) {
	e := newArchivoEnv(t)
	end, pm := readFixture(t, "elnuevodia.html"), readFixture(t, "projectmakers.html")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/nota/feria":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(end)
		case "/blog/libros":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write(pm)
		case "/wayback":
			if strings.Contains(r.URL.Query().Get("url"), "feria-de-empleo") {
				_, _ = w.Write([]byte(`{"archived_snapshots":{"closest":{"available":true,"url":"http://web.archive.org/web/20261105000000/http://x/nota/feria","status":"200","timestamp":"20261105000000"}}}`))
				return
			}
			_, _ = w.Write([]byte(`{"archived_snapshots":{}}`))
		default:
			http.NotFound(w, r) // también /robots.txt: todo permitido
		}
	}))
	defer srv.Close()
	old := watch.WaybackAPI
	watch.WaybackAPI = srv.URL + "/wayback"
	defer func() { watch.WaybackAPI = old }()

	vim := e.brand("Vimenti")
	pub := time.Date(2026, 11, 4, 23, 0, 0, 0, time.UTC)
	mk := func(src, title, link string) *models.WatchlistHit {
		h := &models.WatchlistHit{OrgID: vim.ID, SourceType: src, Title: title, URL: link, URLHash: scraper.HashURL(link), Snippet: "snippet de " + title, PublishedAt: &pub}
		// Menciones «de antes»: filas viejas sin ninguna columna del archivo.
		if err := e.st.watchHits.Create(e.ctx, h); err != nil {
			t.Fatal(err)
		}
		return h
	}
	feria := mk("google_news", "Celebrarán feria de empleo - El Nuevo Día", srv.URL+"/nota/feria")
	libros := mk("web", "Negocio del mes: Libros 787", srv.URL+"/blog/libros")
	roto := mk("web", "Página que ya no existe", srv.URL+"/nota/borrada")
	post := mk("bluesky", "Project Makers: Hoy abrimos convocatoria para emprendedores en San Juan", "https://bsky.app/profile/projectmakerspr.bsky.social/post/3abc")

	// Arranque: el relleno corre solo (la tarea del cron, tal como la llama el arranque).
	j := newJobs(e.st)
	j.archivo.fn(e.ctx)

	get := func(id uuid.UUID) models.WatchlistHit {
		h, err := e.st.watchHits.GetForUser(e.ctx, id, e.user.ID)
		if err != nil {
			t.Fatal(err)
		}
		return *h
	}
	f := get(feria.ID)
	if f.ArchiveStatus != models.ArchiveCopia || !f.HasText || f.SiteName != "El Nuevo Día" || len(f.Authors) != 1 || f.Authors[0] != "Itzel Rivera" ||
		f.Title != "Celebrarán feria de empleo este viernes en San Juan" || f.PublishedAt == nil || f.CanonicalURL == "" || f.Lang != "es" || f.ArchivedAt == nil {
		t.Fatalf("feria mal archivada: %+v", f)
	}
	if f.WaybackURL != "https://web.archive.org/web/20261105000000/http://x/nota/feria" {
		t.Fatalf("wayback = %q", f.WaybackURL)
	}
	if l := get(libros.ID); l.ArchiveStatus != models.ArchiveCopia || l.SiteName != "Project Makers" || l.WaybackURL != "" || l.Summary == "" {
		t.Fatalf("libros mal archivado: %+v", l)
	}
	if p := get(post.ID); p.ArchiveStatus != models.ArchiveCopia || len(p.Authors) != 1 || p.Authors[0] != "Project Makers" || !p.HasText {
		t.Fatalf("post mal archivado: %+v", p)
	}
	r := get(roto.ID)
	if r.ArchiveStatus != models.ArchiveSinCopia || r.ArchiveAttempts != 1 || r.Title != "Página que ya no existe" || r.Summary == "" {
		t.Fatalf("la rota debía quedar sin copia con lo que se tenga: %+v", r)
	}
	// Reintentos: máximo 3.
	for i := 0; i < 4; i++ {
		j.archivo.fn(e.ctx)
	}
	if r = get(roto.ID); r.ArchiveAttempts != models.MaxArchiveAttempts || r.ArchiveStatus != models.ArchiveSinCopia {
		t.Fatalf("intentos = %d, status %s", r.ArchiveAttempts, r.ArchiveStatus)
	}

	// Pantalla: archivo de la marca, con filtros y bibliografía.
	var resp struct {
		Total int `json:"total"`
		Stats models.ArchiveStats
		Hits  []struct {
			ID  uuid.UUID                        `json:"id"`
			APA struct{ Plain, Markdown string } `json:"apa"`
		} `json:"hits"`
		Bibliografia struct{ Plain, Markdown string } `json:"bibliografia"`
	}
	list := func(q string) {
		resp.Hits = nil
		code, b := e.call("GET", "/api/watchlist/orgs/"+vim.ID.String()+"/archivo"+q, nil)
		if code != 200 {
			t.Fatalf("archivo%s: %d %s", q, code, b)
		}
		if err := json.Unmarshal(b, &resp); err != nil {
			t.Fatal(err)
		}
	}
	list("")
	if resp.Total != 4 || resp.Stats.Total != 4 || resp.Stats.Copia != 3 || resp.Stats.SinCopia != 1 || resp.Stats.ConWayback != 1 || resp.Stats.ConAutor < 3 {
		t.Fatalf("stats: %+v total=%d", resp.Stats, resp.Total)
	}
	if !strings.Contains(resp.Bibliografia.Plain, "Rivera, I. (2026, 4 de noviembre). Celebrarán feria de empleo este viernes en San Juan. El Nuevo Día. https://www.elnuevodia.com/") {
		t.Fatalf("bibliografía: %s", resp.Bibliografia.Plain)
	}
	if strings.Contains(resp.Bibliografia.Plain, "*") || !strings.Contains(resp.Bibliografia.Markdown, "*El Nuevo Día*") {
		t.Fatalf("cursiva: %s", resp.Bibliografia.Markdown)
	}
	list("?q=" + "librer%C3%ADa") // sin acentos o con ellos: busca en el texto archivado
	if resp.Total != 1 || resp.Hits[0].ID != libros.ID {
		t.Fatalf("búsqueda de texto: total=%d", resp.Total)
	}
	list("?fuente=bluesky")
	if resp.Total != 1 || resp.Hits[0].ID != post.ID {
		t.Fatalf("filtro por fuente: total=%d", resp.Total)
	}
	list("?desde=2026-11-05")
	if resp.Total != 0 {
		t.Fatalf("filtro por fecha (desde 5 nov): total=%d", resp.Total)
	}
	list("?desde=2026-11-04&hasta=2026-11-04")
	if resp.Total != 3 { // las tres con fecha 4 nov (hora PR: 19:00 del 4)
		t.Fatalf("filtro por fecha (4 nov): total=%d", resp.Total)
	}

	// «Ver copia guardada».
	code, b := e.call("GET", "/api/watchlist/hits/"+feria.ID.String()+"/copia", nil)
	if code != 200 || !strings.Contains(string(b), "segunda feria de empleo de 2026") {
		t.Fatalf("copia: %d %s", code, b)
	}

	// Corregir metadatos: la cita se regenera y la copia automática ya no los pisa.
	code, b = e.call("PATCH", "/api/watchlist/hits/"+feria.ID.String()+"/meta", map[string]any{
		"title": "Feria de empleo en San Juan", "site_name": "El Nuevo Día", "authors": []string{"Itzel Rivera Rivera"},
		"published_at": "2026-11-04", "summary": "Resumen corregido a mano.", "canonical_url": "https://www.elnuevodia.com/n/1",
	})
	if code != 200 || !strings.Contains(string(b), "Rivera Rivera, I. (2026, 4 de noviembre). Feria de empleo en San Juan. El Nuevo Día. https://www.elnuevodia.com/n/1") {
		t.Fatalf("meta: %d %s", code, b)
	}
	if code, _ := e.call("POST", "/api/watchlist/hits/"+feria.ID.String()+"/reintentar", nil); code != http.StatusNotFound {
		t.Fatalf("reintentar una copia completa debía dar 404, dio %d", code) // solo aplica a sin copia / parcial
	}
	if _, err := e.st.watchHits.Pool().Exec(e.ctx, `UPDATE watchlist_hits SET archive_status = 'pendiente' WHERE id = $1`, feria.ID); err != nil {
		t.Fatal(err)
	}
	j.archivo.fn(e.ctx)
	if f = get(feria.ID); f.Title != "Feria de empleo en San Juan" || f.Summary != "Resumen corregido a mano." || !f.Edited || f.ArchiveStatus != models.ArchiveCopia {
		t.Fatalf("la copia pisó la edición: %+v", f)
	}

	// Exportar.
	code, b = e.call("GET", "/api/watchlist/orgs/"+vim.ID.String()+"/archivo/exportar?formato=csv", nil)
	if code != 200 || !bytes.HasPrefix(b, []byte("\xef\xbb\xbf")) || !strings.Contains(string(b), "Resumen corregido a mano.") || strings.Count(string(b), "\n") < 5 {
		t.Fatalf("csv: %d %s", code, b)
	}
	code, b = e.call("GET", "/api/watchlist/orgs/"+vim.ID.String()+"/archivo/exportar?formato=md", nil)
	if code != 200 || !strings.Contains(string(b), "# Archivo de Vimenti") || !strings.Contains(string(b), "- Cita APA: Rivera Rivera, I.") {
		t.Fatalf("md: %d %s", code, b)
	}
	if code, _ := e.call("GET", "/api/watchlist/orgs/"+vim.ID.String()+"/archivo/exportar?formato=pdf", nil); code != 400 {
		t.Fatalf("formato inválido debía dar 400, dio %d", code)
	}
}
