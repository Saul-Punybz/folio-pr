// Package watch escanea menciones de marcas, temas y personas por
// coincidencia de términos (alias, exclusiones, hashtags), sin IA.
package watch

import (
	"context"
	"log/slog"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/archivo"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

const (
	maxResultsPerAgent = 10
	agentTimeout       = 30 * time.Second
	scanTimeout        = 2 * time.Hour
)

// Deps groups dependencies needed by all agents.
type Deps struct {
	Orgs     *models.WatchlistOrgStore
	Hits     *models.WatchlistHitStore
	Articles *models.ArticleStore
}

// RunWatchlistScan is the main entry point called by the worker cron.
// It processes orgs SEQUENTIALLY to keep resource usage low.
func RunWatchlistScan(ctx context.Context, deps Deps) {
	ctx, cancel := context.WithTimeout(ctx, scanTimeout)
	defer cancel()

	// Si la primera admin se creó después del arranque, la plantilla entra aquí.
	SeedBrandTemplates(ctx, deps.Orgs)

	slog.Info("watchlist: starting scan")
	start := time.Now()

	orgs, err := deps.Orgs.ListActive(ctx)
	if err != nil {
		slog.Error("watchlist: list active orgs", "err", err)
		return
	}

	if len(orgs) == 0 {
		slog.Info("watchlist: no active orgs to scan")
		return
	}

	m := Matcher{Context: brandContext(orgs), Brands: brandTerms(orgs)}
	totalHits := 0
	for _, org := range orgs {
		if ctx.Err() != nil {
			break
		}
		totalHits += scanOrg(ctx, org, orgs, m, deps)
	}

	slog.Info("watchlist: scan complete",
		"orgs", len(orgs),
		"new_hits", totalHits,
		"duration", time.Since(start).Round(time.Millisecond),
	)

	// Lo nuevo se archiva enseguida: copia del contenido, cita y respaldo.
	RunArchive(ctx, deps.Hits, ArchiveRunCap)
}

// brandContext: nombres y alias de todas las marcas activas (dan contexto a
// un alias corto como «PSI»).
func brandContext(orgs []models.WatchlistOrg) []string {
	var out []string
	for _, o := range orgs {
		if o.IsBrand() {
			out = append(out, o.Name)
			out = append(out, o.Aliases...)
		}
	}
	return out
}

// scanOrg corre todas las fuentes, una tras otra, para un org.
func scanOrg(ctx context.Context, org models.WatchlistOrg, all []models.WatchlistOrg, m Matcher, deps Deps) int {
	slog.Info("watchlist: scanning org", "name", org.Name, "kind", org.Kind)
	hits := 0

	queries := buildSearchQueries(org)

	hits += ScanGoogleNews(ctx, org, queries, m, deps)
	hits += ScanBingNews(ctx, org, queries, m, deps)
	hits += ScanWeb(ctx, org, queries, m, deps)
	hits += ScanLocalArticles(ctx, org, m, deps)
	if len(youtubeChannels(org)) > 0 {
		hits += ScanYouTube(ctx, org, m, deps)
	}
	hits += ScanReddit(ctx, org, queries, m, deps)
	hits += ScanAlerts(ctx, org, m, deps)
	hits += ScanPodcasts(ctx, org, deps)
	if org.IsBrand() {
		hits += ScanBluesky(ctx, org, m, deps)
		hits += ScanMastodon(ctx, org, m, deps)
		hits += ScanSite(ctx, org, all, deps)
	}

	slog.Info("watchlist: org scan complete", "name", org.Name, "new_hits", hits)
	return hits
}

// buildSearchQueries arma hasta 5 búsquedas. En una marca, una por alias
// (entre comillas; las de varias palabras o cortas con «Puerto Rico»). En
// temas y personas, nombre y palabras clave + «Puerto Rico».
func buildSearchQueries(org models.WatchlistOrg) []string {
	if org.IsBrand() && len(org.Aliases)+len(org.ContextAliases) > 0 {
		var qs []string
		for _, a := range append(append([]string{}, org.Aliases...), org.ContextAliases...) {
			a = strings.TrimSpace(a)
			if a == "" || strings.ContainsAny(a, "._@") {
				continue // dominios y handles no sirven de búsqueda de noticias
			}
			q := `"` + a + `"`
			if weakTerm(a) || isContextAlias(a, org) || strings.Contains(a, " ") {
				q += " Puerto Rico"
			}
			qs = append(qs, q)
			if len(qs) >= 4 {
				break
			}
		}
		// Personas clave, siempre con Puerto Rico.
		for i, p := range org.People {
			if i < 2 && strings.TrimSpace(p.Name) != "" {
				qs = append(qs, `"`+strings.TrimSpace(p.Name)+`" Puerto Rico`)
			}
		}
		// Hashtags distintivos como búsqueda (sin #: los buscadores lo ignoran).
		n := 0
		for _, h := range org.Hashtags {
			h = normTag(h)
			if h == "" || !distinctiveTag(h, org) || n >= 2 {
				continue
			}
			dup := false
			for _, q := range qs {
				if strings.EqualFold(strings.Trim(q, `"`), h) {
					dup = true
				}
			}
			if !dup {
				qs = append(qs, h)
				n++
			}
		}
		return qs
	}
	queries := []string{org.Name + " Puerto Rico"}
	for i, kw := range org.Keywords {
		if i >= 4 {
			break
		}
		if !strings.EqualFold(kw, org.Name) {
			queries = append(queries, kw+" Puerto Rico")
		}
	}
	return queries
}

// youtubeChannels: los channel_id de youtube_channels y de los perfiles.
func youtubeChannels(org models.WatchlistOrg) []string {
	seen := map[string]bool{}
	var out []string
	add := func(id string) {
		id = strings.TrimSpace(id)
		if strings.HasPrefix(id, "UC") && len(id) == 24 && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	for _, c := range org.YouTubeChannels {
		add(c)
	}
	for _, s := range org.Socials {
		if s.Network != "youtube" {
			continue
		}
		add(s.Handle)
		if i := strings.Index(s.URL, "/channel/"); i >= 0 {
			add(strings.Trim(strings.SplitN(s.URL[i+len("/channel/"):], "/", 2)[0], " "))
		}
	}
	return out
}

// saveHit guarda una mención (deduplicada por URL y, en noticias y web, por
// título). true si es nueva.
func saveHit(ctx context.Context, deps Deps, org models.WatchlistOrg, source string, mt match, title, link, snippet string, published time.Time) bool {
	return saveHitWith(ctx, deps, org, source, mt, title, link, snippet, published)
}

// withPost guarda además el texto y el autor de un post de red social: ya
// vienen de la API, así que la mención nace archivada.
func withPost(p socialPost) func(*models.WatchlistHit) {
	return func(h *models.WatchlistHit) {
		h.ArchiveStatus = models.ArchiveCopia
		h.ContentText = strings.TrimSpace(p.Text)
		h.Summary = truncateStr(strings.Join(strings.Fields(p.Text), " "), 600)
		h.SiteName = archivo.PlatformFor(h.SourceType)
		if a := strings.TrimSpace(p.Author); a != "" {
			h.Authors = []string{a}
		}
	}
}

// saveHitWith es saveHit con ajustes opcionales a la mención antes de guardarla.
func saveHitWith(ctx context.Context, deps Deps, org models.WatchlistOrg, source string, mt match, title, link, snippet string, published time.Time, opts ...func(*models.WatchlistHit)) bool {
	link = unwrapRedirect(link)
	title = strings.TrimSpace(title)
	hit := &models.WatchlistHit{
		ID:         uuid.New(),
		OrgID:      org.ID,
		SourceType: source,
		Hashtag:    normTag(mt.Tag),
		Person:     mt.Person,
		Title:      truncateStr(title, 300),
		URL:        link,
		URLHash:    scraper.HashURL(link),
		Snippet:    truncateStr(strings.TrimSpace(snippet), 500),
		Sentiment:  "unknown",
	}
	switch source {
	case "google_news", "bing_news", "local", "web", "alertas":
		hit.TitleKey = titleKey(title)
	}
	if source == "google_news" {
		// Google News agrega « - Medio» al título: el título queda limpio y el medio aparte.
		if t, site := archivo.SplitTitleSite(title); site != "" {
			hit.Title = truncateStr(t, 300)
			hit.SiteName = site
		}
	}
	for _, o := range opts {
		o(hit)
	}
	if !published.IsZero() {
		p := published.UTC()
		hit.PublishedAt = &p
	}
	if err := deps.Hits.Create(ctx, hit); err != nil {
		slog.Error("watchlist: create hit", "source", source, "err", err)
		return false
	}
	return hit.ID != uuid.Nil
}

// unwrapRedirect saca la URL real de los enlaces de rastreo de Bing News
// (apiclick.aspx?url=) y Google Alerts (google.com/url?url=), y quita utm_*.
func unwrapRedirect(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return raw
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	if (host == "bing.com" && strings.Contains(u.Path, "apiclick")) || (host == "google.com" && u.Path == "/url") {
		for _, k := range []string{"url", "q"} {
			if v := u.Query().Get(k); strings.HasPrefix(v, "http") {
				return unwrapRedirect(v)
			}
		}
	}
	q := u.Query()
	changed := false
	for k := range q {
		if strings.HasPrefix(strings.ToLower(k), "utm_") {
			q.Del(k)
			changed = true
		}
	}
	if changed {
		u.RawQuery = q.Encode()
		return u.String()
	}
	return raw
}

// titleKey normaliza un título para deduplicar entre fuentes: sin el
// « - Medio» que añade Google News, sin acentos ni signos. "" si es muy corto.
func titleKey(title string) string {
	t := strings.TrimSpace(title)
	for _, sep := range []string{" - ", " | ", " — "} {
		if i := strings.LastIndex(t, sep); i > 0 && len([]rune(t[i+len(sep):])) <= 40 {
			t = t[:i]
		}
	}
	var b strings.Builder
	space := false
	for _, r := range fold(t) {
		if isWordRune(r) {
			b.WriteRune(r)
			space = false
		} else if !space && b.Len() > 0 {
			b.WriteByte(' ')
			space = true
		}
	}
	k := strings.TrimSpace(b.String())
	if len([]rune(k)) < 20 {
		return ""
	}
	return truncateStr(k, 90)
}

// match dice qué trajo una mención: un hashtag, una persona clave, o nada
// más que el nombre de la marca.
type match struct{ Tag, Person string }

// matchItem decide si una noticia o página habla de la marca: por alias, por
// uno de sus hashtags (con o sin #; sin # solo si es distintivo) o por una de
// sus personas clave (con contexto).
func matchItem(text string, org models.WatchlistOrg, m Matcher) (match, bool) {
	if Excluded(text, org) {
		return match{}, false
	}
	mt := match{Tag: findHashtag(text, org), Person: matchPerson(text, org, m)}
	if _, hit := m.Match(text, org); hit {
		return mt, true
	}
	if mt.Tag != "" && distinctiveTag(mt.Tag, org) {
		return mt, true
	}
	if mt.Person != "" {
		return mt, true
	}
	return match{}, false
}

// Para una persona clave, «Puerto Rico» solo da contexto junto a una de estas.
var socialImpactTerms = []string{"impacto social", "social impact", "nonprofit", "non-profit", "sin fines de lucro"}

// matchPerson devuelve la persona clave nombrada en el texto si hay contexto:
// el nombre de alguna marca o alias (incluso «PSI» u «Oasis»), o «Puerto Rico»
// junto a impacto social / nonprofit / sin fines de lucro. Los nombres de
// persona son comunes: sin contexto no cuentan.
func matchPerson(text string, org models.WatchlistOrg, m Matcher) string {
	if len(org.People) == 0 {
		return ""
	}
	f := fold(text)
	name := ""
	for _, p := range org.People {
		if n := fold(strings.TrimSpace(p.Name)); n != "" && containsWord(f, n) {
			name = p.Name
			break
		}
	}
	if name == "" {
		return ""
	}
	for _, c := range m.Brands {
		if c = fold(strings.TrimSpace(c)); c != "" && containsWord(f, c) {
			return name
		}
	}
	if strings.Contains(f, "puerto rico") {
		for _, t := range socialImpactTerms {
			if strings.Contains(f, t) {
				return name
			}
		}
	}
	return ""
}

// brandTerms: nombres, alias y alias con contexto de todas las marcas, más
// «Oasis»: el contexto que hace válida la mención de una persona clave.
func brandTerms(orgs []models.WatchlistOrg) []string {
	out := []string{"Oasis"}
	for _, o := range orgs {
		if o.IsBrand() {
			out = append(out, o.Name)
			out = append(out, o.Aliases...)
			out = append(out, o.ContextAliases...)
		}
	}
	return out
}

// truncateStr acorta s a maxLen runas (no bytes) para no partir acentos.
func truncateStr(s string, maxLen int) string {
	r := []rune(s)
	if len(r) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return string(r[:maxLen])
	}
	return string(r[:maxLen-3]) + "..."
}

// findHashtag: primero un #hashtag escrito tal cual; si no, un hashtag
// distintivo escrito como palabra («socialimpactpr»).
func findHashtag(text string, org models.WatchlistOrg) string {
	for _, h := range org.Hashtags {
		if h = normTag(h); h != "" && hasHashtag(text, h) {
			return h
		}
	}
	f := fold(text)
	for _, h := range org.Hashtags {
		if h = normTag(h); h != "" && distinctiveTag(h, org) && !isAliasWord(h, org) && containsWord(f, fold(h)) {
			return h
		}
	}
	return ""
}

// isAliasWord: el hashtag sin # es un alias («Vimenti»): la palabra sola es el
// nombre, no el hashtag, y no cuenta como hashtag.
func isAliasWord(h string, org models.WatchlistOrg) bool {
	fh := fold(h)
	for _, a := range matchTerms(org) {
		if fold(strings.TrimSpace(a)) == fh {
			return true
		}
	}
	return false
}
