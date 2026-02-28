package scraper

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
)

// GenerateDailyBrief creates a summary of the day's articles using Ollama.
// It queries articles from the last 24 hours, concatenates titles and summaries,
// calls the AI to generate a daily digest, counts top tags, and creates a brief record.
func GenerateDailyBrief(ctx context.Context, articles *models.ArticleStore, briefs *models.BriefStore, aiClient *ai.OllamaClient) {
	slog.Info("daily brief: starting generation")

	// Get top 60 most recent articles from the last 24 hours.
	allRecent, err := articles.ListRecent(ctx, 24)
	if err != nil {
		slog.Error("daily brief: list recent articles", "err", err)
		return
	}

	if len(allRecent) == 0 {
		slog.Info("daily brief: no articles in the last 24 hours, skipping")
		return
	}

	// Cap at 60 articles — enough for a quality brief without overwhelming the AI.
	recentArticles := allRecent
	if len(recentArticles) > 60 {
		recentArticles = recentArticles[:60]
	}

	slog.Info("daily brief: processing articles", "count", len(recentArticles))

	// Build a text block of titles + summaries/snippets for the AI.
	var sb strings.Builder
	for i, a := range recentArticles {
		if sb.Len() > 12000 {
			break
		}
		sb.WriteString(fmt.Sprintf("%d. [%s] %s", i+1, a.Source, a.Title))
		if a.Summary != "" {
			sb.WriteString(": ")
			sb.WriteString(a.Summary)
		} else if a.CleanText != "" {
			snippet := a.CleanText
			if len(snippet) > 400 {
				snippet = snippet[:400] + "..."
			}
			sb.WriteString(": ")
			sb.WriteString(snippet)
		}
		sb.WriteString("\n")
	}

	inputText := sb.String()
	if len(inputText) > 15000 {
		inputText = inputText[:15000]
	}

	// Generate the daily brief summary via AI.
	systemPrompt := `Eres un analista de inteligencia política de Puerto Rico. Genera un resumen diario conciso de las noticias más importantes.

REGLAS:
- Escribe en español
- Agrupa las noticias por tema (política, economía, crimen, salud, etc.)
- Menciona nombres específicos de personas, agencias y lugares
- Incluye 3-5 párrafos, cada uno sobre un tema diferente
- Usa un tono profesional y analítico
- NO repitas la misma noticia más de una vez
- Empieza directamente con el contenido, sin títulos como "Resumen Diario"`

	// Use the 8b model for briefs — quality matters more than speed for background tasks.
	summary, err := aiClient.GenerateWithModel(ctx, "llama3.1:8b", systemPrompt, inputText)
	if err != nil {
		slog.Error("daily brief: AI generation failed", "err", err)
		// Fall back to a simple concatenation.
		summary = fmt.Sprintf("Daily brief: %d articles collected. ", len(recentArticles))
		if len(recentArticles) > 0 {
			summary += "Top stories: "
			max := 5
			if len(recentArticles) < max {
				max = len(recentArticles)
			}
			titles := make([]string, max)
			for i := 0; i < max; i++ {
				titles[i] = recentArticles[i].Title
			}
			summary += strings.Join(titles, "; ")
		}
	}

	// Count top tags from the day's articles.
	tagCounts := make(map[string]int)
	for _, a := range recentArticles {
		for _, t := range a.Tags {
			tagCounts[t]++
		}
	}

	// Sort tags by frequency and take top 10.
	type tagCount struct {
		Tag   string
		Count int
	}
	var sortedTags []tagCount
	for tag, count := range tagCounts {
		sortedTags = append(sortedTags, tagCount{tag, count})
	}
	sort.Slice(sortedTags, func(i, j int) bool {
		return sortedTags[i].Count > sortedTags[j].Count
	})

	topTags := make([]string, 0, 10)
	for i, tc := range sortedTags {
		if i >= 10 {
			break
		}
		topTags = append(topTags, tc.Tag)
	}

	// Create the brief record.
	brief := &models.Brief{
		Date:         time.Now().UTC().Truncate(24 * time.Hour),
		Summary:      summary,
		TopTags:      topTags,
		ArticleCount: len(recentArticles),
	}

	if err := briefs.Create(ctx, brief); err != nil {
		slog.Error("daily brief: create record", "err", err)
		return
	}

	slog.Info("daily brief: generated successfully",
		"id", brief.ID,
		"article_count", brief.ArticleCount,
		"top_tags", topTags,
	)
}
