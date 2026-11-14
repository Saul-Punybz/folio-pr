package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

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

// ── Opportunity Extraction ─────────────────────────────────────────────

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

// parseAmountCents tries to extract a numeric cents value from an amount string.
func parseAmountCents(amount string) int64 {
	if amount == "" {
		return 0
	}
	// Remove currency symbols and common formatting
	clean := strings.NewReplacer(
		"$", "", "€", "", "£", "", ",", "", " ", "",
		"USD", "", "up to", "", "Up to", "", "UP TO", "",
	).Replace(amount)

	// Try to find first number
	var numStr string
	inNumber := false
	for _, ch := range clean {
		if ch >= '0' && ch <= '9' || ch == '.' {
			numStr += string(ch)
			inNumber = true
		} else if inNumber {
			break
		}
	}
	if numStr == "" {
		return 0
	}

	f, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return 0
	}

	// Handle multipliers in the original string
	lower := strings.ToLower(amount)
	if strings.Contains(lower, "million") || strings.Contains(lower, "millón") || strings.Contains(lower, "m") {
		// Only apply M multiplier if the number is small (likely abbreviated)
		if f < 1000 {
			f *= 1_000_000
		}
	} else if strings.Contains(lower, "billion") || strings.Contains(lower, "b") {
		if f < 1000 {
			f *= 1_000_000_000
		}
	}

	return int64(f * 100)
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

// ExtractAndStoreOpportunity runs AI extraction on a page and stores the result.
func ExtractAndStoreOpportunity(ctx context.Context, deps Deps, page *models.CrawledPage) error {
	if deps.AI == nil || deps.Opportunities == nil {
		return nil
	}

	text := deepCleanText(page.CleanText)
	if len(text) > 6000 {
		text = text[:6000]
	}

	// Check if worth analyzing
	if !shouldCheckOpportunity(page.Tags, text) {
		_ = deps.Pages.MarkOpportunityChecked(ctx, page.ID)
		return nil
	}

	extracted, err := deps.AI.ExtractOpportunity(ctx, text)
	if err != nil {
		slog.Warn("crawler/opportunity: extract", "id", page.ID, "err", err)
		_ = deps.Pages.MarkOpportunityChecked(ctx, page.ID)
		return err
	}

	_ = deps.Pages.MarkOpportunityChecked(ctx, page.ID)

	if !extracted.IsOpportunity || extracted.Confidence < 30 {
		return nil
	}

	// Parse deadline
	var deadline *time.Time
	if extracted.Deadline != "" {
		if t, parseErr := time.Parse("2006-01-02", extracted.Deadline); parseErr == nil {
			deadline = &t
		}
	}

	// Validate category
	validCategories := map[string]bool{
		"rfp": true, "grant": true, "funding": true, "proposal": true,
		"contract": true, "fellowship": true, "scholarship": true, "other": true,
	}
	cat := strings.ToLower(extracted.Category)
	if !validCategories[cat] {
		cat = "other"
	}

	rawJSON, _ := json.Marshal(extracted)

	opp := &models.Opportunity{
		PageID:        &page.ID,
		DomainID:      page.DomainID,
		Title:         extracted.Title,
		Agency:        extracted.Agency,
		Summary:       extracted.Summary,
		Amount:        extracted.Amount,
		AmountCents:   parseAmountCents(extracted.Amount),
		Deadline:      deadline,
		ContactName:   extracted.ContactName,
		ContactEmail:  extracted.ContactEmail,
		Category:      cat,
		Status:        computeOpportunityStatus(deadline),
		SourceURL:     page.URL,
		Confidence:    extracted.Confidence,
		RawExtraction: rawJSON,
	}

	if err := deps.Opportunities.Upsert(ctx, opp); err != nil {
		return fmt.Errorf("opportunity store: %w", err)
	}

	slog.Info("crawler/opportunity: extracted",
		"page_id", page.ID,
		"title", opp.Title,
		"category", opp.Category,
		"confidence", opp.Confidence,
	)
	return nil
}

// RunOpportunityExtraction batch-processes enriched pages that haven't been
// checked for opportunities yet.
func RunOpportunityExtraction(ctx context.Context, deps Deps, batchSize int) {
	if batchSize <= 0 {
		batchSize = 50
	}
	if deps.Opportunities == nil {
		return
	}

	slog.Info("crawler/opportunity: starting extraction", "batch_size", batchSize)

	pages, err := deps.Pages.ListUncheckedForOpportunities(ctx, batchSize)
	if err != nil {
		slog.Error("crawler/opportunity: list unchecked", "err", err)
		return
	}

	if len(pages) == 0 {
		slog.Debug("crawler/opportunity: no pages to check")
		return
	}

	extracted := 0
	for i := range pages {
		if ctx.Err() != nil {
			break
		}
		if err := ExtractAndStoreOpportunity(ctx, deps, &pages[i]); err != nil {
			slog.Warn("crawler/opportunity: extract page", "id", pages[i].ID, "err", err)
			continue
		}
		extracted++
	}

	slog.Info("crawler/opportunity: extraction complete", "checked", extracted, "total", len(pages))
}
