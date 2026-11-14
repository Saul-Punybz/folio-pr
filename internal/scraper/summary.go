package scraper

import (
	"html"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// SummaryMaxRunes es el largo máximo (en runas, no bytes) del resumen por reglas.
const SummaryMaxRunes = 300

// minParagraphRunes: un párrafo más corto que esto (fecha, firma, "Por Redacción")
// no sirve de resumen; se busca el siguiente.
const minParagraphRunes = 60

// maxTags limita cuántas categorías del RSS se guardan como tags.
const maxTags = 8

// RuleSummary devuelve el primer candidato no vacío, limpio de HTML y espacios,
// recortado a SummaryMaxRunes por palabra. Los candidatos van en orden de
// preferencia: descripción del RSS, meta description, primer párrafo del texto.
// No usa IA.
func RuleSummary(candidates ...string) string {
	for _, c := range candidates {
		t := normalizeSpace(html.UnescapeString(stripTags(c)))
		if t == "" {
			continue
		}
		return TruncateWords(t, SummaryMaxRunes)
	}
	return ""
}

// FirstParagraph devuelve el primer párrafo con sustancia del texto limpio
// (separado por saltos de línea). Si ninguno llega al mínimo, devuelve el más
// largo de los primeros que encuentre.
func FirstParagraph(text string) string {
	var best string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if utf8.RuneCountInString(line) >= minParagraphRunes {
			return line
		}
		if utf8.RuneCountInString(line) > utf8.RuneCountInString(best) {
			best = line
		}
	}
	return best
}

// TruncateWords recorta s a lo sumo max runas sin partir palabras ni caracteres
// multibyte (acentos, ñ). Si recorta, termina en "…" (que cuenta dentro de max).
func TruncateWords(s string, max int) string {
	s = strings.TrimSpace(s)
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max == 1 {
		return "…"
	}
	cut := r[:max-1] // deja sitio para "…"
	// Si el corte cae dentro de una palabra, retrocede al último espacio,
	// siempre que no se pierda más de la mitad del texto.
	if !unicode.IsSpace(r[max-1]) {
		for i := len(cut) - 1; i > len(cut)/2; i-- {
			if unicode.IsSpace(cut[i]) {
				cut = cut[:i]
				break
			}
		}
	}
	out := strings.TrimRightFunc(string(cut), func(c rune) bool {
		return unicode.IsSpace(c) || strings.ContainsRune(",;:-–—", c)
	})
	return out + "…"
}

// NormalizeTags limpia las categorías del RSS: sin HTML, sin espacios de más,
// sin duplicados (sin distinguir mayúsculas), máximo maxTags. Nunca devuelve nil.
func NormalizeTags(cats []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, c := range cats {
		c = normalizeSpace(html.UnescapeString(stripTags(c)))
		if c == "" || utf8.RuneCountInString(c) > 60 {
			continue
		}
		k := strings.ToLower(c)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, c)
		if len(out) >= maxTags {
			break
		}
	}
	return out
}

var reMetaDescription = regexp.MustCompile(`(?i)<meta\s+(?:[^>]*?)name=["']description["']\s+(?:[^>]*?)content=["']([^"']+)["']`)
var reMetaDescriptionAlt = regexp.MustCompile(`(?i)<meta\s+(?:[^>]*?)content=["']([^"']+)["']\s+(?:[^>]*?)name=["']description["']`)

// MetaDescription saca og:description / meta description del HTML crudo.
func MetaDescription(rawHTML string) string {
	if rawHTML == "" {
		return ""
	}
	for _, re := range []*regexp.Regexp{
		reOGDescription, reOGDescAlt, reMetaDescription, reMetaDescriptionAlt,
	} {
		if m := re.FindStringSubmatch(rawHTML); len(m) > 1 {
			return m[1]
		}
	}
	return ""
}

func normalizeSpace(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// stripTags quita etiquetas HTML sencillas (la descripción de algunos RSS
// trae <p>, <img>, etc.).
func stripTags(s string) string {
	if !strings.ContainsRune(s, '<') {
		return s
	}
	var b strings.Builder
	depth := 0
	for _, c := range s {
		switch {
		case c == '<':
			depth++
		case c == '>' && depth > 0:
			depth--
			b.WriteRune(' ')
		case depth == 0:
			b.WriteRune(c)
		}
	}
	return b.String()
}
