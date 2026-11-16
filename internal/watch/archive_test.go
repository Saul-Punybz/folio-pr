package watch

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Saul-Punybz/folio/internal/models"
)

func TestDecodeGoogleNewsURL(t *testing.T) {
	id := base64.RawURLEncoding.EncodeToString([]byte("\x08\x13\x22\x4chttps://www.elnuevodia.com/negocios/nota-1/\xd2\x01\x00"))
	if got := decodeGoogleNewsURL("https://news.google.com/rss/articles/" + id + "?oc=5"); got != "https://www.elnuevodia.com/negocios/nota-1/" {
		t.Errorf("decode = %q", got)
	}
	// Identificadores nuevos: no traen la URL.
	if got := decodeGoogleNewsURL("https://news.google.com/rss/articles/CBMiAU_yqLxxx?oc=5"); got != "" {
		t.Errorf("no debía decodificar: %q", got)
	}
	if got := decodeGoogleNewsURL("https://ejemplo.pr/x"); got != "" {
		t.Errorf("otro host: %q", got)
	}
}

func TestArchivarRespetaRobotsYMarcaSinCopia(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/robots.txt":
			_, _ = w.Write([]byte("User-agent: *\nDisallow: /privado/\n"))
		case "/bloqueada":
			http.Error(w, "no", http.StatusForbidden)
		default:
			hits++
			_, _ = w.Write([]byte("<html><title>x</title></html>"))
		}
	}))
	defer srv.Close()
	it := func(path string) models.ArchiveItem {
		return models.ArchiveItem{SourceType: "web", Title: "Título - Medio", URL: srv.URL + path, Snippet: "Descripción del buscador con algo de sustancia para que sirva de resumen cuando la página no carga.", NeedFetch: true, WaybackURL: "ya"}
	}
	for path, want := range map[string]string{"/privado/nota": "robots", "/bloqueada": "403"} {
		r := archiveOne(context.Background(), it(path))
		if r.Status != models.ArchiveSinCopia || !r.Fetched || r.Title == "" || r.Summary == "" {
			t.Errorf("%s: %+v", path, r)
		}
		if !strings.Contains(r.Err, want) {
			t.Errorf("%s: error %q", path, r.Err)
		}
	}
	if hits != 0 {
		t.Errorf("no debía pedir la página protegida (%d)", hits)
	}
	r := archiveOne(context.Background(), it("/nota"))
	if r.Status != models.ArchiveParcial || hits != 1 {
		t.Errorf("página sin cuerpo debía quedar parcial: %+v", r)
	}
}
