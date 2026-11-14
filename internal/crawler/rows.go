package crawler

import (
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/html"
)

// Row es una fila de un listado: su texto y el primer enlace que trae.
type Row struct {
	Text     string
	Link     string
	LinkText string
}

const (
	minRowRunes = 30
	maxRowRunes = 4000
)

// blockTags: elementos que cortan línea al pasar el HTML a texto.
var blockTags = map[string]bool{
	"div": true, "p": true, "br": true, "li": true, "tr": true, "td": true, "th": true,
	"h1": true, "h2": true, "h3": true, "h4": true, "h5": true, "h6": true, "section": true,
	"article": true, "ul": true, "ol": true, "table": true, "dt": true, "dd": true, "header": true,
	"footer": true, "blockquote": true, "address": true, "figcaption": true,
}

// nodeText pasa un nodo a texto con saltos de línea en los bloques.
func nodeText(n *html.Node) string {
	var sb strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			sb.WriteString(n.Data)
		case html.ElementNode:
			switch n.Data {
			case "script", "style", "noscript", "select", "option", "template", "svg", "button":
				return
			case "input":
				return
			}
			block := blockTags[n.Data]
			if block {
				sb.WriteByte('\n')
			}
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				walk(c)
			}
			if block {
				sb.WriteByte('\n')
			}
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return tidyLines(sb.String())
}

var reSpaces = regexp.MustCompile(`[ \t\x{00a0}\x{200b}]+`)

// tidyLines colapsa espacios y quita líneas vacías.
func tidyLines(s string) string {
	lines := strings.Split(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		l = strings.TrimSpace(reSpaces.ReplaceAllString(l, " "))
		if l != "" {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// SplitHTMLRows parte un listado en filas: cada <li>, <tr> o <article> que
// no contiene otra fila adentro (la fila "hoja"), con su texto y su primer
// enlace (resuelto contra baseURL). Se ignoran menús (<nav>), pies de página
// y filas muy cortas o muy largas. Es la entrada para pasar cada convocatoria
// de un listado (p. ej. el Registro Único de Subastas) por extract.Opportunity.
func SplitHTMLRows(rawHTML, baseURL string) []Row {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil
	}
	doc.Find("script, style, noscript, nav, footer, select, template").Remove()
	base, _ := url.Parse(baseURL)

	var rows []Row
	seen := map[string]bool{}
	doc.Find("li, tr, article").Each(func(_ int, s *goquery.Selection) {
		if s.Find("li, tr, article").Length() > 0 {
			return
		}
		if len(s.Nodes) == 0 {
			return
		}
		text := nodeText(s.Nodes[0])
		n := utf8.RuneCountInString(text)
		if n < minRowRunes || n > maxRowRunes || seen[text] {
			return
		}
		seen[text] = true
		r := Row{Text: text}
		if a := s.Find("a[href]").First(); a.Length() > 0 {
			href := strings.TrimSpace(a.AttrOr("href", ""))
			if u, err := url.Parse(href); err == nil && base != nil && href != "" &&
				!strings.HasPrefix(href, "#") && !strings.HasPrefix(strings.ToLower(href), "javascript:") &&
				!strings.HasPrefix(strings.ToLower(href), "mailto:") {
				abs := base.ResolveReference(u)
				abs.Fragment = ""
				r.Link = abs.String()
				r.LinkText = tidyLines(strings.ReplaceAll(a.Text(), "\n", " "))
			}
		}
		rows = append(rows, r)
	})
	return rows
}

// HTMLText: el texto de toda la página (sin scripts, menús ni pies), con
// saltos de línea por bloque. Sirve cuando el texto "de artículo" de la
// página quedó vacío o pobre.
func HTMLText(rawHTML string) string {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return ""
	}
	doc.Find("script, style, noscript, nav, footer, header, select, template").Remove()
	body := doc.Find("body")
	if body.Length() == 0 || len(body.Nodes) == 0 {
		return ""
	}
	return nodeText(body.Nodes[0])
}

// reBlockStart: línea que empieza una convocatoria dentro de un texto largo
// (un PDF con varias subastas): "Subasta Núm. 2027-016", "RFP 2026-004"…
var reBlockStart = regexp.MustCompile(`(?i)^\s*(?:\d+[.)]\s*)?(?:aviso\s+de\s+)?(?:subasta|rfp|rfq|rfi|ifb|licitaci[oó]n|solicitud\s+de\s+(?:propuestas?|cualificaciones|cotizaciones))\b[^\n]{0,40}\d`)

// SplitTextBlocks parte un texto (de PDF) en bloques, uno por cada línea que
// arranca una convocatoria. Con menos de dos arranques devuelve nil.
func SplitTextBlocks(text string) []string {
	lines := strings.Split(text, "\n")
	var blocks []string
	var cur []string
	starts := 0
	for _, l := range lines {
		if reBlockStart.MatchString(l) {
			starts++
			if len(cur) > 0 {
				blocks = append(blocks, strings.Join(cur, "\n"))
				cur = nil
			}
		}
		cur = append(cur, l)
	}
	if len(cur) > 0 {
		blocks = append(blocks, strings.Join(cur, "\n"))
	}
	if starts < 2 {
		return nil
	}
	// Lo que va antes del primer arranque es encabezado, no convocatoria.
	if !reBlockStart.MatchString(strings.SplitN(blocks[0], "\n", 2)[0]) {
		blocks = blocks[1:]
	}
	return blocks
}
