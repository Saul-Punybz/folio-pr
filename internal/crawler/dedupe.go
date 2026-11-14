package crawler

import (
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Saul-Punybz/folio/internal/scraper"
)

// DedupeKey es la huella de una convocatoria, para que la misma vista en
// dos URL (el listado y su ficha, la página y su PDF) quede en una fila:
//
//	con referencia:           "ref:" + referencia normalizada + "|" + agencia normalizada
//	sin referencia, con fecha: "td:"  + título normalizado + "|" + AAAA-MM-DD
//	sin fecha, con título:     "ta:"  + título normalizado + "|" + agencia normalizada
//	                          (la misma página servida con ?tab=… distintos es una sola)
//	si no:                    "url:" + URL canónica
func DedupeKey(reference, agency, title string, deadline *time.Time, sourceURL string) string {
	if r := NormalizeReference(reference); r != "" {
		return "ref:" + r + "|" + NormalizeAgency(agency)
	}
	t := normWords(title)
	if len(t) > 120 {
		t = t[:120]
	}
	if t != "" && deadline != nil {
		return "td:" + t + "|" + deadline.Format("2006-01-02")
	}
	if t != "" {
		return "ta:" + t + "|" + NormalizeAgency(agency)
	}
	return "url:" + scraper.CanonicalizeURL(sourceURL)
}

func fold(s string) string {
	var sb strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'à', 'ä', 'â':
			r = 'a'
		case 'é', 'è', 'ë', 'ê':
			r = 'e'
		case 'í', 'ì', 'ï', 'î':
			r = 'i'
		case 'ó', 'ò', 'ö', 'ô':
			r = 'o'
		case 'ú', 'ù', 'ü', 'û':
			r = 'u'
		case 'ñ':
			r = 'n'
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// tokens: palabras y números (sin acentos, en minúsculas).
func tokens(s string) []string {
	return strings.FieldsFunc(fold(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

func normWords(s string) string { return strings.Join(tokens(s), " ") }

// Palabras que no distinguen una referencia de otra.
var refStop = map[string]bool{
	"subasta": true, "formal": true, "informal": true, "num": true, "numero": true, "no": true,
	"nro": true, "aviso": true, "de": true, "del": true, "la": true, "el": true, "solicitud": true,
	"propuesta": true, "propuestas": true, "opportunity": true, "number": true, "ref": true,
	"referencia": true, "publica": true, "general": true,
}

// NormalizeReference: "RFP 2026-001" y "RFP 2026-1" → "rfp-2026-1";
// "Subasta Formal Núm. 2026-27-53" → "2026-27-53".
func NormalizeReference(ref string) string {
	var out []string
	for _, t := range tokens(ref) {
		if refStop[t] {
			continue
		}
		if n, err := strconv.ParseUint(t, 10, 64); err == nil {
			t = strconv.FormatUint(n, 10)
		}
		out = append(out, t)
	}
	return strings.Join(out, "-")
}

var reAcronym = regexp.MustCompile(`\(([A-Za-zÁÉÍÓÚÑáéíóúñ0-9&\-]{2,12})\)`)

// NormalizeAgency: la sigla entre paréntesis si la hay ("Autoridad de
// Acueductos y Alcantarillados (AAA)" → "aaa"); si no, las palabras.
func NormalizeAgency(agency string) string {
	if m := reAcronym.FindStringSubmatch(agency); m != nil {
		return strings.Join(tokens(m[1]), "")
	}
	a := strings.Join(tokens(agency), " ")
	if len(a) > 80 {
		a = a[:80]
	}
	return a
}
