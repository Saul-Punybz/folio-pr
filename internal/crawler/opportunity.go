package crawler

import (
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Reglas sin IA para oportunidades (fondos). Se reusan en la obra de Fondos.

// opportunityKeywords are terms that suggest a page might contain grant/RFP info.
var opportunityKeywords = []string{
	"rfp", "request for proposal", "grant", "funding", "solicitud",
	"convocatoria", "licitacion", "licitación", "subasta", "propuesta",
	"fellowship", "scholarship", "beca", "fondos", "subvención",
	"contract", "contrato", "procurement", "adquisición", "subsidio",
	"request for qualifications", "rfq", "rfi", "request for information",
	"notice of funding", "nofo", "funding opportunity", "foa",
}

// shouldCheckOpportunity determines if a page is worth checking for opportunities
// based on its tags and text content.
func shouldCheckOpportunity(tags json.RawMessage, text string) bool {
	// Check tags
	var tagList []string
	if err := json.Unmarshal(tags, &tagList); err == nil {
		for _, t := range tagList {
			t = strings.ToLower(t)
			if t == "grants" || t == "federal" || t == "government" {
				return true
			}
		}
	}

	// Check text for keywords
	lower := strings.ToLower(text)
	for _, kw := range opportunityKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// reAmount encuentra una cifra con su multiplicador opcional:
//   - sufijo pegado al número: 5M, 5MM, 2B, 2BN, 50K
//   - palabra (con o sin espacio): million(s), millón, millones, billion(s),
//     billón, billones, "mil millones"
//
// Antes se multiplicaba por un millón si el texto tenía una "m" en cualquier
// parte ("$500 mensuales" → $500 millones). Ahora el multiplicador tiene que
// ir pegado a la cifra.
var reAmount = regexp.MustCompile(`(?i)(\$|€|£|usd\s*)?(\d+(?:,\d{3})+(?:\.\d+)?|\d+(?:\.\d+)?)(?:(mm|bn|m|b|k)\b|\s*(mil\s+millones|millones|millón|millon|millions?|billones|billón|billon|billions?)\b)?`)

// parseAmountCents extrae la primera cifra (preferiblemente la que lleva
// símbolo de moneda) y la devuelve en centavos. 0 si no hay cifra.
// "billón/billones" se interpreta como 10^9 (uso de EE. UU., común en PR).
func parseAmountCents(amount string) int64 {
	if amount == "" {
		return 0
	}
	matches := reAmount.FindAllStringSubmatch(amount, -1)
	if len(matches) == 0 {
		return 0
	}
	m := matches[0]
	for _, cand := range matches {
		if cand[1] != "" {
			m = cand
			break
		}
	}

	f, err := strconv.ParseFloat(strings.ReplaceAll(m[2], ",", ""), 64)
	if err != nil {
		return 0
	}

	switch suffix := strings.ToLower(m[3]); suffix {
	case "m", "mm":
		f *= 1_000_000
	case "b", "bn":
		f *= 1_000_000_000
	case "k":
		f *= 1_000
	}
	word := strings.ToLower(strings.Join(strings.Fields(m[4]), " "))
	switch {
	case word == "":
	case strings.HasPrefix(word, "mil millones"), strings.HasPrefix(word, "bill"):
		f *= 1_000_000_000
	case strings.HasPrefix(word, "mill"):
		f *= 1_000_000
	}

	return int64(f*100 + 0.5)
}

// computeOpportunityStatus determines the status based on the deadline date.
func computeOpportunityStatus(deadline *time.Time) string {
	if deadline == nil {
		return "unknown"
	}
	now := time.Now()
	if deadline.Before(now) {
		return "closed"
	}
	if deadline.Before(now.AddDate(0, 0, 7)) {
		return "closing_soon"
	}
	return "open"
}

// deepCleanText applies aggressive text cleaning to remove CSS/JS/nav garbage
// that leaked through the initial HTML stripping. This is used before sending
// text to AI for opportunity extraction.
func deepCleanText(text string) string {
	lines := strings.Split(text, "\n")
	var cleaned []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || len(trimmed) < 4 {
			continue
		}
		lower := strings.ToLower(trimmed)

		// Skip CSS patterns
		if strings.Contains(trimmed, "{") || strings.Contains(trimmed, "}") {
			continue
		}
		if strings.HasPrefix(trimmed, ".") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, "@") {
			if !strings.HasPrefix(trimmed, "...") {
				continue
			}
		}
		if strings.Contains(lower, "!important") || strings.Contains(lower, "rgba(") {
			continue
		}

		// Skip JS patterns
		if strings.HasPrefix(lower, "var ") || strings.HasPrefix(lower, "let ") || strings.HasPrefix(lower, "const ") ||
			strings.HasPrefix(lower, "function ") || strings.HasPrefix(lower, "ga(") || strings.HasPrefix(lower, "gtag(") ||
			strings.HasPrefix(lower, "window.") || strings.HasPrefix(lower, "document.") {
			continue
		}

		// Skip navigation-like single words
		if len(trimmed) < 20 && !strings.Contains(trimmed, " ") {
			// Allow numbers, dates, amounts
			if !strings.ContainsAny(trimmed, "0123456789$€") {
				continue
			}
		}

		// Skip common boilerplate
		boilerplate := []string{
			"skip to", "ir al contenido", "cookie", "search for", "buscar",
			"formulario de", "toggle", "copyright", "all rights", "powered by",
			"share this", "compartir", "follow us", "back to top",
			"accede aquí", "accede aqui", "ver calendario",
		}
		skip := false
		for _, bp := range boilerplate {
			if strings.HasPrefix(lower, bp) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		cleaned = append(cleaned, trimmed)
	}
	return strings.Join(cleaned, "\n")
}
