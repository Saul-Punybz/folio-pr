package scraper

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func leer(t *testing.T, nombre string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + nombre)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseAnyFeedTelemundoFechaConAMPM(t *testing.T) {
	items, err := parseAnyFeed(leer(t, "telemundo.xml"))
	if err != nil || len(items) == 0 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	// "Mon, Nov 16 2026 09:45:54 AM" en hora de Puerto Rico (UTC-4) = 13:45:54 UTC.
	var hallada bool
	for _, it := range items {
		if it.Published.IsZero() {
			t.Fatalf("fecha sin parsear en %q", it.Title)
		}
		if it.Published.UTC().Format("2006-01-02 15:04:05") == "2026-11-16 13:45:54" {
			hallada = true
		}
	}
	if !hallada {
		t.Fatalf("no se encontró 2026-11-16 13:45:54 UTC: %v", items[0].Published)
	}
}

func TestParseAnyFeedWordPressREST(t *testing.T) {
	items, err := parseAnyFeed(leer(t, "nimb_wp.json"))
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	if !strings.Contains(items[0].Title, "R&D") || strings.Contains(items[0].Title, "&#038;") {
		t.Fatalf("título sin desescapar: %q", items[0].Title)
	}
	if items[0].Published.Location() != time.UTC && items[0].Published.UTC().Hour() != 9 {
		t.Fatalf("date_gmt debe tratarse como UTC: %v", items[0].Published)
	}
	if !strings.HasPrefix(items[0].Link, "https://newsismybusiness.com/") {
		t.Fatalf("link: %q", items[0].Link)
	}
}

func TestParseAnyFeedSitemapDeNoticias(t *testing.T) {
	items, err := parseAnyFeed(leer(t, "jay_news_sitemap.xml"))
	if err != nil || len(items) != 3 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	if items[0].Title == "" || items[0].Published.IsZero() || !strings.HasPrefix(items[0].Link, "https://jayfonseca.com/") {
		t.Fatalf("item: %+v", items[0])
	}
}

func TestParseAnyFeedSitemapNormalNoEsFeed(t *testing.T) {
	_, err := parseAnyFeed([]byte(`<?xml version="1.0"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"><url><loc>https://x.pr/a</loc></url></urlset>`))
	if err == nil {
		t.Fatal("un sitemap sin news:title no debe tomarse como feed")
	}
}

func TestParseAnyFeedVacioDistingueDeDesconocido(t *testing.T) {
	if _, err := parseAnyFeed(leer(t, "teleonce_empty.xml")); err != errEmptyFeed {
		t.Fatalf("esperaba errEmptyFeed, obtuve %v", err)
	}
	if _, err := parseAnyFeed([]byte("<html><body>hola</body></html>")); err == nil || err == errEmptyFeed {
		t.Fatalf("HTML debe ser formato no reconocido, obtuve %v", err)
	}
}

func TestParseAnyFeedJSONFeedYRDF(t *testing.T) {
	jf := `{"version":"https://jsonfeed.org/version/1.1","items":[{"id":"1","url":"https://x.pr/a","title":"A","summary":"s","date_published":"2026-11-10T12:00:00Z","tags":["pr"]}]}`
	items, err := parseAnyFeed([]byte(jf))
	if err != nil || len(items) != 1 || items[0].Categories[0] != "pr" || items[0].Published.IsZero() {
		t.Fatalf("json feed: %v %+v", err, items)
	}
	rdf := `<?xml version="1.0"?><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#" xmlns="http://purl.org/rss/1.0/" xmlns:dc="http://purl.org/dc/elements/1.1/"><channel><title>c</title></channel><item><title>B</title><link>https://x.pr/b</link><dc:date>2026-11-11T08:00:00-04:00</dc:date></item></rdf:RDF>`
	items, err = parseAnyFeed([]byte(rdf))
	if err != nil || len(items) != 1 || items[0].Published.IsZero() {
		t.Fatalf("rdf: %v %+v", err, items)
	}
}

func TestParseFeedHTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(leer(t, "nimb_wp.json"))
	}))
	defer srv.Close()
	items, err := ParseFeed(context.Background(), srv.URL)
	if err != nil || len(items) != 2 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
}

func TestGoogleNewsTraeMedioReal(t *testing.T) {
	items, err := parseAnyFeed(leer(t, "googlenews.xml"))
	if err != nil || len(items) == 0 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	for _, it := range items {
		if it.SourceName == "" || !strings.HasPrefix(it.SourceURL, "http") {
			t.Fatalf("falta el medio real: %+v", it)
		}
		if !strings.Contains(it.Link, "news.google.com/rss/articles/") {
			t.Fatalf("enlace inesperado: %q", it.Link)
		}
	}
}

func TestBingNewsResuelveURLReal(t *testing.T) {
	items, err := parseAnyFeed(leer(t, "bingnews.xml"))
	if err != nil || len(items) == 0 {
		t.Fatalf("items=%d err=%v", len(items), err)
	}
	for _, it := range items {
		real := ResolveBingNewsURL(it.Link)
		if !strings.HasPrefix(real, "http") || strings.Contains(real, "bing.com") {
			t.Fatalf("no se resolvió %q -> %q", it.Link, real)
		}
	}
	if got := ResolveBingNewsURL("https://example.com/a?url=https://x.y"); got != "https://example.com/a?url=https://x.y" {
		t.Fatalf("un enlace que no es de Bing no se toca: %q", got)
	}
}

func TestRegionForDomain(t *testing.T) {
	casos := map[string]string{
		"www.elnuevodia.com":     "PR",
		"noticias.upr.edu":       "PR",
		"fortaleza.pr.gov":       "PR",
		"cms.metro.pr":           "PR",
		"nuevo-medio.com":        "PR", // viene en extraPR
		"www.nytimes.com":        "US",
		"abcnews.go.com":         "US",
		"hud.gov":                "US",
		"elpais.com":             "Global",
		"jamaica-gleaner.com":    "Global",
		"":                       "Global",
		"bbc.co.uk":              "Global",
		"mipueblo.puertorico.us": "US",
	}
	for host, want := range casos {
		if got := RegionForDomain(host, []string{"nuevo-medio.com"}); got != want {
			t.Errorf("%q: %s, esperaba %s", host, got, want)
		}
	}
	if !isSearchAggregator("https://news.google.com/rss/search?q=x") || !isSearchAggregator("https://www.bing.com/news/search?q=x&format=rss") || isSearchAggregator("https://elpais.com/rss") {
		t.Fatal("detección de fuentes de búsqueda")
	}
}
