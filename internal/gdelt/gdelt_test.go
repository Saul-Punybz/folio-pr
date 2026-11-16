package gdelt

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"
)

func leer(t *testing.T, n string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + n)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func servidor(t *testing.T, status int, body []byte, visto *url.Values) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if visto != nil {
			*visto = r.URL.Query()
		}
		w.WriteHeader(status)
		w.Write(body)
	}))
}

func cliente(u string) *Client {
	c := NewClient()
	c.BaseURL = u
	c.MinInterval = 0
	return c
}

func TestSearchRespuestaReal(t *testing.T) {
	var q url.Values
	srv := servidor(t, 200, leer(t, "artlist.json"), &q)
	defer srv.Close()
	arts, err := cliente(srv.URL).Search(context.Background(), Phrase("Puerto Rico"), "1year", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(arts) != 3 {
		t.Fatalf("artículos: %d", len(arts))
	}
	a := arts[0]
	if a.URL != "https://jamaica-gleaner.com/article/commentary/20260902/elizabeth-morgan-water-crisis-puerto-rico" ||
		a.Domain != "jamaica-gleaner.com" || a.Language != "English" || a.Country != "Jamaica" || a.Title == "" {
		t.Fatalf("campos: %+v", a)
	}
	if want := time.Date(2026, 9, 2, 5, 30, 0, 0, time.UTC); !a.Seen.Equal(want) {
		t.Fatalf("seendate: %v", a.Seen)
	}
	if q.Get("query") != `"Puerto Rico"` || q.Get("mode") != "artlist" || q.Get("format") != "json" ||
		q.Get("maxrecords") != "3" || q.Get("timespan") != "1year" {
		t.Fatalf("parámetros: %v", q)
	}
}

func TestSearchSinResultadosYErroresDeTexto(t *testing.T) {
	srv := servidor(t, 200, leer(t, "vacio.json"), nil)
	arts, err := cliente(srv.URL).Search(context.Background(), "Vimenti", "3months", 10)
	srv.Close()
	if err != nil || len(arts) != 0 {
		t.Fatalf("{} debe ser vacío sin error: %v %v", arts, err)
	}
	srv = servidor(t, 200, []byte("Your query was too short or too long."), nil)
	defer srv.Close()
	if _, err := cliente(srv.URL).Search(context.Background(), "a", "3months", 10); err == nil {
		t.Fatal("texto plano con 200 debe ser error")
	}
}

func TestSearchLimiteDeFrecuencia(t *testing.T) {
	srv := servidor(t, 429, leer(t, "limite_429.txt"), nil)
	defer srv.Close()
	_, err := cliente(srv.URL).Search(context.Background(), "Vimenti", "3months", 10)
	if !errors.Is(err, ErrRateLimited) {
		t.Fatalf("esperaba ErrRateLimited, obtuve %v", err)
	}
}

func TestSearchTopeDeMaxYQueryVacia(t *testing.T) {
	var q url.Values
	srv := servidor(t, 200, []byte("{}"), &q)
	defer srv.Close()
	c := cliente(srv.URL)
	if _, err := c.Search(context.Background(), "x", "", 9999); err != nil {
		t.Fatal(err)
	}
	if q.Get("maxrecords") != "250" || q.Has("timespan") {
		t.Fatalf("parámetros: %v", q)
	}
	if _, err := c.Search(context.Background(), " ", "1week", 5); err == nil {
		t.Fatal("query vacía debe fallar")
	}
}

func TestSearchToleraControlEnTitulos(t *testing.T) {
	body := []byte("{\"articles\":[{\"url\":\"https://x.pr/a\",\"title\":\"Hola\x01 mundo\",\"seendate\":\"20261110T120000Z\",\"domain\":\"x.pr\",\"language\":\"Spanish\",\"sourcecountry\":\"Puerto Rico\"}]}")
	srv := servidor(t, 200, body, nil)
	defer srv.Close()
	arts, err := cliente(srv.URL).Search(context.Background(), "x", "1week", 5)
	if err != nil || len(arts) != 1 || arts[0].Title != "Hola mundo" {
		t.Fatalf("%v %+v", err, arts)
	}
}

func TestEspaciadoDeCincoSegundosPorDefecto(t *testing.T) {
	if NewClient().MinInterval != 5*time.Second {
		t.Fatal("el intervalo por defecto debe ser 5 s")
	}
	srv := servidor(t, 200, []byte("{}"), nil)
	defer srv.Close()
	c := cliente(srv.URL)
	c.MinInterval = 150 * time.Millisecond
	start := time.Now()
	for i := 0; i < 3; i++ {
		if _, err := c.Search(context.Background(), "x", "1week", 1); err != nil {
			t.Fatal(err)
		}
	}
	if d := time.Since(start); d < 300*time.Millisecond {
		t.Fatalf("3 peticiones tardaron %v, esperaba >= 300ms", d)
	}
}

func TestCancelacionDuranteEspera(t *testing.T) {
	srv := servidor(t, 200, []byte("{}"), nil)
	defer srv.Close()
	c := cliente(srv.URL)
	c.MinInterval = time.Hour
	if _, err := c.Search(context.Background(), "x", "1week", 1); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := c.Search(ctx, "x", "1week", 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("esperaba DeadlineExceeded, obtuve %v", err)
	}
}
