package archivo

import (
	"bytes"
	"encoding/json"
	"html"
	"net/url"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html/charset"
)

// Page es lo que se saca de una página: metadatos y texto limpio.
type Page struct {
	Title       string
	Site        string
	Canonical   string
	Lang        string
	Image       string
	Description string
	Authors     []string
	Published   *time.Time
	Text        string
}

// MinCopyRunes: menos texto que esto no se considera copia del contenido
// (la página probablemente se arma con JavaScript o está bloqueada).
const MinCopyRunes = 200

// MaxTextRunes: tope del texto archivado por mención.
const MaxTextRunes = 200000

// ParsePage lee el HTML de una página y saca título, medio, autores, fecha,
// idioma, imagen y texto limpio. pageURL sirve para resolver enlaces
// relativos y para el nombre del medio si la página no lo dice.
func ParsePage(raw []byte, pageURL string) Page {
	var p Page
	rd, err := charset.NewReader(bytes.NewReader(raw), "text/html")
	if err != nil {
		rd = bytes.NewReader(raw)
	}
	doc, err := goquery.NewDocumentFromReader(rd)
	if err != nil {
		return p
	}
	base, _ := url.Parse(pageURL)
	abs := func(ref string) string {
		ref = strings.TrimSpace(html.UnescapeString(ref))
		if ref == "" {
			return ""
		}
		if u, err := url.Parse(ref); err == nil && base != nil {
			return base.ResolveReference(u).String()
		}
		return ref
	}
	meta := func(keys ...string) string {
		for _, k := range keys {
			var v string
			doc.Find("meta").EachWithBreak(func(_ int, s *goquery.Selection) bool {
				n, _ := s.Attr("property")
				if n == "" {
					n, _ = s.Attr("name")
				}
				if n == "" {
					n, _ = s.Attr("itemprop")
				}
				if strings.EqualFold(n, k) {
					c, _ := s.Attr("content")
					if c = collapse(c); c != "" {
						v = c
						return false
					}
				}
				return true
			})
			if v != "" {
				return v
			}
		}
		return ""
	}

	graph := jsonLDNodes(doc)
	article := pickArticle(graph)

	// Título.
	site := collapse(meta("og:site_name", "application-name"))
	if site == "" {
		site = ldName(article, "publisher", graph)
	}
	if site == "" {
		site = SiteFromURL(pageURL)
	}
	p.Site = site
	title := firstNonEmpty(meta("og:title", "twitter:title"), ldString(article, "headline"),
		collapse(doc.Find("title").First().Text()), collapse(doc.Find("h1").First().Text()))
	p.Title = stripSiteSuffix(html.UnescapeString(title), site)

	// Descripción.
	p.Description = firstNonEmpty(meta("og:description", "description", "twitter:description"), ldString(article, "description"))

	// Canónica.
	canon := ""
	doc.Find("link[rel=canonical]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		canon, _ = s.Attr("href")
		return canon == ""
	})
	if canon == "" {
		canon = meta("og:url")
	}
	p.Canonical = abs(canon)

	// Idioma.
	lang, _ := doc.Find("html").First().Attr("lang")
	if lang == "" {
		lang = meta("og:locale", "inLanguage")
	}
	p.Lang = shortLang(lang)

	// Imagen.
	p.Image = abs(firstNonEmpty(meta("og:image", "og:image:url", "twitter:image", "twitter:image:src"), ldImage(article)))

	// Autores.
	var authors []string
	authors = append(authors, ldAuthors(article, graph)...)
	if len(authors) == 0 {
		authors = append(authors, splitAuthors(meta("author", "article:author", "parsely-author", "dc.creator", "citation_author"))...)
	}
	if len(authors) == 0 {
		// Microdatos (YouTube): <span itemprop="author"><link itemprop="name" content="Canal">
		doc.Find(`[itemprop=author] [itemprop=name]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			n, _ := s.Attr("content")
			if n == "" {
				n = s.Text()
			}
			authors = append(authors, n)
			return false
		})
	}
	if len(authors) == 0 {
		doc.Find(`a[rel=author], [itemprop=author], .byline, .author-name, [class*=byline]`).EachWithBreak(func(_ int, s *goquery.Selection) bool {
			t := collapse(s.First().Text())
			if utf8.RuneCountInString(t) <= 80 {
				authors = append(authors, splitAuthors(t)...)
			}
			return len(authors) == 0
		})
	}
	p.Authors = cleanAuthors(authors, site)

	// Fecha.
	for _, c := range []string{ldString(article, "datePublished"), meta("article:published_time", "datePublished", "pubdate", "date", "dc.date", "dc.date.issued", "og:published_time", "sailthru.date"), timeAttr(doc)} {
		if t := ParseDate(c); t != nil {
			p.Published = t
			break
		}
	}

	p.Text = extractText(doc)
	return p
}

var dateLayouts = []string{
	time.RFC3339Nano, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02T15:04:05-0700",
	"2006-01-02T15:04:05.000-0700", time.RFC1123Z, time.RFC1123, "2006-01-02", "2006/01/02",
	"January 2, 2006", "2 January 2006",
}

// ParseDate lee una fecha en los formatos que usan los medios. nil si no puede.
func ParseDate(s string) *time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	for _, l := range dateLayouts {
		loc := time.UTC
		if l == "2006-01-02T15:04:05" || l == "2006-01-02 15:04:05" || l == "2006-01-02" {
			loc = puertoRico
		}
		if t, err := time.ParseInLocation(l, s, loc); err == nil && t.Year() > 1990 && t.Before(time.Now().AddDate(0, 0, 2)) {
			t = t.UTC()
			return &t
		}
	}
	return nil
}

func timeAttr(doc *goquery.Document) string {
	v := ""
	doc.Find("time[datetime]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		v, _ = s.Attr("datetime")
		return ParseDate(v) == nil
	})
	return v
}

func shortLang(l string) string {
	l = strings.ToLower(strings.TrimSpace(l))
	if i := strings.IndexAny(l, "-_"); i > 0 {
		l = l[:i]
	}
	if len(l) < 2 || len(l) > 3 {
		return ""
	}
	return l
}

// ── JSON-LD ──────────────────────────────────────────────────────

type node = map[string]any

// jsonLDNodes junta todos los nodos de los <script type="application/ld+json">
// (incluidos los de @graph y arreglos).
func jsonLDNodes(doc *goquery.Document) []node {
	var out []node
	var walk func(v any)
	walk = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, i := range x {
				walk(i)
			}
		case node:
			out = append(out, x)
			if g, ok := x["@graph"]; ok {
				walk(g)
			}
		}
	}
	doc.Find(`script[type="application/ld+json"]`).Each(func(_ int, s *goquery.Selection) {
		var v any
		if err := json.Unmarshal([]byte(strings.TrimSpace(s.Text())), &v); err == nil {
			walk(v)
		}
	})
	return out
}

func ldTypes(n node) []string {
	switch t := n["@type"].(type) {
	case string:
		return []string{t}
	case []any:
		var out []string
		for _, i := range t {
			if s, ok := i.(string); ok {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// pickArticle elige el nodo que describe la nota (Article, NewsArticle,
// BlogPosting, Reportage…, VideoObject).
func pickArticle(nodes []node) node {
	for _, n := range nodes {
		for _, t := range ldTypes(n) {
			if strings.Contains(t, "Article") || t == "BlogPosting" || t == "VideoObject" || t == "SocialMediaPosting" {
				return n
			}
		}
	}
	return nil
}

func ldString(n node, key string) string {
	if n == nil {
		return ""
	}
	if s, ok := n[key].(string); ok {
		return collapse(s)
	}
	return ""
}

// ldRef resuelve un valor de JSON-LD que puede ser objeto, texto o {"@id"}.
func ldRef(v any, graph []node) []node {
	switch x := v.(type) {
	case node:
		if id, ok := x["@id"].(string); ok && len(x) == 1 {
			for _, g := range graph {
				if g["@id"] == id {
					return []node{g}
				}
			}
			return nil
		}
		if _, hasName := x["name"]; !hasName {
			if id, ok := x["@id"].(string); ok {
				for _, g := range graph {
					if g["@id"] == id {
						return []node{g}
					}
				}
			}
		}
		return []node{x}
	case []any:
		var out []node
		for _, i := range x {
			out = append(out, ldRef(i, graph)...)
		}
		return out
	case string:
		return []node{{"name": x}}
	}
	return nil
}

func ldName(n node, key string, graph []node) string {
	if n == nil {
		return ""
	}
	for _, r := range ldRef(n[key], graph) {
		if s, ok := r["name"].(string); ok && collapse(s) != "" {
			return collapse(html.UnescapeString(s))
		}
	}
	return ""
}

func ldAuthors(n node, graph []node) []string {
	if n == nil {
		return nil
	}
	var out []string
	for _, r := range ldRef(n["author"], graph) {
		if s, ok := r["name"].(string); ok {
			out = append(out, html.UnescapeString(s))
		}
	}
	return out
}

func ldImage(n node) string {
	if n == nil {
		return ""
	}
	switch x := n["image"].(type) {
	case string:
		return x
	case node:
		s, _ := x["url"].(string)
		return s
	case []any:
		for _, i := range x {
			switch y := i.(type) {
			case string:
				return y
			case node:
				if s, _ := y["url"].(string); s != "" {
					return s
				}
			}
		}
	}
	return ""
}

// ── Autores ──────────────────────────────────────────────────────

var rePor = regexp.MustCompile(`(?i)^(por|by|de|escrito por|written by|texto:?)\s+`)

// splitAuthors parte «A y B», «A; B», «A and B» y «A, B» (si cada parte
// parece nombre y apellido; «Rivera, Itzel» no se parte).
func splitAuthors(s string) []string {
	s = collapse(html.UnescapeString(s))
	if s == "" || strings.HasPrefix(strings.ToLower(s), "http") {
		return nil
	}
	s = rePor.ReplaceAllString(s, "")
	seps := regexp.MustCompile(`(?i)\s*(?:;|\s&\s|\sand\s|\sy\s|\se\s|\|)\s*`)
	parts := seps.Split(s, -1)
	var out []string
	for _, p := range parts {
		if strings.Contains(p, ",") {
			sub := strings.Split(p, ",")
			ok := len(sub) > 1
			for _, x := range sub {
				if len(strings.Fields(x)) < 2 {
					ok = false
				}
			}
			if ok {
				out = append(out, sub...)
				continue
			}
		}
		out = append(out, p)
	}
	return out
}

var malosAutores = map[string]bool{"admin": true, "administrator": true, "administrador": true, "wordpress": true,
	"author": true, "autor": true, "editor": true, "user": true, "usuario": true, "null": true, "undefined": true}

func cleanAuthors(in []string, site string) []string {
	var out []string
	seen := map[string]bool{}
	for _, a := range in {
		a = collapse(rePor.ReplaceAllString(collapse(a), ""))
		a = strings.Trim(a, " ,;.|-")
		k := fold(a)
		if a == "" || seen[k] || malosAutores[k] || strings.HasPrefix(k, "http") || utf8.RuneCountInString(a) > 80 || strings.ContainsAny(a, "<>{}") {
			continue
		}
		seen[k] = true
		out = append(out, a)
		if len(out) >= 20 {
			break
		}
	}
	return out
}

// ── Título, medio, URL ───────────────────────────────────────────

var sepsTitulo = []string{" | ", " - ", " – ", " — ", " :: ", " • "}

// stripSiteSuffix quita « | Medio» del final del título.
func stripSiteSuffix(title, site string) string {
	title = collapse(title)
	for _, sep := range sepsTitulo {
		if i := strings.LastIndex(title, sep); i > 0 {
			suf := collapse(title[i+len(sep):])
			if site != "" && (fold(suf) == fold(site) || strings.Contains(fold(site), fold(suf)) && utf8.RuneCountInString(suf) >= 4) {
				return strings.TrimSpace(title[:i])
			}
		}
	}
	return title
}

// SplitTitleSite separa «Título - Medio» (como lo arma Google News).
func SplitTitleSite(title string) (string, string) {
	title = collapse(title)
	for _, sep := range sepsTitulo {
		if i := strings.LastIndex(title, sep); i > 0 {
			if suf := collapse(title[i+len(sep):]); utf8.RuneCountInString(suf) <= 40 && utf8.RuneCountInString(suf) >= 2 {
				return strings.TrimSpace(title[:i]), suf
			}
		}
	}
	return title, ""
}

var sitiosConocidos = map[string]string{
	"elnuevodia.com": "El Nuevo Día", "newsismybusiness.com": "News is My Business", "primerahora.com": "Primera Hora",
	"noticel.com": "Noticel", "elvocero.com": "El Vocero", "metro.pr": "Metro Puerto Rico", "presenciapr.com": "Periódico Presencia",
	"periodicolaperla.com": "Periódico La Perla del Sur", "elcalce.com": "El Calce", "sincomillas.com": "Sin Comillas",
	"wapa.tv": "WAPA TV", "telemundopr.com": "Telemundo Puerto Rico", "noticiascmu.com": "NotiCel", "centropr.pr": "Centro de Periodismo Investigativo",
	"periodismoinvestigativo.com": "Centro de Periodismo Investigativo", "elmundo.es": "El Mundo", "nytimes.com": "The New York Times",
	"apnews.com": "Associated Press", "lisc.org": "LISC", "nff.org": "Nonprofit Finance Fund", "projectmakerspr.org": "Project Makers",
	"vimenti.org": "Vimenti", "impactpr.co": "Platform for Social Impact", "youtube.com": "YouTube", "bsky.app": "Bluesky",
}

// SiteFromURL da un nombre de medio legible a partir del dominio.
func SiteFromURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return ""
	}
	host := strings.TrimPrefix(strings.ToLower(u.Hostname()), "www.")
	for h, n := range sitiosConocidos {
		if host == h || strings.HasSuffix(host, "."+h) {
			return n
		}
	}
	return host
}

// ── Texto limpio ─────────────────────────────────────────────────

var quitar = "script, style, noscript, nav, header, footer, aside, form, iframe, svg, button, select, template, figcaption, " +
	"[role=navigation], [role=banner], [role=contentinfo], [aria-hidden=true], .share, .sharing, .social, .related, .comments, " +
	"#comments, .newsletter, .advertisement, .ad, .sidebar, .breadcrumb, .breadcrumbs, .menu, .tags, .tag-list, .author-bio"

var candidatos = []string{`[itemprop=articleBody]`, `[data-hook=post-description]`, `.entry-content`, `.post-content`, `.article-body`,
	`.article-content`, `.story-body`, `.c-article-body`, `article`, `main`, `#content`, `body`}

var boilerplate = []string{"todos los derechos reservados", "all rights reserved", "suscríbete", "suscribete", "síguenos", "siguenos",
	"lee también", "lee tambien", "te puede interesar", "read more", "subscribe", "sign up for", "compartir en", "share on",
	"política de privacidad", "privacy policy", "términos de uso", "terms of use", "newsletter", "cookies"}

// extractText saca el texto del cuerpo: el contenedor con más texto de
// párrafos, sin menús, anuncios ni pies de página.
func extractText(doc *goquery.Document) string {
	doc.Find(quitar).Remove()
	var best *goquery.Selection
	bestScore := -1
	for _, sel := range candidatos {
		doc.Find(sel).Each(func(_ int, s *goquery.Selection) {
			score := 0
			s.Find("p").Each(func(_ int, p *goquery.Selection) { score += utf8.RuneCountInString(collapse(p.Text())) })
			if score > bestScore {
				best, bestScore = s, score
			}
		})
		if bestScore >= 400 && sel != "body" {
			break
		}
	}
	if best == nil {
		return ""
	}
	var lines []string
	last := ""
	best.Find("p, h2, h3, h4, li, blockquote").Each(func(_ int, s *goquery.Selection) {
		tag := goquery.NodeName(s)
		if (tag == "li" || tag == "blockquote") && s.Find("p").Length() > 0 {
			return
		}
		t := collapse(s.Text())
		n := utf8.RuneCountInString(t)
		isHead := strings.HasPrefix(tag, "h")
		if t == "" || t == last || (!isHead && n < 30) || (isHead && n < 3) || n > 6000 {
			return
		}
		lf := strings.ToLower(t)
		for _, b := range boilerplate {
			if strings.Contains(lf, b) && n < 160 {
				return
			}
		}
		last = t
		lines = append(lines, t)
	})
	text := strings.Join(lines, "\n\n")
	if r := []rune(text); len(r) > MaxTextRunes {
		text = string(r[:MaxTextRunes])
	}
	return text
}
