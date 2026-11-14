package crawler

import (
	"strings"
)

// deepCleanText applies aggressive text cleaning to remove CSS/JS/nav garbage
// that leaked through the initial HTML stripping. Se usa antes de pasar el
// texto de una página a extract.Opportunity. (El parser de montos está en
// internal/extract: es el único del repo.)
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
