package research

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
)

// RunPhase3 scores relevance, aggregates entities, builds timeline, and generates dossier.
func RunPhase3(ctx context.Context, deps Deps, project *models.ResearchProject) error {
	slog.Info("research/phase3: starting synthesis", "topic", project.Topic)

	findings, err := deps.Findings.ListAllByProject(ctx, project.ID)
	if err != nil {
		return fmt.Errorf("research/phase3: list findings: %w", err)
	}

	if len(findings) == 0 {
		return fmt.Errorf("research/phase3: no findings to synthesize")
	}

	// Step 1: Score relevance for each finding
	scoreRelevance(ctx, deps, project.Topic, findings)

	// Step 2: Aggregate entities across all findings
	entities := aggregateEntities(findings)

	// Step 3: Build timeline from dated findings
	timeline := buildTimeline(findings)

	// Step 4: Generate dossier using AI
	dossier := generateDossier(ctx, deps.AI, project.Topic, findings, entities, timeline)

	// Step 5: Save results
	entitiesJSON, _ := json.Marshal(entities)
	timelineJSON, _ := json.Marshal(timeline)

	if err := deps.Projects.SetFinished(ctx, project.ID, dossier, entitiesJSON, timelineJSON); err != nil {
		return fmt.Errorf("research/phase3: set finished: %w", err)
	}

	slog.Info("research/phase3: complete", "topic", project.Topic,
		"findings", len(findings),
		"entities_people", len(entities.People),
		"entities_orgs", len(entities.Organizations),
		"timeline_events", len(timeline),
	)
	return nil
}

// scoreRelevance uses AI to score each finding's relevance to the topic (0.0-1.0).
func scoreRelevance(ctx context.Context, deps Deps, topic string, findings []models.ResearchFinding) {
	systemPrompt := fmt.Sprintf(`Score the relevance of this text to the research topic "%s" in Puerto Rico.
Output ONLY a number between 0.0 and 1.0.
- 1.0 = directly about the topic in Puerto Rico
- 0.5 = somewhat related
- 0.0 = completely unrelated
Output ONLY the number, nothing else.`, topic)

	for i := range findings {
		if ctx.Err() != nil {
			break
		}

		text := findings[i].Title + " " + findings[i].Snippet
		if findings[i].CleanText != "" {
			text = truncateStr(findings[i].CleanText, 1000)
		}

		resp, err := deps.AI.Generate(ctx, systemPrompt, text)
		if err != nil {
			continue
		}

		score := parseRelevanceScore(resp)
		findings[i].Relevance = score
		_ = deps.Findings.UpdateRelevance(ctx, findings[i].ID, score)
	}
}

// parseRelevanceScore extracts a float from AI response.
func parseRelevanceScore(resp string) float32 {
	resp = strings.TrimSpace(resp)
	var score float32
	_, err := fmt.Sscanf(resp, "%f", &score)
	if err != nil || score < 0 || score > 1 {
		return 0.5
	}
	return score
}

// DossierEntities holds aggregated entities from all findings.
type DossierEntities struct {
	People        []string `json:"people"`
	Organizations []string `json:"organizations"`
	Places        []string `json:"places"`
}

// aggregateEntities collects unique entities from all findings.
func aggregateEntities(findings []models.ResearchFinding) DossierEntities {
	peopleSeen := make(map[string]bool)
	orgsSeen := make(map[string]bool)
	placesSeen := make(map[string]bool)

	var result DossierEntities

	for _, f := range findings {
		if len(f.Entities) == 0 {
			continue
		}
		var ents ai.ExtractedEntities
		if err := json.Unmarshal(f.Entities, &ents); err != nil {
			continue
		}
		for _, p := range ents.People {
			lower := strings.ToLower(p)
			if !peopleSeen[lower] {
				peopleSeen[lower] = true
				result.People = append(result.People, p)
			}
		}
		for _, o := range ents.Organizations {
			lower := strings.ToLower(o)
			if !orgsSeen[lower] {
				orgsSeen[lower] = true
				result.Organizations = append(result.Organizations, o)
			}
		}
		for _, pl := range ents.Places {
			lower := strings.ToLower(pl)
			if !placesSeen[lower] {
				placesSeen[lower] = true
				result.Places = append(result.Places, pl)
			}
		}
	}

	return result
}

// TimelineEvent represents a dated event in the research timeline.
type TimelineEvent struct {
	Date      string `json:"date"`
	Event     string `json:"event"`
	SourceURL string `json:"source_url"`
}

// buildTimeline creates a chronological timeline from dated findings.
func buildTimeline(findings []models.ResearchFinding) []TimelineEvent {
	var events []TimelineEvent

	for _, f := range findings {
		if f.PublishedAt == nil || f.Title == "" {
			continue
		}
		if f.Relevance < 0.3 {
			continue
		}

		events = append(events, TimelineEvent{
			Date:      f.PublishedAt.Format("2006-01-02"),
			Event:     f.Title,
			SourceURL: f.URL,
		})
	}

	// Sort by date (simple string sort works for YYYY-MM-DD)
	for i := 0; i < len(events); i++ {
		for j := i + 1; j < len(events); j++ {
			if events[j].Date < events[i].Date {
				events[i], events[j] = events[j], events[i]
			}
		}
	}

	// Cap at 50 events
	if len(events) > 50 {
		events = events[:50]
	}

	return events
}

// generateDossier uses AI to create a structured research report.
func generateDossier(ctx context.Context, aiClient *ai.OllamaClient, topic string, findings []models.ResearchFinding, entities DossierEntities, timeline []TimelineEvent) string {
	if aiClient == nil {
		return buildFallbackDossier(topic, findings, entities)
	}

	// Build context from top findings
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Tema de investigacion: %s en Puerto Rico\n\n", topic))
	sb.WriteString(fmt.Sprintf("Total de fuentes encontradas: %d\n\n", len(findings)))

	// Add top findings
	count := 0
	for _, f := range findings {
		if count >= 20 {
			break
		}
		if f.Relevance < 0.3 {
			continue
		}
		sb.WriteString(fmt.Sprintf("FUENTE %d:\nTitulo: %s\nURL: %s\n", count+1, f.Title, f.URL))
		text := f.Snippet
		if f.CleanText != "" {
			text = truncateStr(f.CleanText, 500)
		}
		sb.WriteString(fmt.Sprintf("Contenido: %s\n\n", text))
		count++
	}

	// Add entities
	if len(entities.People) > 0 {
		sb.WriteString(fmt.Sprintf("Personas mencionadas: %s\n", strings.Join(entities.People, ", ")))
	}
	if len(entities.Organizations) > 0 {
		sb.WriteString(fmt.Sprintf("Organizaciones: %s\n", strings.Join(entities.Organizations, ", ")))
	}

	// Truncate context to avoid exceeding model limits
	context := sb.String()
	if len(context) > 8000 {
		context = context[:8000]
	}

	systemPrompt := `Eres un analista de inteligencia politica especializado en Puerto Rico. Genera un dossier de investigacion completo en formato Markdown.

ESTRUCTURA OBLIGATORIA:
## Resumen Ejecutivo
(3-5 parrafos resumiendo los hallazgos principales)

## Hallazgos Clave
(5-10 puntos principales, cada uno con evidencia de las fuentes)

## Actores Principales
(Personas y organizaciones relevantes con su rol)

## Analisis de Sentimiento
(Balance general: positivo, negativo, neutro. Tendencias observadas)

## Conclusiones y Recomendaciones
(Que se puede concluir y que areas necesitan mas investigacion)

REGLAS:
- Escribe en español profesional
- Se objetivo y basado en evidencia
- Cita las fuentes cuando sea posible
- NO inventes informacion — usa solo lo proporcionado
- Si hay poca informacion, indica que la investigacion fue limitada`

	dossier, err := aiClient.Generate(ctx, systemPrompt, context)
	if err != nil {
		slog.Error("research/phase3: generate dossier", "err", err)
		return buildFallbackDossier(topic, findings, entities)
	}

	return dossier
}

// buildFallbackDossier creates a basic dossier without AI.
func buildFallbackDossier(topic string, findings []models.ResearchFinding, entities DossierEntities) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# Investigacion: %s en Puerto Rico\n\n", topic))
	sb.WriteString(fmt.Sprintf("## Resumen\n\nSe encontraron %d fuentes relacionadas con el tema.\n\n", len(findings)))

	sb.WriteString("## Fuentes Principales\n\n")
	count := 0
	for _, f := range findings {
		if count >= 15 {
			break
		}
		if f.Title != "" {
			sb.WriteString(fmt.Sprintf("- **%s** — [%s](%s)\n", f.Title, f.SourceType, f.URL))
			if f.Snippet != "" {
				sb.WriteString(fmt.Sprintf("  %s\n", truncateStr(f.Snippet, 200)))
			}
			count++
		}
	}

	if len(entities.People) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Personas Mencionadas\n%s\n", strings.Join(entities.People, ", ")))
	}
	if len(entities.Organizations) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Organizaciones\n%s\n", strings.Join(entities.Organizations, ", ")))
	}
	if len(entities.Places) > 0 {
		sb.WriteString(fmt.Sprintf("\n## Lugares\n%s\n", strings.Join(entities.Places, ", ")))
	}

	return sb.String()
}
