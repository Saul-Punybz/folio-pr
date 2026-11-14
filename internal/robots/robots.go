// Package robots comprueba robots.txt por host, con caché y seguro para uso concurrente.
package robots

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/temoto/robotstxt"
)

const (
	okTTL   = 24 * time.Hour
	failTTL = 10 * time.Minute
	maxBody = 512 << 10
)

type entry struct {
	data    *robotstxt.RobotsData // nil = todo permitido
	expires time.Time
}

// Checker consulta y cachea robots.txt por host.
type Checker struct {
	ua     string // user-agent completo, para la petición
	token  string // token de producto, para elegir el grupo
	client *http.Client
	now    func() time.Time

	mu    sync.Mutex
	cache map[string]entry
}

// New crea un Checker. Con client nil usa uno con timeout de 10 s.
func New(userAgent string, client *http.Client) *Checker {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	tok := userAgent
	if i := strings.IndexAny(tok, "/ "); i > 0 {
		tok = tok[:i]
	}
	return &Checker{ua: userAgent, token: tok, client: client, now: time.Now, cache: map[string]entry{}}
}

func parse(raw string) (*url.URL, bool) {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return nil, false
	}
	return u, true
}

func hostKey(u *url.URL) string { return u.Scheme + "://" + strings.ToLower(u.Host) }

// Allowed dice si el user-agent puede pedir rawURL. URL inválida devuelve false.
func (c *Checker) Allowed(ctx context.Context, rawURL string) bool {
	u, ok := parse(rawURL)
	if !ok {
		return false
	}
	e := c.get(ctx, u)
	if e.data == nil {
		return true
	}
	path := u.EscapedPath()
	if path == "" {
		path = "/"
	}
	if u.RawQuery != "" {
		path += "?" + u.RawQuery
	}
	return e.data.FindGroup(c.token).Test(path)
}

// CrawlDelay devuelve el Crawl-delay declarado (0 si no hay o aún no se ha consultado el host).
func (c *Checker) CrawlDelay(rawURL string) time.Duration {
	u, ok := parse(rawURL)
	if !ok {
		return 0
	}
	c.mu.Lock()
	e, found := c.cache[hostKey(u)]
	c.mu.Unlock()
	if !found || e.data == nil || !c.now().Before(e.expires) {
		return 0
	}
	return e.data.FindGroup(c.token).CrawlDelay
}

func (c *Checker) get(ctx context.Context, u *url.URL) entry {
	key := hostKey(u)
	c.mu.Lock()
	e, found := c.cache[key]
	c.mu.Unlock()
	if found && c.now().Before(e.expires) {
		return e
	}
	e = c.fetch(ctx, key)
	c.mu.Lock()
	c.cache[key] = e
	c.mu.Unlock()
	return e
}

func (c *Checker) fetch(ctx context.Context, key string) entry {
	fail := entry{expires: c.now().Add(failTTL)}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, key+"/robots.txt", nil)
	if err != nil {
		return fail
	}
	req.Header.Set("User-Agent", c.ua)
	resp, err := c.client.Do(req)
	if err != nil {
		return fail
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 500:
		return fail
	case resp.StatusCode != http.StatusOK:
		// 404/403/410 y demás 4xx: sin robots, todo permitido.
		return entry{expires: c.now().Add(okTTL)}
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return fail
	}
	data, err := robotstxt.FromBytes(body)
	if err != nil {
		return entry{expires: c.now().Add(okTTL)}
	}
	return entry{data: data, expires: c.now().Add(okTTL)}
}
