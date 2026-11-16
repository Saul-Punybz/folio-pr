package watch

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// ScanWeb busca en la web (DuckDuckGo) para cada búsqueda.
func ScanWeb(ctx context.Context, org models.WatchlistOrg, queries []string, m Matcher, deps Deps) int {
	hits := 0
	for _, query := range queries {
		if hits >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}
		if err := clock.wait(ctx, "https://lite.duckduckgo.com/"); err != nil {
			break
		}
		if webBlocked() {
			break
		}
		results, err := scraper.WebSearch(ctx, webQuery(query, org), 20)
		if err != nil {
			slog.Warn("watchlist/web: search", "query", query, "err", err)
			if strings.Contains(err.Error(), "status 202") || strings.Contains(err.Error(), "status 403") || strings.Contains(err.Error(), "status 429") {
				blockWeb() // DuckDuckGo pide verificación: no insistir en esta hora
			}
			continue
		}
		for _, result := range results {
			if hits >= maxResultsPerAgent {
				break
			}
			if result.URL == "" || isSpamHit(result.URL, result.Title, result.Snippet) {
				continue
			}
			if ownSite(result.URL, org) || socialHost(result.URL) {
				continue // lo del sitio propio va por «Sitio»; las redes no se leen
			}
			mt, ok := matchItem(result.Title+" "+result.Snippet, org, m)
			if !ok {
				continue
			}
			if saveHit(ctx, deps, org, "web", mt, result.Title, result.URL, result.Snippet, time.Time{}) {
				hits++
			}
		}
	}
	if hits > 0 {
		slog.Info("watchlist/web: done", "org", org.Name, "new_hits", hits)
	}
	return hits
}

// webQuery quita el sitio propio de la búsqueda web: interesan las páginas
// de otros (aliados, prensa, gobierno, universidades).
func webQuery(q string, org models.WatchlistOrg) string {
	if org.IsBrand() {
		if h, _, ok := siteKey(org.Website); ok {
			q += " -site:" + h
		}
	}
	return q
}

// ownSite: la página es del sitio de la marca.
func ownSite(u string, org models.WatchlistOrg) bool {
	h1, _, ok1 := siteKey(u)
	h2, _, ok2 := siteKey(org.Website)
	return ok1 && ok2 && h1 == h2
}

var socialHosts = []string{"instagram.com", "facebook.com", "x.com", "twitter.com", "linkedin.com", "tiktok.com", "threads.net", "threads.com", "youtube.com"}

// socialHost: enlaces a redes que no se leen (van como enlace en la ficha).
func socialHost(u string) bool {
	h, _, ok := siteKey(u)
	if !ok {
		return false
	}
	for _, s := range socialHosts {
		if h == s || strings.HasSuffix(h, "."+s) {
			return true
		}
	}
	return false
}

// DuckDuckGo Lite responde 202 («anomaly») tras unas pocas búsquedas
// seguidas. Cuando pasa, se deja de buscar en la web por una hora.
var (
	webMu         sync.Mutex
	webBlockUntil time.Time
)

func blockWeb() {
	webMu.Lock()
	webBlockUntil = time.Now().Add(time.Hour)
	webMu.Unlock()
}

func webBlocked() bool {
	webMu.Lock()
	defer webMu.Unlock()
	return time.Now().Before(webBlockUntil)
}
