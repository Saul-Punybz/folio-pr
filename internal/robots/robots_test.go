package robots

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func server(t *testing.T, status int, body string) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/robots.txt" {
			http.NotFound(w, r)
			return
		}
		atomic.AddInt32(&hits, 1)
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(s.Close)
	return s, &hits
}

func TestPermitidoYProhibido(t *testing.T) {
	s, _ := server(t, 200, "User-agent: *\nDisallow: /privado\nCrawl-delay: 3\n")
	c := New("FolioBot/1.0", nil)
	ctx := context.Background()
	if !c.Allowed(ctx, s.URL+"/publico") {
		t.Error("/publico debería estar permitido")
	}
	if c.Allowed(ctx, s.URL+"/privado/x?a=1") {
		t.Error("/privado/x debería estar prohibido")
	}
	if d := c.CrawlDelay(s.URL + "/"); d != 3*time.Second {
		t.Errorf("crawl-delay = %v", d)
	}
}

func TestPorUserAgent(t *testing.T) {
	s, _ := server(t, 200, "User-agent: foliobot\nDisallow: /\n\nUser-agent: *\nDisallow:\n")
	ctx := context.Background()
	if New("FolioBot/1.0", nil).Allowed(ctx, s.URL+"/a") {
		t.Error("FolioBot debería estar bloqueado")
	}
	if !New("OtroBot/2.0", nil).Allowed(ctx, s.URL+"/a") {
		t.Error("OtroBot debería estar permitido")
	}
}

func TestSinRobots(t *testing.T) {
	for _, st := range []int{404, 403, 410} {
		s, _ := server(t, st, "")
		c := New("FolioBot/1.0", nil)
		if !c.Allowed(context.Background(), s.URL+"/x") {
			t.Errorf("status %d: debería permitir", st)
		}
		if c.CrawlDelay(s.URL) != 0 {
			t.Errorf("status %d: delay debería ser 0", st)
		}
	}
}

func TestError5xxNoSeCacheaMucho(t *testing.T) {
	s, hits := server(t, 503, "")
	c := New("FolioBot/1.0", nil)
	now := time.Now()
	c.now = func() time.Time { return now }
	ctx := context.Background()
	if !c.Allowed(ctx, s.URL+"/x") {
		t.Fatal("5xx debería permitir")
	}
	c.Allowed(ctx, s.URL+"/y")
	if atomic.LoadInt32(hits) != 1 {
		t.Errorf("dentro de 10 min no debe reconsultar; hits=%d", atomic.LoadInt32(hits))
	}
	now = now.Add(11 * time.Minute)
	c.Allowed(ctx, s.URL+"/z")
	if atomic.LoadInt32(hits) != 2 {
		t.Errorf("pasados 10 min debe reconsultar; hits=%d", atomic.LoadInt32(hits))
	}
}

func TestErrorDeRed(t *testing.T) {
	s, _ := server(t, 200, "")
	url := s.URL
	s.Close()
	if !New("FolioBot/1.0", nil).Allowed(context.Background(), url+"/x") {
		t.Error("error de red debería permitir")
	}
}

func TestCacheYConcurrencia(t *testing.T) {
	s, hits := server(t, 200, "User-agent: *\nDisallow: /no\n")
	c := New("FolioBot/1.0", nil)
	now := time.Now()
	var mu sync.Mutex
	c.now = func() time.Time { mu.Lock(); defer mu.Unlock(); return now }
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); c.Allowed(context.Background(), s.URL+"/ok"); c.CrawlDelay(s.URL) }()
	}
	wg.Wait()
	before := atomic.LoadInt32(hits)
	c.Allowed(context.Background(), s.URL+"/otra")
	if atomic.LoadInt32(hits) != before {
		t.Error("la segunda consulta debió salir de caché")
	}
	mu.Lock()
	now = now.Add(25 * time.Hour)
	mu.Unlock()
	c.Allowed(context.Background(), s.URL+"/ok")
	if atomic.LoadInt32(hits) != before+1 {
		t.Error("tras 24 h debe reconsultar")
	}
}

func TestURLInvalida(t *testing.T) {
	c := New("FolioBot/1.0", nil)
	for _, u := range []string{"", "://x", "ftp://a/b", "/solo/ruta", "http://"} {
		if c.Allowed(context.Background(), u) {
			t.Errorf("%q debería ser false", u)
		}
	}
}
