package agents

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Saul-Punybz/folio/internal/models"
)

// classifyAndDraft runs sentiment analysis on unclassified hits,
// then generates PR response drafts for negative/important hits.
func classifyAndDraft(ctx context.Context, deps Deps) {
	unknownHits, err := deps.Hits.ListBySentiment(ctx, "unknown", 20)
	if err != nil {
		slog.Error("watchlist/drafter: list unknown hits", "err", err)
		return
	}

	if len(unknownHits) == 0 {
		return
	}

	slog.Info("watchlist/drafter: classifying hits", "count", len(unknownHits))

	classified := 0
	drafted := 0

	for _, hit := range unknownHits {
		if ctx.Err() != nil {
			break
		}

		sentiment := classifySentiment(ctx, deps, hit)
		if err := deps.Hits.UpdateSentiment(ctx, hit.ID, sentiment); err != nil {
			slog.Error("watchlist/drafter: update sentiment", "id", hit.ID, "err", err)
			continue
		}
		classified++

		// Generate PR draft for negative hits.
		if sentiment == "negative" {
			draft := generatePRDraft(ctx, deps, hit)
			if draft != "" {
				if err := deps.Hits.UpdateAIDraft(ctx, hit.ID, draft); err != nil {
					slog.Error("watchlist/drafter: update draft", "id", hit.ID, "err", err)
				} else {
					drafted++
				}
			}
		}
	}

	slog.Info("watchlist/drafter: complete", "classified", classified, "drafted", drafted)
}

func classifySentiment(ctx context.Context, deps Deps, hit models.WatchlistHit) string {
	systemPrompt := `You are a PR sentiment classifier. Classify the following news mention as one of: positive, neutral, negative.

RULES:
- Output ONLY one word: positive, neutral, or negative
- positive = praise, awards, good coverage, success stories
- neutral = factual mention, event listing, directory entry
- negative = criticism, scandal, complaint, legal issue, failure
- When in doubt, output "neutral"
- Do NOT explain your reasoning`

	userPrompt := fmt.Sprintf("Title: %s\nSnippet: %s", hit.Title, hit.Snippet)

	// Use the default model (3b) for fast classification.
	resp, err := deps.AI.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		slog.Warn("watchlist/drafter: classify sentiment", "err", err)
		return "neutral"
	}

	resp = strings.TrimSpace(strings.ToLower(resp))
	switch {
	case strings.Contains(resp, "positive"):
		return "positive"
	case strings.Contains(resp, "negative"):
		return "negative"
	case strings.Contains(resp, "neutral"):
		return "neutral"
	default:
		return "neutral"
	}
}

func generatePRDraft(ctx context.Context, deps Deps, hit models.WatchlistHit) string {
	systemPrompt := `Eres un especialista en relaciones publicas para organizaciones sin fines de lucro en Puerto Rico. Tu trabajo es redactar respuestas de PR a menciones negativas en los medios.

REGLAS:
- Escribe en español profesional
- Se conciso (2-3 parrafos maximo)
- Tono empatico pero firme
- Reconoce la preocupacion del publico sin admitir culpa
- Incluye una accion concreta que la organizacion tomara
- No uses jerga legal
- Empieza directamente con el borrador, sin titulos ni encabezados`

	userPrompt := fmt.Sprintf("Mencion negativa:\nTitulo: %s\nDetalle: %s\n\nRedacta un comunicado de respuesta de PR.", hit.Title, hit.Snippet)

	// Use 8b model for quality PR drafts.
	draft, err := deps.AI.GenerateWithModel(ctx, "llama3.1:8b", systemPrompt, userPrompt)
	if err != nil {
		slog.Error("watchlist/drafter: generate PR draft", "hit_id", hit.ID, "err", err)
		return ""
	}
	return strings.TrimSpace(draft)
}
