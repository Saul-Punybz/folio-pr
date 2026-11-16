package podcasts

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func servidor(t *testing.T, status int, fixture string, visto *string) *httptest.Server {
	t.Helper()
	var body []byte
	if fixture != "" {
		var err error
		if body, err = os.ReadFile("testdata/" + fixture); err != nil {
			t.Fatal(err)
		}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if visto != nil {
			*visto = r.URL.RawQuery
		}
		w.WriteHeader(status)
		w.Write(body)
	}))
}

func cliente(url string) *Client {
	c := NewClient()
	c.BaseURL = url
	c.MinInterval = 0
	return c
}

func TestSearchEpisodesRespuestaReal(t *testing.T) {
	var q string
	srv := servidor(t, 200, "vimenti.json", &q)
	defer srv.Close()
	eps, err := cliente(srv.URL).SearchEpisodes(context.Background(), "Vimenti", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(eps) != 4 {
		t.Fatalf("episodios: %d", len(eps))
	}
	e := eps[0]
	if e.Title == "" || e.Show == "" || e.URL == "" || e.Released.IsZero() || e.Description == "" {
		t.Fatalf("campos vacíos: %+v", e)
	}
	for _, want := range []string{"media=podcast", "entity=podcastEpisode", "term=Vimenti", "limit=4"} {
		if !contains(q, want) {
			t.Errorf("la consulta %q no incluye %q", q, want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func TestSearchEpisodesLimiteYErrores(t *testing.T) {
	for _, st := range []int{http.StatusTooManyRequests, http.StatusForbidden} {
		srv := servidor(t, st, "", nil)
		_, err := cliente(srv.URL).SearchEpisodes(context.Background(), "x", 5)
		srv.Close()
		if !errors.Is(err, ErrRateLimited) {
			t.Errorf("HTTP %d: esperaba ErrRateLimited, obtuve %v", st, err)
		}
	}
	srv := servidor(t, 500, "", nil)
	defer srv.Close()
	if _, err := cliente(srv.URL).SearchEpisodes(context.Background(), "x", 5); err == nil || errors.Is(err, ErrRateLimited) {
		t.Errorf("HTTP 500 debe ser error normal, obtuve %v", err)
	}
	if _, err := cliente(srv.URL).SearchEpisodes(context.Background(), "  ", 5); err == nil {
		t.Error("término vacío debe fallar")
	}
}

func TestFilterMentionsIgnoraAcentosYMayusculas(t *testing.T) {
	eps, err := parse(mustRead(t, "vimenti.json"))
	if err != nil {
		t.Fatal(err)
	}
	if got := FilterMentions(eps, "VIMENTI"); len(got) == 0 || len(got) > len(eps) {
		t.Fatalf("filtro: %d de %d", len(got), len(eps))
	}
	if got := FilterMentions(eps, "término que no existe jamás"); len(got) != 0 {
		t.Fatalf("no debía quedar nada: %d", len(got))
	}
	if got := FilterMentions([]Episode{{Title: "Educación en Puerto Rico"}}, "educacion"); len(got) != 1 {
		t.Fatal("debe ignorar acentos")
	}
}

func mustRead(t *testing.T, n string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + n)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestEspaciadoEntrePeticiones(t *testing.T) {
	srv := servidor(t, 200, "vimenti.json", nil)
	defer srv.Close()
	c := cliente(srv.URL)
	c.MinInterval = 150 * time.Millisecond
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.SearchEpisodes(context.Background(), "x", 1); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 300*time.Millisecond {
		t.Fatalf("3 peticiones tardaron %v, esperaba >= 300ms", d)
	}
}

func TestCancelacionDuranteEspera(t *testing.T) {
	srv := servidor(t, 200, "vimenti.json", nil)
	defer srv.Close()
	c := cliente(srv.URL)
	c.MinInterval = time.Hour
	if _, err := c.SearchEpisodes(context.Background(), "x", 1); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.SearchEpisodes(ctx, "x", 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("esperaba DeadlineExceeded, obtuve %v", err)
	}
}
