package research

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Saul-Punybz/folio/internal/ai"
)

// buildResearchQueries generates 8-15 search query variations from the topic and keywords.
// Includes Spanish and English variations, time-scoped queries, and domain-specific queries.
func buildResearchQueries(topic string, keywords []string) []string {
	seen := make(map[string]bool)
	var queries []string

	add := func(q string) {
		q = strings.TrimSpace(q)
		lower := strings.ToLower(q)
		if !seen[lower] && q != "" {
			seen[lower] = true
			queries = append(queries, q)
		}
	}

	// Core topic queries
	add(topic + " Puerto Rico")
	add(topic + " PR")
	add(topic + " en Puerto Rico")

	// Keyword-based queries
	for _, kw := range keywords {
		if len(queries) >= 15 {
			break
		}
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		add(kw + " Puerto Rico")
		add(topic + " " + kw)
	}

	// Time-scoped
	add(topic + " Puerto Rico 2025 2026")
	add(topic + " Puerto Rico noticias recientes")

	// English variant
	add(topic + " Puerto Rico news")

	// Government/policy angle
	add(topic + " gobierno Puerto Rico")
	add(topic + " legislacion Puerto Rico")

	// Cap at 15
	if len(queries) > 15 {
		queries = queries[:15]
	}

	return queries
}

// expandTopicKeywords uses AI to generate 5-8 related search terms for a research topic.
func expandTopicKeywords(ctx context.Context, aiClient *ai.OllamaClient, topic string) ([]string, error) {
	systemPrompt := `Eres un investigador especializado en Puerto Rico. Dado un tema de investigacion, genera entre 5 y 8 palabras clave o frases cortas relacionadas que ayuden a encontrar informacion relevante.

REGLAS:
- Palabras clave en español (a menos que el tema sea en ingles)
- Frases cortas (1-3 palabras max)
- Incluye: sinonimos, subtemas, organizaciones relacionadas, leyes o programas relevantes
- NO incluyas "Puerto Rico" como palabra clave (ya se agrega automaticamente)
- NO incluyas palabras genericas como "informacion", "noticias", "datos"
- Output SOLO las palabras separadas por comas, sin numeros ni explicaciones`

	userPrompt := fmt.Sprintf("Tema de investigacion: %s", topic)

	resp, err := aiClient.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		slog.Warn("research/queries: AI keyword expansion failed", "topic", topic, "err", err)
		return nil, err
	}

	// Parse comma-separated response
	parts := strings.Split(resp, ",")
	var keywords []string
	seen := make(map[string]bool)

	for _, p := range parts {
		p = strings.TrimSpace(p)
		p = strings.Trim(p, `"'`)
		p = strings.TrimLeft(p, "0123456789.- ")
		p = strings.TrimSpace(p)

		if p == "" || len(p) < 2 || len(p) > 50 {
			continue
		}

		lower := strings.ToLower(p)
		if lower == "puerto rico" || lower == "informacion" || lower == "noticias" || lower == "datos" {
			continue
		}

		if !seen[lower] {
			seen[lower] = true
			keywords = append(keywords, p)
		}
	}

	if len(keywords) > 8 {
		keywords = keywords[:8]
	}

	slog.Info("research/queries: expanded keywords", "topic", topic, "keywords", keywords)
	return keywords, nil
}
