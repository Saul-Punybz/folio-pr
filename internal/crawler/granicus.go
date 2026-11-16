package crawler

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/gocolly/colly/v2"
)

// Portales de subastas «Streamline» de Granicus (portalsuplidores.<municipio>…):
// la página de búsqueda se llena con una llamada a /Auction/FillAuctionGrid que
// devuelve JSON. Guaynabo publica así sus subastas y RFP. Aquí se vuelve cada
// subasta una fila de texto para el mismo extractor de siempre.

type granicusAuction struct {
	AuctionID    int    `json:"AuctionId"`
	Number       string `json:"Number"`
	Name         string `json:"AuctionName"`
	Type         string `json:"AuctionType"`
	Stage        string `json:"AuctionStage"`
	Published    string `json:"PublicationDate"`
	DocsDeadline string `json:"SealedAttachmentsDownloadEndDate"`
	Closing      string `json:"ClosingDate"`
	Services     string `json:"ServiceTypesDescription"`
}

type granicusGrid struct {
	Data []granicusAuction `json:"Data"`
}

var reGranicusHead = regexp.MustCompile(`^\{\s*"Data"\s*:\s*\[`)

var reMSDate = regexp.MustCompile(`/Date\((-?\d+)\)/`)

// msDate: "/Date(1795527000000)/" → hora de Puerto Rico.
func msDate(s string) (time.Time, bool) {
	m := reMSDate.FindStringSubmatch(s)
	if m == nil {
		return time.Time{}, false
	}
	ms, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return time.Time{}, false
	}
	return time.UnixMilli(ms).In(time.FixedZone("AST", -4*3600)), true
}

var spanishMonths = []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio",
	"agosto", "septiembre", "octubre", "noviembre", "diciembre"}

func spanishDate(t time.Time) string {
	return fmt.Sprintf("%d de %s de %d", t.Day(), spanishMonths[t.Month()-1], t.Year())
}

func parseGranicus(raw string) (granicusGrid, bool) {
	var g granicusGrid
	s := strings.TrimSpace(raw)
	if !reGranicusHead.MatchString(s) {
		return g, false
	}
	if err := json.Unmarshal([]byte(s), &g); err != nil || g.Data == nil {
		return g, false
	}
	return g, true
}

// isGranicusJSON: el cuerpo es la rejilla JSON de un portal Granicus.
func isGranicusJSON(raw string) bool {
	_, ok := parseGranicus(raw)
	return ok
}

// granicusOpen: solo las subastas abiertas (Publicada). «En evaluación»,
// «Adjudicada», «Cancelada»… ya no reciben ofertas.
func granicusOpen(a granicusAuction) bool {
	return strings.EqualFold(strings.TrimSpace(a.Stage), "Publicada")
}

// granicusRows: una fila de texto por cada subasta abierta.
func granicusRows(raw, baseURL string) []Row {
	g, ok := parseGranicus(raw)
	if !ok {
		return nil
	}
	link := baseURL
	if u, err := url.Parse(baseURL); err == nil && u.Host != "" {
		link = u.Scheme + "://" + u.Host + "/Auction/AuctionSearch"
	}
	var rows []Row
	for _, a := range g.Data {
		if !granicusOpen(a) {
			continue
		}
		name := strings.Join(strings.Fields(a.Name), " ")
		if name == "" {
			continue
		}
		label := "Subasta Núm."
		if strings.Contains(strings.ToLower(a.Type), "propuesta") || strings.Contains(strings.ToUpper(a.Number), "RFP") {
			label = "Solicitud de Propuestas Núm."
		}
		var sb strings.Builder
		sb.WriteString(name + "\n")
		sb.WriteString(label + " " + strings.TrimSpace(a.Number) + "\n")
		if a.Type != "" {
			sb.WriteString("Tipo: " + strings.TrimSpace(a.Type) + "\n")
		}
		if s := strings.Join(strings.Fields(a.Services), " "); s != "" {
			sb.WriteString("Servicios: " + s + "\n")
		}
		if t, ok := msDate(a.Published); ok {
			sb.WriteString("Fecha de publicación: " + spanishDate(t) + "\n")
		}
		if t, ok := msDate(a.Closing); ok {
			sb.WriteString("Fecha límite de entrega de propuestas: " + spanishDate(t) + "\n")
		}
		rows = append(rows, Row{Text: strings.TrimSpace(sb.String()), Link: link, LinkText: name})
	}
	return rows
}

// granicusText: el texto de toda la rejilla (para la huella de cambios).
func granicusText(raw string) (string, bool) {
	g, ok := parseGranicus(raw)
	if !ok {
		return "", false
	}
	var sb strings.Builder
	for _, a := range g.Data {
		fmt.Fprintf(&sb, "%s | %s | %s | %s | %s | %s\n", a.Number, a.Name, a.Type, a.Stage, a.Closing, a.Published)
	}
	return sb.String(), true
}

// genericAnchor: textos de enlace que no dicen de qué trata el archivo.
var genericAnchor = regexp.MustCompile(`^(?:aviso|avisos|ver|ver pdf|pdf|descargar|descarga|download|click aqu[ií]|aqu[ií]|leer m[aá]s|m[aá]s|documento|archivo|ver aviso|ver documento|enlace)\W*$`)

// anchorWithContext: el texto del enlace; si es corto o genérico («Aviso»,
// «Ver PDF»), le suma el texto de su fila (<tr> o <li>) para que se sepa de
// qué trata antes de bajarlo y sirva de título. Máximo 200 letras.
func anchorWithContext(e *colly.HTMLElement) string {
	anchor := strings.Join(strings.Fields(e.Text), " ")
	if len([]rune(anchor)) < 25 || genericAnchor.MatchString(strings.ToLower(anchor)) {
		if row := e.DOM.Closest("tr, li"); row.Length() > 0 {
			if ctx := rowText(row); len([]rune(ctx)) > len([]rune(anchor)) && len([]rune(ctx)) <= 600 {
				anchor = ctx
			}
		}
	}
	return truncateRunes(anchor, 200)
}

func rowText(s *goquery.Selection) string {
	var parts []string
	s.Find("td, th, span, p, div, a").Each(func(_ int, c *goquery.Selection) {
		if c.Children().Length() == 0 {
			if t := strings.Join(strings.Fields(c.Text()), " "); t != "" {
				parts = append(parts, t)
			}
		}
	})
	if len(parts) == 0 {
		return strings.Join(strings.Fields(s.Text()), " ")
	}
	// Sin repetir (un <span> dentro de un <td> sale una vez).
	seen := map[string]bool{}
	out := parts[:0]
	for _, p := range parts {
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
