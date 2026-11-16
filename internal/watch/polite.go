package watch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/Saul-Punybz/folio/internal/scraper"
)

const (
	hostGap      = time.Second
	userAgent    = "Folio/1.0 (+https://github.com/Saul-Punybz/folio)"
	maxBodyBytes = 5 << 20
)

// hostClock guarda la última petición por host: ≥1 s entre dos al mismo host.
type hostClock struct {
	mu   sync.Mutex
	last map[string]time.Time
}

var clock = &hostClock{last: map[string]time.Time{}}

// wait espera lo que falte para poder pedirle otra vez a ese host.
func (c *hostClock) wait(ctx context.Context, rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	host := strings.ToLower(u.Hostname())
	c.mu.Lock()
	next := c.last[host].Add(hostGap)
	now := time.Now()
	if next.Before(now) {
		next = now
	}
	c.last[host] = next
	c.mu.Unlock()
	d := time.Until(next)
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// fetch hace un GET cortés (espera por host, UA de Folio, tope de tamaño).
// Devuelve el código HTTP aunque no sea 200.
func fetch(ctx context.Context, rawURL, accept string) ([]byte, int, error) {
	if err := clock.wait(ctx, rawURL); err != nil {
		return nil, 0, err
	}
	ctx, cancel := context.WithTimeout(ctx, agentTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("User-Agent", userAgent)
	if accept != "" {
		req.Header.Set("Accept", accept)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	if resp.StatusCode != http.StatusOK {
		return body, resp.StatusCode, fmt.Errorf("GET %s: status %d", rawURL, resp.StatusCode)
	}
	return body, resp.StatusCode, nil
}

// parseFeed es scraper.ParseFeed con la espera por host.
func parseFeed(ctx context.Context, feedURL string) ([]scraper.FeedItem, error) {
	if err := clock.wait(ctx, feedURL); err != nil {
		return nil, err
	}
	agentCtx, cancel := context.WithTimeout(ctx, agentTimeout)
	defer cancel()
	return scraper.ParseFeed(agentCtx, feedURL)
}
