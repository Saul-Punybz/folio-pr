package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
)

// EnrichPage runs the full AI enrichment pipeline on a crawled page:
// summarize, classify tags, extract entities, sentiment, embed, persist entities,
// detect relationships.
func EnrichPage(ctx context.Context, deps Deps, page *models.CrawledPage) error {
	if deps.AI == nil {
		return fmt.Errorf("enrich: AI client not available")
	}

	text := page.CleanText
	if len(text) > 8000 {
		text = text[:8000]
	}
	if len(text) < 50 {
		slog.Debug("crawler/enrich: skipping short page", "id", page.ID, "len", len(text))
		return nil
	}

	// 1. Summarize
	summary, err := deps.AI.Summarize(ctx, text)
	if err != nil {
		slog.Warn("crawler/enrich: summarize", "id", page.ID, "err", err)
		summary = ""
	}

	// 2. Classify tags
	tags, err := deps.AI.Classify(ctx, text)
	if err != nil {
		slog.Warn("crawler/enrich: classify", "id", page.ID, "err", err)
		tags = nil
	}
	tagsJSON, _ := json.Marshal(tags)

	// 3. Extract entities
	extracted, err := deps.AI.ExtractEntities(ctx, text)
	if err != nil {
		slog.Warn("crawler/enrich: extract entities", "id", page.ID, "err", err)
		extracted = &ai.ExtractedEntities{}
	}
	entitiesJSON, _ := json.Marshal(extracted)

	// 4. Sentiment
	sentiment, err := deps.AI.ClassifySentiment(ctx, text)
	if err != nil {
		slog.Warn("crawler/enrich: sentiment", "id", page.ID, "err", err)
		sentiment = "unknown"
	}

	// 5. Embed
	embedding, err := deps.AI.Embed(ctx, text)
	if err != nil {
		slog.Warn("crawler/enrich: embed", "id", page.ID, "err", err)
		embedding = nil
	}

	// 6. Persist enrichment to crawled_pages
	if err := deps.Pages.UpdateEnrichment(ctx, page.ID, summary, sentiment, tagsJSON, entitiesJSON, embedding); err != nil {
		return fmt.Errorf("enrich: update enrichment: %w", err)
	}

	// 7. Persist entities and link to page
	linkEntities(ctx, deps, page.ID, extracted)

	// 8. Extract relationships between entities
	if len(extracted.People)+len(extracted.Organizations)+len(extracted.Places) >= 2 {
		triples, relErr := ExtractRelationships(ctx, deps.AI, extracted, text)
		if relErr != nil {
			slog.Warn("crawler/enrich: extract relationships", "id", page.ID, "err", relErr)
		} else {
			persistRelationships(ctx, deps, triples)
		}
	}

	slog.Info("crawler/enrich: done",
		"id", page.ID,
		"summary_len", len(summary),
		"tags", len(tags),
		"entities", len(extracted.People)+len(extracted.Organizations)+len(extracted.Places),
		"sentiment", sentiment,
	)

	return nil
}

// DetectChangeSummary uses AI to describe what changed between old and new
// content.
func DetectChangeSummary(ctx context.Context, aiClient *ai.OllamaClient, oldText, newText string) (string, error) {
	if aiClient == nil {
		return "", nil
	}
	if len(oldText) > 3000 {
		oldText = oldText[:3000]
	}
	if len(newText) > 3000 {
		newText = newText[:3000]
	}

	prompt := fmt.Sprintf(
		"Compare these two versions of a web page and describe what changed in 1-2 sentences. Focus on substantive content changes, not formatting.\n\nOLD VERSION:\n%s\n\nNEW VERSION:\n%s\n\nWhat changed:",
		oldText, newText,
	)
	summary, err := aiClient.Generate(ctx, "You are a change detection assistant. Be concise and factual.", prompt)
	if err != nil {
		return "", fmt.Errorf("detect change summary: %w", err)
	}
	return strings.TrimSpace(summary), nil
}

// ExtractRelationships uses AI to infer relationships between entities found
// in text.
func ExtractRelationships(ctx context.Context, aiClient *ai.OllamaClient, entities *ai.ExtractedEntities, text string) ([]RelationshipTriple, error) {
	if aiClient == nil {
		return nil, nil
	}

	allNames := make([]string, 0)
	allNames = append(allNames, entities.People...)
	allNames = append(allNames, entities.Organizations...)
	allNames = append(allNames, entities.Places...)

	if len(allNames) < 2 {
		return nil, nil
	}

	if len(text) > 4000 {
		text = text[:4000]
	}

	prompt := fmt.Sprintf(
		`Given these entities found in a text: %s

And this text excerpt:
%s

Identify relationships between pairs of entities. Use ONLY these relation types:
associated, works_for, heads, member_of, located_in, funds, opposes, supports, regulates

Return a JSON array of objects with "source", "target", "relation" fields. Return [] if no clear relationships.
Example: [{"source":"Juan Garcia","target":"Senado de PR","relation":"member_of"}]`,
		strings.Join(allNames, ", "), text,
	)

	response, err := aiClient.Generate(ctx, "You are a relationship extraction assistant. Return only valid JSON.", prompt)
	if err != nil {
		return nil, fmt.Errorf("extract relationships: %w", err)
	}

	response = strings.TrimSpace(response)
	start := strings.Index(response, "[")
	end := strings.LastIndex(response, "]")
	if start == -1 || end == -1 || end <= start {
		return nil, nil
	}
	response = response[start : end+1]

	var triples []RelationshipTriple
	if err := json.Unmarshal([]byte(response), &triples); err != nil {
		slog.Warn("crawler/enrich: parse relationships JSON", "err", err)
		return nil, nil
	}

	validTypes := map[string]bool{
		"associated": true, "works_for": true, "heads": true, "member_of": true,
		"located_in": true, "funds": true, "opposes": true, "supports": true, "regulates": true,
	}
	var valid []RelationshipTriple
	for _, t := range triples {
		if validTypes[t.Relation] && t.Source != "" && t.Target != "" {
			valid = append(valid, t)
		}
	}

	return valid, nil
}

// RelationshipTriple is a source-target-relation triple from AI extraction.
type RelationshipTriple struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	Relation string `json:"relation"`
}

// linkEntities upserts entities and links them to the crawled page.
func linkEntities(ctx context.Context, deps Deps, pageID uuid.UUID, extracted *ai.ExtractedEntities) {
	if deps.Entities == nil || deps.PageEnts == nil {
		return
	}

	link := func(name, entityType string) {
		entityID, err := deps.Entities.Upsert(ctx, name, entityType)
		if err != nil {
			slog.Warn("crawler/enrich: upsert entity", "name", name, "err", err)
			return
		}
		if err := deps.PageEnts.Link(ctx, pageID, entityID); err != nil {
			slog.Warn("crawler/enrich: link entity to page", "name", name, "err", err)
		}
	}

	for _, name := range extracted.People {
		link(name, "person")
	}
	for _, name := range extracted.Organizations {
		link(name, "organization")
	}
	for _, name := range extracted.Places {
		link(name, "place")
	}
}

// persistRelationships upserts entity relationships from extracted triples.
func persistRelationships(ctx context.Context, deps Deps, triples []RelationshipTriple) {
	if deps.Entities == nil || deps.Rels == nil {
		return
	}

	for _, t := range triples {
		// Look up entity IDs by canonical name. Upsert ensures they exist.
		sourceID, err := deps.Entities.Upsert(ctx, t.Source, guessEntityType(t.Source))
		if err != nil {
			continue
		}
		targetID, err := deps.Entities.Upsert(ctx, t.Target, guessEntityType(t.Target))
		if err != nil {
			continue
		}
		if err := deps.Rels.Upsert(ctx, sourceID, targetID, t.Relation); err != nil {
			slog.Warn("crawler/enrich: upsert relationship", "err", err)
		}
	}
}

// guessEntityType returns a default entity type. The actual type was determined
// during extraction but may not be passed through the triple. Default to
// "organization" as government entities are the primary crawl targets.
func guessEntityType(name string) string {
	_ = name
	return "organization"
}
