package watch

import (
	"context"
	"encoding/xml"
	"html"
	"log/slog"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// Sitio propio de la marca: su RSS/Atom si lo tiene; si no, las páginas de su
// sitemap cambiadas hace poco. Todo lo del sitio es de la marca (no se exige
// coincidencia), pero dos marcas pueden compartir dominio (Oasis 360 vive en
// impactpr.co): cada página va a la marca cuyo sitio es el prefijo más largo.

const sitemapWindow = 45 * 24 * time.Hour

var (
	reFeedLink = regexp.MustCompile(`(?is)<link\b[^>]*>`)
	reAttr     = regexp.MustCompile(`(?is)\b(rel|type|href)\s*=\s*["']([^"']*)["']`)
	reTitle    = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)
)

// commonFeedPaths: rutas de feed de WordPress, Wix, Squarespace, Ghost…
var commonFeedPaths = []string{"/feed/", "/blog-feed.xml", "/rss.xml", "/feed.xml", "/atom.xml", "/blog?format=rss"}

// discoverFeedLinks saca los <link rel="alternate" type="application/rss+xml|atom+xml">.
func discoverFeedLinks(page []byte, base *url.URL) []string {
	var out []string
	for _, tag := range reFeedLink.FindAll(page, -1) {
		attrs := map[string]string{}
		for _, m := range reAttr.FindAllSubmatch(tag, -1) {
			attrs[strings.ToLower(string(m[1]))] = html.UnescapeString(string(m[2]))
		}
		if !strings.Contains(strings.ToLower(attrs["rel"]), "alternate") {
			continue
		}
		t := strings.ToLower(attrs["type"])
		if t != "application/rss+xml" && t != "application/atom+xml" {
			continue
		}
		// Los feeds de comentarios de WordPress no son noticias.
		if strings.Contains(strings.ToLower(attrs["href"]), "comments") {
			continue
		}
		if u, err := base.Parse(attrs["href"]); err == nil {
			out = append(out, u.String())
		}
	}
	return out
}

// siteKey: host sin www + ruta sin extensión ni barra final.
func siteKey(raw string) (host, path string, ok bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return "", "", false
	}
	host = strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	path = strings.TrimSuffix(u.Path, "/")
	for _, ext := range []string{".html", ".htm", ".php"} {
		path = strings.TrimSuffix(path, ext)
	}
	return host, path, true
}

// siteOwner dice qué org (por id) se queda con una página de ese sitio: el de
// sitio con el prefijo de ruta más largo. uuid como string; "" si ninguno.
func siteOwner(pageURL string, orgs []models.WatchlistOrg) string {
	ph, pp, ok := siteKey(pageURL)
	if !ok {
		return ""
	}
	best, bestLen := "", -1
	for _, o := range orgs {
		if !o.IsBrand() || o.Website == "" {
			continue
		}
		h, p, ok := siteKey(o.Website)
		if !ok || h != ph {
			continue
		}
		if p == "" || pp == p || strings.HasPrefix(pp, p) {
			if len(p) > bestLen {
				best, bestLen = o.ID.String(), len(p)
			}
		}
	}
	return best
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod"`
}

// parseSitemapRecent devuelve las páginas con lastmod después de since.
func parseSitemapRecent(body []byte, since time.Time) []sitemapURL {
	var set struct {
		URLs []sitemapURL `xml:"url"`
	}
	if err := xml.Unmarshal(body, &set); err != nil {
		return nil
	}
	var out []sitemapURL
	for _, u := range set.URLs {
		t := parseLastMod(u.LastMod)
		if !t.IsZero() && t.After(since) {
			out = append(out, u)
		}
	}
	return out
}

func parseLastMod(s string) time.Time {
	s = strings.TrimSpace(s)
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05Z07:00", "2006-01-02"} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// findSiteFeed busca el feed del sitio: enlace en la portada, luego rutas comunes.
func findSiteFeed(ctx context.Context, website string) (string, []scraper.FeedItem) {
	base, err := url.Parse(website)
	if err != nil || base.Host == "" {
		return "", nil
	}
	var candidates []string
	if page, _, err := fetch(ctx, website, "text/html"); err == nil {
		candidates = append(candidates, discoverFeedLinks(page, base)...)
	}
	root := &url.URL{Scheme: base.Scheme, Host: base.Host}
	for _, p := range commonFeedPaths {
		u, _ := root.Parse(p)
		candidates = append(candidates, u.String())
	}
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] || ctx.Err() != nil {
			continue
		}
		seen[c] = true
		items, err := parseFeed(ctx, c)
		if err == nil && len(items) > 0 {
			return c, items
		}
	}
	return "", nil
}

// ScanSite lee el sitio propio de la marca.
func ScanSite(ctx context.Context, org models.WatchlistOrg, all []models.WatchlistOrg, deps Deps) int {
	if org.Website == "" {
		return 0
	}
	mine := func(u string) bool { return siteOwner(u, all) == org.ID.String() }
	saved := 0
	feedURL, items := findSiteFeed(ctx, org.Website)
	if feedURL != "" {
		for _, it := range items {
			if saved >= maxResultsPerAgent {
				break
			}
			if it.Link == "" || !mine(it.Link) {
				continue
			}
			if saveHit(ctx, deps, org, "sitio", match{}, html.UnescapeString(it.Title), it.Link, stripHTML(it.Description), it.Published) {
				saved++
			}
		}
	} else {
		base, err := url.Parse(org.Website)
		if err != nil {
			return 0
		}
		sm, _ := (&url.URL{Scheme: base.Scheme, Host: base.Host}).Parse("/sitemap.xml")
		body, code, err := fetch(ctx, sm.String(), "application/xml")
		if err != nil {
			slog.Info("watchlist/sitio: sin feed ni sitemap", "org", org.Name, "code", code)
			return 0
		}
		for _, u := range parseSitemapRecent(body, time.Now().Add(-sitemapWindow)) {
			if saved >= maxResultsPerAgent || ctx.Err() != nil {
				break
			}
			if !mine(u.Loc) {
				continue
			}
			title := u.Loc
			if page, _, err := fetch(ctx, u.Loc, "text/html"); err == nil {
				if m := reTitle.FindSubmatch(page); m != nil {
					title = strings.TrimSpace(html.UnescapeString(string(m[1])))
				}
			}
			if saveHit(ctx, deps, org, "sitio", match{}, title, u.Loc, "", parseLastMod(u.LastMod)) {
				saved++
			}
		}
	}
	if saved > 0 {
		slog.Info("watchlist/sitio: done", "org", org.Name, "feed", feedURL, "new_hits", saved)
	}
	return saved
}
