package agents

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// EnrichOrgKeywords fetches the org's website (if provided) and uses AI to extract
// relevant keywords for monitoring. If no website is given, falls back to web search.
// Returns the suggested keywords (does NOT save them — caller decides).
func EnrichOrgKeywords(ctx context.Context, orgName, websiteURL string, aiClient *ai.OllamaClient) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	var pageContent string
	var results []scraper.WebResult

	// Step 1: If website URL provided, fetch it directly.
	if websiteURL != "" {
		content, fetchErr := fetchPageText(ctx, websiteURL)
		if fetchErr != nil {
			slog.Warn("enrich: failed to fetch provided website", "url", websiteURL, "err", fetchErr)
		} else if len(content) > 200 {
			pageContent = content
			slog.Info("enrich: fetched org website", "org", orgName, "url", websiteURL, "len", len(content))
		}
	}

	// Step 2: Also do a web search for extra context.
	query := orgName + " Puerto Rico"
	var err error
	results, err = scraper.WebSearch(ctx, query, 5)
	if err != nil {
		slog.Warn("enrich: web search failed", "org", orgName, "err", err)
	}

	// Step 3: If no page content yet (no website provided or fetch failed), try search results.
	if pageContent == "" {
		for _, result := range results {
			if ctx.Err() != nil {
				break
			}
			content, fetchErr := fetchPageText(ctx, result.URL)
			if fetchErr != nil {
				continue
			}
			if len(content) > 200 {
				pageContent = content
				slog.Info("enrich: fetched org website from search", "org", orgName, "url", result.URL, "len", len(content))
				break
			}
		}
	}

	// Step 4: Build context from web search snippets + page content.
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Organizacion: %s\n\n", orgName))

	if pageContent != "" {
		// Cap to first 3000 chars of page content.
		if len(pageContent) > 3000 {
			pageContent = pageContent[:3000]
		}
		sb.WriteString("Contenido de su pagina web:\n")
		sb.WriteString(pageContent)
		sb.WriteString("\n\n")
	}

	if len(results) > 0 {
		sb.WriteString("Resultados de busqueda:\n")
		for i, r := range results {
			sb.WriteString(fmt.Sprintf("%d. %s — %s\n", i+1, r.Title, r.Snippet))
		}
	}

	if sb.Len() < 50 {
		return nil, fmt.Errorf("enrich: insufficient data found for %q", orgName)
	}

	// Step 4: Ask AI to extract keywords.
	systemPrompt := `Eres un analista de monitoreo de medios para organizaciones sin fines de lucro en Puerto Rico.

TAREA: Dado el nombre de una organizacion y datos de su pagina web/busqueda, extrae entre 6 y 10 palabras clave o frases cortas que serian utiles para monitorear menciones de esta organizacion en noticias, redes sociales y publicaciones.

REGLAS:
- La primera palabra clave DEBE ser el nombre exacto de la organizacion
- Incluye: nombre abreviado, siglas si existen, programa principal, director/lider si aparece, temas clave
- Las palabras clave deben ser en español (a menos que el nombre sea en ingles)
- Frases cortas (1-3 palabras max por keyword)
- NO incluyas palabras genericas como "Puerto Rico", "organizacion", "sin fines de lucro"
- Output SOLO las palabras clave separadas por comas, sin numeros ni explicaciones
- Si encuentras que la organizacion tiene programas especificos, incluye el nombre del programa
- Si la organizacion tiene liderazgo conocido, incluye el nombre del director/presidente`

	resp, err := aiClient.GenerateWithModel(ctx, "llama3.2:3b", systemPrompt, sb.String())
	if err != nil {
		return nil, fmt.Errorf("enrich: AI generation failed: %w", err)
	}

	// Parse the comma-separated response.
	keywords := parseKeywords(resp, orgName)
	if len(keywords) == 0 {
		return []string{orgName}, nil
	}

	slog.Info("enrich: keywords extracted", "org", orgName, "keywords", keywords)
	return keywords, nil
}

// fetchPageText fetches a URL and returns stripped text content.
func fetchPageText(ctx context.Context, rawURL string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 256*1024))
	if err != nil {
		return "", err
	}

	// Strip HTML to get text.
	return stripHTMLTags(string(body)), nil
}

// stripHTMLTags removes HTML tags and collapses whitespace.
func stripHTMLTags(s string) string {
	var out strings.Builder
	inTag := false
	lastSpace := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
			if !lastSpace {
				out.WriteRune(' ')
				lastSpace = true
			}
		case !inTag:
			if r == '\n' || r == '\r' || r == '\t' {
				if !lastSpace {
					out.WriteRune(' ')
					lastSpace = true
				}
			} else {
				out.WriteRune(r)
				lastSpace = r == ' '
			}
		}
	}

	result := strings.TrimSpace(out.String())
	// Decode common HTML entities.
	result = strings.ReplaceAll(result, "&amp;", "&")
	result = strings.ReplaceAll(result, "&lt;", "<")
	result = strings.ReplaceAll(result, "&gt;", ">")
	result = strings.ReplaceAll(result, "&quot;", `"`)
	result = strings.ReplaceAll(result, "&#x27;", "'")
	result = strings.ReplaceAll(result, "&nbsp;", " ")
	return result
}

// parseKeywords parses AI output into a clean keyword list.
// Ensures the org name is always the first keyword.
func parseKeywords(raw string, orgName string) []string {
	raw = strings.TrimSpace(raw)
	// Remove numbered list formatting if present.
	raw = strings.ReplaceAll(raw, "\n", ",")

	parts := strings.Split(raw, ",")
	seen := make(map[string]bool)
	var keywords []string

	// Always ensure org name is first.
	normalizedName := strings.ToLower(strings.TrimSpace(orgName))
	seen[normalizedName] = true
	keywords = append(keywords, strings.TrimSpace(orgName))

	for _, p := range parts {
		p = strings.TrimSpace(p)
		// Strip list formatting: "1. ", "- ", etc.
		for len(p) > 0 && (p[0] >= '0' && p[0] <= '9' || p[0] == '.' || p[0] == '-' || p[0] == ' ') {
			p = p[1:]
		}
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		p = strings.TrimSpace(p)

		if p == "" || len(p) < 2 || len(p) > 50 {
			continue
		}

		lower := strings.ToLower(p)
		// Skip generic terms.
		if isGenericKeyword(lower) {
			continue
		}

		if !seen[lower] {
			seen[lower] = true
			keywords = append(keywords, p)
		}
	}

	// Cap at 10 keywords.
	if len(keywords) > 10 {
		keywords = keywords[:10]
	}
	return keywords
}

// isGenericKeyword returns true for words too generic to be useful as search terms.
func isGenericKeyword(lower string) bool {
	generics := []string{
		"puerto rico", "organizacion", "organización", "sin fines de lucro",
		"non-profit", "nonprofit", "ong", "ngo",
		"comunidad", "community", "servicio", "programa",
		"website", "pagina web", "contacto", "email",
	}
	for _, g := range generics {
		if lower == g {
			return true
		}
	}
	return false
}
