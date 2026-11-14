// Package generator implements the Escritos SEO article generation pipeline.
// It runs a 5-phase process: research, planning, generation, scoring, verification.
package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// Deps groups dependencies needed by the generator.
type Deps struct {
	Escritos *models.EscritoStore
	Sources  *models.EscritoSourceStore
	Articles *models.ArticleStore
	AI       *ai.OllamaClient
}

// ArticleSection represents a planned section for the article.
type ArticleSection struct {
	Heading    string   `json:"heading"`
	Angle      string   `json:"angle"`
	WordTarget int      `json:"word_target"`
	SourceIDs  []string `json:"source_ids,omitempty"`
}

type editorialProfile struct {
	Format        string
	Style         string
	Audience      string
	CitationStyle string
}

// webSource holds a single result from Phase 0 web research.
type webSource struct {
	URL       string
	Title     string
	Snippet   string
	CleanText string // scraped full text (truncated to ~2000 chars)
	Domain    string // e.g. "wikipedia.org", "gobierno.pr"
}

// ── Phase 0: Topic Interpretation + Web Research ─────────────────

// interpretTopic asks the AI to interpret a raw user topic into a clear subject
// and optimized search queries.
func interpretTopic(ctx context.Context, aiClient *ai.OllamaClient, rawTopic string) (subject string, queries []string) {
	if aiClient == nil {
		return rawTopic, []string{rawTopic, rawTopic + " Puerto Rico"}
	}

	systemPrompt := `Eres un investigador experto. Dado un tema de usuario (puede ser informal, un codigo postal, una frase corta), genera:
1. Un titulo claro de articulo en español
2. 3-5 consultas de busqueda web optimizadas (mezcla español e ingles)

REGLAS:
- Responde SOLO con JSON valido: {"subject":"...","queries":["...","..."]}
- Interpreta la intencion del usuario (ej: "00924 San Juan" = barrios y comunidades de ese codigo postal)
- Las queries deben ser especificas y variadas para encontrar datos reales
- Incluye al menos una query en ingles para fuentes federales/internacionales`

	userPrompt := fmt.Sprintf("Tema del usuario: %s", rawTopic)

	result, err := aiClient.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		slog.Warn("generator: interpretTopic AI failed, using raw topic", "err", err)
		return rawTopic, []string{rawTopic, rawTopic + " Puerto Rico"}
	}

	result = extractJSON(result)

	var parsed struct {
		Subject string   `json:"subject"`
		Queries []string `json:"queries"`
	}
	if err := json.Unmarshal([]byte(result), &parsed); err != nil {
		slog.Warn("generator: interpretTopic parse failed", "err", err)
		return rawTopic, []string{rawTopic, rawTopic + " Puerto Rico"}
	}

	if parsed.Subject == "" {
		parsed.Subject = rawTopic
	}
	if len(parsed.Queries) == 0 {
		parsed.Queries = []string{rawTopic, rawTopic + " Puerto Rico"}
	}

	slog.Info("generator: topic interpreted", "raw", rawTopic, "subject", parsed.Subject, "queries", len(parsed.Queries))
	return parsed.Subject, parsed.Queries
}

// runWebResearch searches the web and scrapes top results for real content.
func runWebResearch(ctx context.Context, queries []string) []webSource {
	if len(queries) > 3 {
		queries = queries[:3]
	}

	// Collect raw search results from all queries
	type searchHit struct {
		title   string
		url     string
		snippet string
	}
	var allHits []searchHit
	seen := make(map[string]bool)

	for _, q := range queries {
		// MultiWebSearch runs DDG + Bing in parallel
		results, err := scraper.MultiWebSearch(ctx, q, 10)
		if err != nil {
			slog.Warn("generator: web search failed", "query", q, "err", err)
			continue
		}
		for _, r := range results {
			key := normalizeWebURL(r.URL)
			if seen[key] {
				continue
			}
			seen[key] = true
			allHits = append(allHits, searchHit{title: r.Title, url: r.URL, snippet: r.Snippet})
		}

		// Also try Google News RSS
		feedURL := "https://news.google.com/rss/search?q=" + url.QueryEscape(q) + "&hl=es-419&gl=PR&ceid=PR:es-419"
		items, err := scraper.ParseFeed(ctx, feedURL)
		if err == nil {
			for _, item := range items {
				key := normalizeWebURL(item.Link)
				if seen[key] {
					continue
				}
				seen[key] = true
				allHits = append(allHits, searchHit{title: item.Title, url: item.Link, snippet: item.Description})
			}
		}

		if len(allHits) >= 15 {
			break
		}
	}

	// Cap at 15 unique results
	if len(allHits) > 15 {
		allHits = allHits[:15]
	}

	slog.Info("generator: web search complete", "unique_hits", len(allHits))

	if len(allHits) == 0 {
		return nil
	}

	// Scrape top 10 results in parallel
	toScrape := allHits
	if len(toScrape) > 10 {
		toScrape = toScrape[:10]
	}

	type scrapeResult struct {
		idx       int
		cleanText string
	}

	results := make(chan scrapeResult, len(toScrape))
	var wg sync.WaitGroup

	for i, hit := range toScrape {
		wg.Add(1)
		go func(idx int, pageURL string) {
			defer wg.Done()
			text := scrapePage(ctx, pageURL)
			results <- scrapeResult{idx: idx, cleanText: text}
		}(i, hit.url)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	scraped := make(map[int]string)
	for r := range results {
		scraped[r.idx] = r.cleanText
	}

	// Build webSource list, filtering out empty/short scrapes
	var sources []webSource
	for i, hit := range toScrape {
		text := scraped[i]
		if len(text) < 200 {
			// Still include if we have snippet, just no full text
			if hit.snippet == "" {
				continue
			}
			text = hit.snippet
		}
		// Truncate to 2000 chars
		if len([]rune(text)) > 2000 {
			text = string([]rune(text)[:2000])
		}
		sources = append(sources, webSource{
			URL:       hit.url,
			Title:     hit.title,
			Snippet:   hit.snippet,
			CleanText: text,
			Domain:    extractDomain(hit.url),
		})
	}

	// Also include non-scraped hits (11-15) with snippet only
	for i := len(toScrape); i < len(allHits); i++ {
		hit := allHits[i]
		if hit.snippet == "" {
			continue
		}
		sources = append(sources, webSource{
			URL:       hit.url,
			Title:     hit.title,
			Snippet:   hit.snippet,
			CleanText: hit.snippet,
			Domain:    extractDomain(hit.url),
		})
	}

	slog.Info("generator: web research complete", "sources_with_content", len(sources))
	return sources
}

// scrapePage fetches a URL and extracts clean article text.
func scrapePage(ctx context.Context, pageURL string) string {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pageURL, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		return ""
	}

	cleanText, _ := scraper.ExtractArticle(string(body))
	return cleanText
}

// runPhase0 orchestrates topic interpretation and web research.
func runPhase0(ctx context.Context, deps Deps, escrito *models.Escrito) (subject string, queries []string, sources []webSource) {
	subject, queries = interpretTopic(ctx, deps.AI, escrito.Topic)

	sources = runWebResearch(ctx, queries)
	if len(sources) == 0 {
		slog.Info("generator: phase 0 found no web sources, continuing with local-only", "topic", escrito.Topic)
	} else {
		slog.Info("generator: phase 0 complete", "subject", subject, "web_sources", len(sources))
	}
	return
}

// normalizeWebURL extracts host+path for deduplication.
func normalizeWebURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.ToLower(rawURL)
	}
	return strings.ToLower(u.Host + u.Path)
}

// extractDomain returns the domain from a URL (e.g. "wikipedia.org").
func extractDomain(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	host := strings.ToLower(u.Host)
	// Strip www. prefix
	host = strings.TrimPrefix(host, "www.")
	return host
}

type verificationCheck struct {
	Name    string `json:"name"`
	Passed  bool   `json:"passed"`
	Details string `json:"details"`
}

type verificationReport struct {
	Passed   bool                `json:"passed"`
	Issues   []string            `json:"issues,omitempty"`
	Warnings []string            `json:"warnings,omitempty"`
	Checks   []verificationCheck `json:"checks"`
}

// RunGeneration orchestrates all 5 phases for a single escrito.
func RunGeneration(ctx context.Context, deps Deps, escritoID uuid.UUID, articleIDs []uuid.UUID) {
	slog.Info("generator: starting escrito", "id", escritoID)

	escrito, err := deps.Escritos.GetByID(ctx, escritoID)
	if err != nil {
		slog.Error("generator: get escrito", "id", escritoID, "err", err)
		return
	}

	if err := deps.Escritos.SetStarted(ctx, escrito.ID); err != nil {
		slog.Error("generator: set started", "id", escritoID, "err", err)
		return
	}

	profile := resolveEditorialProfile(escrito)

	// ── Phase 0: Topic Interpretation + Web Research ──
	slog.Info("generator: phase 0 — web research", "topic", escrito.Topic)
	subject, _, webSources := runPhase0(ctx, deps, escrito)
	updateProgress(ctx, deps, escrito.ID, map[string]any{
		"phase":         0,
		"web_sources":   len(webSources),
		"subject":       subject,
		"output_format": profile.Format,
		"writing_style": profile.Style,
		"audience":      profile.Audience,
		"citation":      profile.CitationStyle,
	})

	// ── Phase 1: Source Gathering ──
	slog.Info("generator: phase 1 — source gathering", "topic", escrito.Topic)
	sources, err := runPhase1(ctx, deps, escrito, articleIDs, webSources)
	if err != nil {
		slog.Error("generator: phase 1 failed", "err", err)
		_ = deps.Escritos.SetFailed(ctx, escrito.ID, "Phase 1 failed: "+err.Error())
		return
	}
	if ctx.Err() != nil {
		return
	}
	// sources may be nil (AI-only mode) — that's OK, phases 2-3 handle it

	updateProgress(ctx, deps, escrito.ID, map[string]any{"phase": 1, "sources": len(sources)})

	// ── Phase 2: Article Planning ──
	if err := deps.Escritos.UpdateStatus(ctx, escrito.ID, "planning", 2); err != nil {
		slog.Error("generator: update status to planning", "err", err)
	}

	slog.Info("generator: phase 2 — article planning", "subject", subject, "sources", len(sources))
	sections, err := runPhase2(ctx, deps, escrito, sources, subject, webSources, profile)
	if err != nil {
		slog.Error("generator: phase 2 failed", "err", err)
		_ = deps.Escritos.SetFailed(ctx, escrito.ID, "Phase 2 failed: "+err.Error())
		return
	}
	if ctx.Err() != nil {
		return
	}

	// Save the plan
	planJSON, _ := json.Marshal(sections)
	_ = deps.Escritos.SetArticlePlan(ctx, escrito.ID, planJSON)
	updateProgress(ctx, deps, escrito.ID, map[string]any{"phase": 2, "sections": len(sections)})

	// ── Phase 3: Section-by-Section Generation ──
	if err := deps.Escritos.UpdateStatus(ctx, escrito.ID, "generating", 3); err != nil {
		slog.Error("generator: update status to generating", "err", err)
	}

	slog.Info("generator: phase 3 — generating content", "subject", subject, "sections", len(sections))
	err = runPhase3(ctx, deps, escrito, sources, sections, subject, webSources, profile)
	if err != nil {
		slog.Error("generator: phase 3 failed", "err", err)
		_ = deps.Escritos.SetFailed(ctx, escrito.ID, "Phase 3 failed: "+err.Error())
		return
	}
	if ctx.Err() != nil {
		return
	}

	// ── Phase 4: SEO Scoring ──
	if err := deps.Escritos.UpdateStatus(ctx, escrito.ID, "scoring", 4); err != nil {
		slog.Error("generator: update status to scoring", "err", err)
	}

	slog.Info("generator: phase 4 — SEO scoring", "topic", escrito.Topic)
	err = runPhase4(ctx, deps, escrito, profile)
	if err != nil {
		slog.Error("generator: phase 4 failed", "err", err)
		_ = deps.Escritos.SetFailed(ctx, escrito.ID, "Phase 4 failed: "+err.Error())
		return
	}

	// ── Phase 5: Verification ──
	if err := deps.Escritos.UpdateStatus(ctx, escrito.ID, "verifying", 5); err != nil {
		slog.Error("generator: update status to verifying", "err", err)
	}

	slog.Info("generator: phase 5 — verification", "topic", escrito.Topic)
	err = runPhase5(ctx, deps, escrito, profile)
	if err != nil {
		slog.Error("generator: phase 5 failed", "err", err)
		_ = deps.Escritos.SetFailed(ctx, escrito.ID, "Phase 5 failed: "+err.Error())
		return
	}

	_ = deps.Escritos.SetFinished(ctx, escrito.ID)

	slog.Info("generator: escrito complete",
		"topic", escrito.Topic,
		"sources", len(sources),
		"sections", len(sections),
		"duration", time.Since(escrito.CreatedAt).Round(time.Second),
	)
}

// ── Phase 1: Source Gathering ──────────────────────────────────────

type sourceArticle struct {
	ID      uuid.UUID
	Title   string
	Summary string
	URL     string
	Source  string
}

func runPhase1(ctx context.Context, deps Deps, escrito *models.Escrito, articleIDs []uuid.UUID, webSources []webSource) ([]sourceArticle, error) {
	var articles []sourceArticle
	var relevances []float64

	if len(articleIDs) > 0 {
		// Use provided article IDs — user explicitly picked these
		for _, aid := range articleIDs {
			a, err := deps.Articles.GetByID(ctx, aid)
			if err != nil {
				continue
			}
			articles = append(articles, sourceArticle{
				ID: a.ID, Title: a.Title, Summary: a.Summary,
				URL: a.URL, Source: a.Source,
			})
			relevances = append(relevances, 0.90) // user-picked = high relevance
		}
	} else {
		// ── Strategy 1: Semantic vector search + keyword validation ──
		slog.Info("generator: trying vector search", "topic", escrito.Topic)
		topicKeywords := extractTopicKeywords(escrito.Topic)
		embedding, err := deps.AI.Embed(ctx, escrito.Topic)
		if err == nil && len(embedding) > 0 {
			results, scores, err := deps.Articles.SearchByVector(ctx, embedding, 30, 0.70)
			if err == nil && len(results) > 0 {
				for i, a := range results {
					// Validate: article must contain at least one topic keyword
					if !articleMatchesKeywords(a.Title, a.Summary, topicKeywords) {
						continue
					}
					articles = append(articles, sourceArticle{
						ID: a.ID, Title: a.Title, Summary: a.Summary,
						URL: a.URL, Source: a.Source,
					})
					relevances = append(relevances, scores[i])
				}
				slog.Info("generator: vector search found sources", "count", len(articles), "topic", escrito.Topic)
			}
		} else {
			slog.Warn("generator: embedding failed, skipping vector search", "err", err)
		}

		// ── Strategy 2: ILIKE keyword AND (fallback if vector found nothing) ──
		if len(articles) == 0 {
			slog.Info("generator: trying keyword AND search", "topic", escrito.Topic)
			results, err := deps.Articles.SearchByKeywords(ctx, escrito.Topic, 15)
			if err == nil {
				for _, a := range results {
					articles = append(articles, sourceArticle{
						ID: a.ID, Title: a.Title, Summary: a.Summary,
						URL: a.URL, Source: a.Source,
					})
					relevances = append(relevances, 0.70) // keyword match = moderate
				}
			}
		}
	}

	// Append web research sources (these have verified URLs)
	for _, ws := range webSources {
		articles = append(articles, sourceArticle{
			ID:      uuid.Nil,
			Title:   ws.Title,
			Summary: ws.Snippet,
			URL:     ws.URL,
			Source:  ws.Domain,
		})
		relevances = append(relevances, 0.85) // web-researched = high relevance
	}

	// Cap at 20 sources (increased from 15 to accommodate web sources)
	if len(articles) > 20 {
		articles = articles[:20]
		relevances = relevances[:20]
	}

	if len(articles) == 0 {
		slog.Info("generator: no relevant sources found, proceeding with AI-only generation", "topic", escrito.Topic)
		return nil, nil
	}

	// Insert escrito_sources with real relevance scores
	for i, a := range articles {
		rel := float32(relevances[i])
		if rel < 0.1 {
			rel = 0.1
		}
		src := &models.EscritoSource{
			EscritoID:     escrito.ID,
			Relevance:     rel,
			UsedInSection: "",
		}
		if a.ID != uuid.Nil {
			id := a.ID
			src.ArticleID = &id
		} else {
			// Web source — store metadata directly
			src.WebURL = a.URL
			src.WebTitle = a.Title
			src.WebDomain = a.Source
		}
		_ = deps.Sources.Create(ctx, src)
	}

	return articles, nil
}

// ── Phase 2: Article Planning ──────────────────────────────────────

func runPhase2(ctx context.Context, deps Deps, escrito *models.Escrito, sources []sourceArticle, subject string, webSources []webSource, profile editorialProfile) ([]ArticleSection, error) {
	// Use interpreted subject if available, fall back to raw topic
	articleSubject := subject
	if articleSubject == "" {
		articleSubject = escrito.Topic
	}

	// Build context from local + web sources (cap at 8000 chars)
	var contextBuf strings.Builder

	// Web sources first (they're topic-specific and richer)
	for _, ws := range webSources {
		entry := fmt.Sprintf("- [WEB] %s (%s): %s\n", ws.Title, ws.Domain, truncate(ws.Snippet, 200))
		if contextBuf.Len()+len(entry) > 8000 {
			break
		}
		contextBuf.WriteString(entry)
	}
	// Then local DB sources
	for _, s := range sources {
		if s.ID == uuid.Nil {
			continue // already included as web source above
		}
		entry := fmt.Sprintf("- [LOCAL] %s: %s\n", s.Title, truncate(s.Summary, 200))
		if contextBuf.Len()+len(entry) > 8000 {
			break
		}
		contextBuf.WriteString(entry)
	}

	minWords, maxWords := targetWordRange(profile.Format)

	systemPrompt := fmt.Sprintf(`Eres un editor SEO experto en contenido en español para Puerto Rico.
Tu tarea es crear un plan de %s basado en las fuentes proporcionadas.

REGLAS ESTRICTAS:
- Responde SOLO con un JSON array valido, sin texto adicional
- Cada elemento tiene: heading, angle, word_target
- El formato final es "%s" para audiencia "%s" con estilo "%s"
- Ajusta la estructura a ese formato, pero SIEMPRE incluye una seccion final de referencias/bibliografia
- Si el formato es article/report/op_ed: incluye introduccion, desarrollo, conclusion y referencias
- Si el formato es newsletter: incluye apertura fuerte, bloques escaneables, cierre y referencias
- Si el formato es memo/email/letter: incluye asunto o encabezado implicito, cuerpo claro, cierre y referencias
- Total target: %d-%d palabras segun formato
- Todos los headings en español
- La seccion final SIEMPRE debe ser "Referencias y Recursos" o "Bibliografia"
- Basa las secciones en DATOS REALES de las fuentes web proporcionadas
- Crea suficiente profundidad: incluye contexto, datos, implicaciones, FAQ o recomendaciones cuando aplique
- Evita planes superficiales de 4-5 bloques genericos si el formato pide mas desarrollo`, profile.Format, profile.Format, profile.Audience, profile.Style, minWords, maxWords)

	userPrompt := fmt.Sprintf(`Tema: %s
Perfil editorial:
- Formato: %s
- Estilo: %s
- Audiencia: %s
- Citacion: %s

Fuentes disponibles:
%s

Genera el plan de articulo como JSON array. Ejemplo de formato:
[{"heading":"Introduccion","angle":"contexto del tema en PR","word_target":180},{"heading":"Estado Actual de [Tema]","angle":"situacion presente con datos","word_target":300}]`, articleSubject, profile.Format, profile.Style, profile.Audience, profile.CitationStyle, contextBuf.String())

	result, err := deps.AI.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("AI planning: %w", err)
	}

	// Extract JSON from response
	result = extractJSON(result)

	var sections []ArticleSection
	if err := json.Unmarshal([]byte(result), &sections); err != nil {
		// Try to build a default plan
		slog.Warn("generator: AI plan parse failed, using default", "err", err)
		sections = defaultPlan(articleSubject, profile)
	}

	if len(sections) < 3 {
		sections = defaultPlan(articleSubject, profile)
	}

	// Ensure a References section exists at the end
	hasRefs := false
	for _, s := range sections {
		lower := strings.ToLower(s.Heading)
		if strings.Contains(lower, "referencia") || strings.Contains(lower, "recurso") || strings.Contains(lower, "bibliograf") {
			hasRefs = true
			break
		}
	}
	if !hasRefs {
		sections = append(sections, ArticleSection{
			Heading:    "Referencias y Recursos",
			Angle:      "bibliografia con enlaces a fuentes externas reales: Wikipedia, gobierno, universidades, organizaciones",
			WordTarget: 200,
		})
	}

	sections = normalizeSectionTargets(sections, profile.Format)

	return sections, nil
}

// ── Phase 3: Section Generation ────────────────────────────────────

func runPhase3(ctx context.Context, deps Deps, escrito *models.Escrito, sources []sourceArticle, sections []ArticleSection, subject string, webSources []webSource, profile editorialProfile) error {
	articleSubject := subject
	if articleSubject == "" {
		articleSubject = escrito.Topic
	}

	// Build source context — web sources get priority (richer, topic-specific)
	var srcContext strings.Builder

	// Web sources with real scraped content
	for _, ws := range webSources {
		entry := fmt.Sprintf("Fuente Web: %s (%s)\nURL: %s\nContenido: %s\n\n", ws.Title, ws.Domain, ws.URL, truncate(ws.CleanText, 500))
		if srcContext.Len()+len(entry) > 8000 {
			break
		}
		srcContext.WriteString(entry)
	}
	// Local DB sources fill remaining budget
	for _, s := range sources {
		if s.ID == uuid.Nil {
			continue // already included as web source
		}
		entry := fmt.Sprintf("Fuente Local: %s (%s)\nResumen: %s\n\n", s.Title, s.Source, truncate(s.Summary, 300))
		if srcContext.Len()+len(entry) > 8000 {
			break
		}
		srcContext.WriteString(entry)
	}

	var fullContent strings.Builder

	for i, section := range sections {
		if ctx.Err() != nil {
			return ctx.Err()
		}

		updateProgress(ctx, deps, escrito.ID, map[string]any{
			"phase":           3,
			"current_section": i + 1,
			"total_sections":  len(sections),
			"section_name":    section.Heading,
		})

		sectionContent, err := generateSection(ctx, deps.AI, articleSubject, section, srcContext.String(), webSources, profile)
		if err != nil {
			slog.Error("generator: section generation failed", "section", section.Heading, "err", err)
			// Use placeholder instead of failing entire article
			sectionContent = fmt.Sprintf("## %s\n\n[Seccion pendiente de generacion]\n", section.Heading)
		}

		if i > 0 {
			fullContent.WriteString("\n\n")
		}
		fullContent.WriteString(sectionContent)
	}

	// Scrub AI phrases from assembled text
	content := ScrubAIPhrases(fullContent.String())

	// Generate SEO metadata
	title, slug, metaDesc, keywords, hashtags := generateMetadata(ctx, deps.AI, articleSubject, content, profile)

	wordCount := countWordsInText(content)
	planJSON, _ := json.Marshal(sections)

	err := deps.Escritos.UpdateContent(ctx, escrito.ID, title, slug, metaDesc, content, keywords, hashtags, wordCount, planJSON)
	if err != nil {
		return fmt.Errorf("save content: %w", err)
	}

	return nil
}

func generateSection(ctx context.Context, aiClient *ai.OllamaClient, topic string, section ArticleSection, srcContext string, webSources []webSource, profile editorialProfile) (string, error) {
	isReferences := strings.Contains(strings.ToLower(section.Heading), "referencia") ||
		strings.Contains(strings.ToLower(section.Heading), "recurso") ||
		strings.Contains(strings.ToLower(section.Heading), "bibliograf")

	var systemPrompt string
	if isReferences && len(webSources) > 0 {
		// Build verified URL list from actual web research
		var verifiedURLs strings.Builder
		for _, ws := range webSources {
			verifiedURLs.WriteString(fmt.Sprintf("- %s | %s | %s\n", ws.Title, ws.Domain, ws.URL))
		}

		systemPrompt = fmt.Sprintf(`Eres un editor que formatea bibliografias para textos en español.

Formatea las siguientes fuentes VERIFICADAS como una seccion de referencias.

REGLAS ESTRICTAS:
- Usa UNICAMENTE las URLs proporcionadas abajo — NO inventes ni agregues ninguna URL
- Usa estilo de citacion %s de forma simplificada y consistente
- Formato markdown: - Autor/Institucion. [Titulo de la Fuente](URL)
- Agrupa por tipo si es posible (Gobierno, Medios, Organizaciones, Academicas)
- Comienza con "## %s"
- Responde SOLO con el contenido de la seccion

FUENTES VERIFICADAS:
%s`, strings.ToUpper(profile.CitationStyle), section.Heading, verifiedURLs.String())
	} else if isReferences {
		// Fallback: no web sources available, use old behavior but be honest
		systemPrompt = fmt.Sprintf(`Eres un investigador que compila bibliografias para textos en español sobre Puerto Rico.
Genera la seccion "%s" para un articulo sobre "%s".

REGLAS:
- Usa estilo de citacion %s
- Lista 5-10 fuentes de dominios conocidos y verificables
- Formato markdown: [Nombre](URL)
- Prioriza: Wikipedia, .gov, .pr.gov, universidades, organizaciones internacionales
- Comienza con "## %s"
 - Responde SOLO con el contenido de la seccion`, section.Heading, topic, strings.ToUpper(profile.CitationStyle), section.Heading)
	} else {
		// Build inline source hints from web research for this section
		var sourceHints string
		if len(webSources) > 0 {
			var hints strings.Builder
			hints.WriteString("\n\nFuentes web verificadas que puedes citar con hyperlinks:\n")
			for _, ws := range webSources {
				hints.WriteString(fmt.Sprintf("- [%s](%s) — %s\n", ws.Title, ws.URL, truncate(ws.Snippet, 100)))
			}
			sourceHints = hints.String()
		}

		systemPrompt = fmt.Sprintf(`Eres un escritor SEO experto en español para Puerto Rico.
Escribe la seccion "%s" de un %s sobre "%s".

REGLAS ESTRICTAS:
- Escribe EXACTAMENTE en español
- NO uses frases como "es importante destacar", "cabe señalar", "sin duda alguna"
- NO uses lenguaje artificial o generico
- Escribe con voz activa, datos concretos, ejemplos locales de PR
- Adapta el tono al estilo "%s" y a la audiencia "%s"
- Basa tu contenido en los DATOS REALES de las fuentes proporcionadas
- Cuando cites un dato, estadistica o hecho, incluye la fuente entre parentesis
  Ejemplo: "La generacion solar aumento un 45%% en 2024 (Autoridad de Energia Electrica de PR)"
- Incluye 1-2 hyperlinks por seccion usando las URLs verificadas de las fuentes proporcionadas
  Ejemplo: segun [IRENA](https://www.irena.org/), la capacidad instalada...
- NO inventes URLs — usa SOLO las URLs de las fuentes proporcionadas
- Target: aproximadamente %d palabras
- La seccion debe sentirse desarrollada, no resumida: agrega contexto, explicaciones, implicaciones y ejemplos concretos
- Prioriza densidad informativa y profundidad tematica por encima de frases de relleno
- Si el target es alto, usa varios parrafos o bullets utiles para cubrir el angulo con suficiente detalle
- Si es la primera seccion (Introduccion), NO incluyas heading H2
- Para las demas secciones, comienza con "## %s"
- NO incluyas saludo, despedida ni meta-comentarios
- Si el formato es newsletter, usa bloques mas escaneables y frases mas cortas
- Si el formato es memo, carta o email, usa una cadencia mas ejecutiva y directa
- Responde SOLO con el contenido de la seccion%s`, section.Heading, profile.Format, topic, profile.Style, profile.Audience, section.WordTarget, section.Heading, sourceHints)
	}

	userPrompt := fmt.Sprintf(`Angulo: %s

Informacion de fuentes:
%s

Escribe la seccion ahora:`, section.Angle, truncate(srcContext, 4000))

	result, err := aiClient.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(result), nil
}

func generateMetadata(ctx context.Context, aiClient *ai.OllamaClient, topic, content string, profile editorialProfile) (title, slug, metaDesc string, keywords, hashtags []string) {
	// Defaults
	title = topic
	slug = slugify(topic)
	metaDesc = truncate(content, 155)
	keywords = []string{strings.ToLower(topic)}
	hashtags = []string{"#" + strings.ReplaceAll(strings.ToLower(topic), " ", "")}

	if aiClient == nil {
		return
	}

	systemPrompt := fmt.Sprintf(`Eres un especialista SEO. Genera metadatos para un %s en español.

REGLAS:
- Responde SOLO con JSON valido, sin texto adicional
- Formato exacto: {"title":"...","slug":"...","meta_description":"...","keywords":["..."],"hashtags":["..."]}
- Title: 50-60 caracteres, incluir keyword principal
- Slug: solo minusculas, guiones, sin acentos
- Meta description: 150-160 caracteres, incluir keyword
- Keywords: 5-8 keywords relevantes
- Hashtags: 5-8 hashtags con # prefix, en español
- Ajusta el title y meta a audiencia "%s" y estilo "%s"`, profile.Format, profile.Audience, profile.Style)

	userPrompt := fmt.Sprintf("Tema: %s\n\nPrimeros 500 caracteres del articulo:\n%s",
		topic, truncate(content, 500))

	result, err := aiClient.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return
	}

	result = extractJSON(result)

	var meta struct {
		Title string   `json:"title"`
		Slug  string   `json:"slug"`
		Meta  string   `json:"meta_description"`
		KW    []string `json:"keywords"`
		HT    []string `json:"hashtags"`
	}
	if err := json.Unmarshal([]byte(result), &meta); err != nil {
		return
	}

	if meta.Title != "" {
		title = meta.Title
	}
	if meta.Slug != "" {
		slug = meta.Slug
	}
	if meta.Meta != "" {
		metaDesc = meta.Meta
	}
	if len(meta.KW) > 0 {
		keywords = meta.KW
	}
	if len(meta.HT) > 0 {
		hashtags = meta.HT
	}

	return
}

// RegenerateMetadata rebuilds title, slug, meta description, keywords, and hashtags
// for existing content while respecting the escrito's editorial profile.
func RegenerateMetadata(ctx context.Context, aiClient *ai.OllamaClient, escrito *models.Escrito, content string) (title, slug, metaDesc string, keywords, hashtags []string) {
	profile := resolveEditorialProfile(escrito)
	return generateMetadata(ctx, aiClient, escrito.Topic, content, profile)
}

// ── Improve Content ──────────────────────────────────────────────

// ImproveContent takes existing escrito content and user instructions,
// then asks the AI to rewrite/improve the article accordingly.
func ImproveContent(ctx context.Context, aiClient *ai.OllamaClient, escrito *models.Escrito, instructions string) (string, error) {
	profile := resolveEditorialProfile(escrito)
	systemPrompt := fmt.Sprintf(`Eres un editor experto en contenido SEO en español para Puerto Rico.
Tu tarea es MEJORAR y autocorregir un %s existente segun las instrucciones del usuario.

REGLAS:
- Mantén la estructura H2 existente (puedes agregar secciones si las instrucciones lo piden)
- Conserva todos los hyperlinks y referencias existentes
- Si el usuario pide agregar fuentes, usa URLs reales de sitios verificables
- Mantén el idioma en español
- Mantén el estilo "%s", la audiencia "%s" y el formato "%s" salvo que el usuario pida cambiarlos
- Autocorrige claridad, repeticiones, fluidez, SEO y consistencia de citas
- Si detectas afirmaciones debiles, reformulalas con cautela en vez de inventar datos
- NO uses frases genericas de IA ("es importante destacar", "cabe señalar")
- Devuelve SOLO el articulo completo mejorado en markdown, sin comentarios adicionales`, profile.Format, profile.Style, profile.Audience, profile.Format)

	// Keep more context for long-form rewrites and expansions.
	content := escrito.Content
	if len(content) > 24000 {
		content = content[:24000]
	}

	userPrompt := fmt.Sprintf(`INSTRUCCIONES DE MEJORA:
%s

ARTICULO ACTUAL:
%s

Reescribe el articulo completo aplicando las mejoras solicitadas:`, instructions, content)

	result, err := aiClient.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("AI improve: %w", err)
	}

	result = strings.TrimSpace(result)
	result = ScrubAIPhrases(result)

	if len(result) < 200 {
		return "", fmt.Errorf("improved content too short (%d chars)", len(result))
	}

	return result, nil
}

// ExpandContent rewrites an existing article with explicit goals to increase
// depth, structure, and overall word count while preserving sourcing.
func ExpandContent(ctx context.Context, aiClient *ai.OllamaClient, escrito *models.Escrito, instructions string) (string, error) {
	profile := resolveEditorialProfile(escrito)
	minWords, _ := targetWordRange(profile.Format)
	systemPrompt := fmt.Sprintf(`Eres un editor senior de long-form en español para Puerto Rico.
Tu tarea es EXPANDIR un %s existente para volverlo mas profundo, util y publicable.

REGLAS:
- Mantén el idioma en español
- Mantén y fortalece el estilo "%s" y la audiencia "%s"
- Conserva hyperlinks, citas y referencias ya presentes
- No inventes datos ni URLs
- Puedes agregar nuevas subsecciones H2 y H3 si hacen falta
- Expande con contexto, antecedentes, implicaciones, ejemplos y preguntas frecuentes
- Aumenta sustancialmente la profundidad del texto; objetivo minimo aproximado: %d palabras
- Si faltan transiciones o una conclusion fuerte, agrégalas
- Si detectas secciones debiles, reescribelas con mas detalle
- Devuelve SOLO el articulo completo expandido en markdown`, profile.Format, profile.Style, profile.Audience, minWords)

	content := escrito.Content
	if len(content) > 24000 {
		content = content[:24000]
	}

	userPrompt := fmt.Sprintf(`OBJETIVO DE EXPANSION:
%s

ARTICULO ACTUAL:
%s

Expande y reescribe el articulo completo ahora:`, instructions, content)

	result, err := aiClient.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("AI expand: %w", err)
	}

	result = strings.TrimSpace(ScrubAIPhrases(result))
	if len(result) < len(content)+300 {
		return "", fmt.Errorf("expanded content too short (%d chars)", len(result))
	}
	return result, nil
}

// CountWords is the exported version of countWordsInText.
func CountWords(text string) int {
	return countWordsInText(text)
}

// ── Phase 4: SEO Scoring ──────────────────────────────────────────

func runPhase4(ctx context.Context, deps Deps, escrito *models.Escrito, profile editorialProfile) error {
	// Re-fetch to get latest content
	e, err := deps.Escritos.GetByID(ctx, escrito.ID)
	if err != nil {
		return fmt.Errorf("get escrito for scoring: %w", err)
	}

	primaryKW := ""
	if len(e.Keywords) > 0 {
		primaryKW = e.Keywords[0]
	}

	score := ScoreArticle(e.Content, e.Title, e.MetaDescription, primaryKW)
	if profile.Format == "newsletter" || profile.Format == "email" || profile.Format == "letter" {
		score.Warnings = append(score.Warnings, "SEO score interpretado para formato no tradicional; prioriza legibilidad y conversion.")
	}
	scoreJSON, err := json.Marshal(score)
	if err != nil {
		return fmt.Errorf("marshal seo score: %w", err)
	}

	return deps.Escritos.UpdateSEOScore(ctx, escrito.ID, scoreJSON)
}

// ── Phase 5: Verification ─────────────────────────────────────────

func runPhase5(ctx context.Context, deps Deps, escrito *models.Escrito, profile editorialProfile) error {
	e, err := deps.Escritos.GetByID(ctx, escrito.ID)
	if err != nil {
		return fmt.Errorf("get escrito for verification: %w", err)
	}

	sources, err := deps.Sources.ListByEscrito(ctx, escrito.ID)
	if err != nil {
		return fmt.Errorf("list sources for verification: %w", err)
	}

	report := verifyGeneratedContent(e, sources, profile)

	progressJSON, err := mergeProgress(e.Progress, map[string]any{
		"phase":        5,
		"verification": report,
	})
	if err != nil {
		return fmt.Errorf("marshal verification progress: %w", err)
	}

	if err := deps.Escritos.UpdateProgress(ctx, escrito.ID, progressJSON); err != nil {
		return fmt.Errorf("save verification progress: %w", err)
	}

	if len(report.Issues) == 0 && len(report.Warnings) == 0 {
		return nil
	}

	seoWarnings, err := mergeSEOWarnings(e.SEOScore, append(append([]string{}, report.Issues...), report.Warnings...))
	if err != nil {
		return fmt.Errorf("merge verification warnings: %w", err)
	}

	return deps.Escritos.UpdateSEOScore(ctx, escrito.ID, seoWarnings)
}

func verifyGeneratedContent(e *models.Escrito, sources []models.EscritoSource, profile editorialProfile) verificationReport {
	report := verificationReport{}

	hasBibliographySection := hasReferencesSection(e.Content)
	hasSourceList := len(uniqueSourceEntries(sources)) > 0
	bibliographyOK := hasBibliographySection || hasSourceList
	report.Checks = append(report.Checks, verificationCheck{
		Name:    "bibliography",
		Passed:  bibliographyOK,
		Details: fmt.Sprintf("section=%t, sources=%d", hasBibliographySection, len(uniqueSourceEntries(sources))),
	})
	if !bibliographyOK {
		report.Issues = append(report.Issues, "No se detecto una bibliografia ni una lista de fuentes verificadas.")
	}

	markdownReady := strings.TrimSpace(e.Title) != "" &&
		strings.TrimSpace(e.Content) != "" &&
		!strings.Contains(strings.ToLower(e.Content), "<html") &&
		!strings.Contains(strings.ToLower(e.Content), "<body")
	report.Checks = append(report.Checks, verificationCheck{
		Name:    "markdown_export",
		Passed:  markdownReady,
		Details: "contenido exportable en markdown sin HTML envolvente",
	})
	if !markdownReady {
		report.Issues = append(report.Issues, "El contenido no parece limpio para export markdown.")
	}

	structureOK, structureDetails := verifyStructureByFormat(e.Content, profile.Format)
	report.Checks = append(report.Checks, verificationCheck{
		Name:    "structure",
		Passed:  structureOK,
		Details: structureDetails,
	})
	if !structureOK {
		report.Issues = append(report.Issues, "La estructura minima no cumple con el formato editorial solicitado.")
	}

	unsupportedClaims := countUnsupportedClaims(e.Content)
	claimsOK := unsupportedClaims <= 3
	report.Checks = append(report.Checks, verificationCheck{
		Name:    "claims_with_sources",
		Passed:  claimsOK,
		Details: fmt.Sprintf("%d afirmaciones potencialmente sin apoyo explicito", unsupportedClaims),
	})
	if unsupportedClaims > 6 {
		report.Issues = append(report.Issues, fmt.Sprintf("Se detectaron %d afirmaciones que parecen necesitar fuente explicita.", unsupportedClaims))
	} else if unsupportedClaims > 3 {
		report.Warnings = append(report.Warnings, fmt.Sprintf("Hay %d afirmaciones con soporte dudoso; conviene revisar citas y enlaces.", unsupportedClaims))
	}

	report.Passed = len(report.Issues) == 0
	return report
}

func verifyStructureByFormat(content, format string) (bool, string) {
	wordCount := countWordsInText(content)
	h2Count := strings.Count(content, "\n## ")
	if strings.HasPrefix(strings.TrimSpace(content), "## ") {
		h2Count++
	}
	hasConclusion := containsAnyFold(content, "conclusion", "conclusión", "cierre")

	switch format {
	case "newsletter":
		ok := wordCount >= 500 && h2Count >= 2
		return ok, fmt.Sprintf("newsletter: %d palabras, %d H2", wordCount, h2Count)
	case "memo", "email", "letter":
		ok := wordCount >= 250 && (h2Count >= 1 || hasConclusion)
		return ok, fmt.Sprintf("%s: %d palabras, %d H2, cierre=%t", format, wordCount, h2Count, hasConclusion)
	case "report", "article", "op_ed", "auto":
		ok := wordCount >= 700 && h2Count >= 3 && hasConclusion
		return ok, fmt.Sprintf("%s: %d palabras, %d H2, cierre=%t", format, wordCount, h2Count, hasConclusion)
	default:
		ok := wordCount >= 500 && h2Count >= 2
		return ok, fmt.Sprintf("%s: %d palabras, %d H2", format, wordCount, h2Count)
	}
}

func countUnsupportedClaims(content string) int {
	normalized := strings.NewReplacer("\n", " ", "!", ".", "?", ".").Replace(content)
	sentences := strings.Split(normalized, ".")
	count := 0
	for _, sentence := range sentences {
		s := strings.TrimSpace(sentence)
		if len([]rune(s)) < 35 {
			continue
		}
		lower := strings.ToLower(s)
		looksLikeClaim := containsDigit(lower) ||
			containsAnyFold(lower, "%", "por ciento", "aumento", "aumentó", "disminuyo", "disminuyó", "segun", "según", "revelo", "reveló", "informo", "informó", "reporto", "reportó")
		hasInlineSupport := strings.Contains(s, "(") || strings.Contains(s, "[") || strings.Contains(lower, "segun ") || strings.Contains(lower, "según ")
		if looksLikeClaim && !hasInlineSupport {
			count++
		}
	}
	return count
}

func containsDigit(s string) bool {
	for _, r := range s {
		if r >= '0' && r <= '9' {
			return true
		}
	}
	return false
}

func hasReferencesSection(content string) bool {
	return containsAnyFold(content, "## referencias", "## bibliografia", "## bibliografía", "## fuentes", "## referencias y recursos")
}

func containsAnyFold(s string, needles ...string) bool {
	lower := strings.ToLower(s)
	for _, needle := range needles {
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func mergeProgress(existing json.RawMessage, updates map[string]any) (json.RawMessage, error) {
	var merged map[string]any
	if len(existing) > 0 {
		_ = json.Unmarshal(existing, &merged)
	}
	if merged == nil {
		merged = make(map[string]any, len(updates))
	}
	for k, v := range updates {
		merged[k] = v
	}
	return json.Marshal(merged)
}

func mergeSEOWarnings(existing json.RawMessage, warnings []string) (json.RawMessage, error) {
	var score SEOScore
	if len(existing) > 0 {
		if err := json.Unmarshal(existing, &score); err != nil {
			return nil, err
		}
	}

	seen := make(map[string]struct{}, len(score.Warnings)+len(warnings))
	merged := make([]string, 0, len(score.Warnings)+len(warnings))
	for _, warning := range append(score.Warnings, warnings...) {
		warning = strings.TrimSpace(warning)
		if warning == "" {
			continue
		}
		if _, ok := seen[warning]; ok {
			continue
		}
		seen[warning] = struct{}{}
		merged = append(merged, warning)
	}
	score.Warnings = merged
	return json.Marshal(score)
}

func uniqueSourceEntries(sources []models.EscritoSource) []string {
	seen := make(map[string]struct{}, len(sources))
	out := make([]string, 0, len(sources))
	for _, s := range sources {
		key := strings.TrimSpace(sourceURL(s))
		if key == "" {
			key = strings.TrimSpace(sourceTitle(s))
		}
		if key == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func sourceTitle(s models.EscritoSource) string {
	if strings.TrimSpace(s.WebTitle) != "" {
		return s.WebTitle
	}
	return s.ArticleTitle
}

func sourceURL(s models.EscritoSource) string {
	if strings.TrimSpace(s.WebURL) != "" {
		return s.WebURL
	}
	return s.ArticleURL
}

// ── Helpers ──────────────────────────────────────────────────────

func updateProgress(ctx context.Context, deps Deps, id uuid.UUID, data map[string]any) {
	j, _ := json.Marshal(data)
	_ = deps.Escritos.UpdateProgress(ctx, id, j)
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

func slugify(s string) string {
	s = strings.ToLower(s)
	replacements := map[rune]rune{
		'á': 'a', 'é': 'e', 'í': 'i', 'ó': 'o', 'ú': 'u',
		'ñ': 'n', 'ü': 'u',
	}
	var result strings.Builder
	for _, r := range s {
		if rep, ok := replacements[r]; ok {
			result.WriteRune(rep)
		} else if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			result.WriteRune(r)
		} else if r == ' ' || r == '_' {
			result.WriteRune('-')
		}
	}
	// Clean up multiple dashes
	slug := result.String()
	for strings.Contains(slug, "--") {
		slug = strings.ReplaceAll(slug, "--", "-")
	}
	slug = strings.Trim(slug, "-")
	return slug
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	// Try to find JSON array
	start := strings.Index(s, "[")
	if start >= 0 {
		end := strings.LastIndex(s, "]")
		if end > start {
			return s[start : end+1]
		}
	}
	// Try to find JSON object
	start = strings.Index(s, "{")
	if start >= 0 {
		end := strings.LastIndex(s, "}")
		if end > start {
			return s[start : end+1]
		}
	}
	return s
}

// extractTopicKeywords returns meaningful keywords from a topic string,
// filtering out stopwords and geographic noise like "puerto rico".
func extractTopicKeywords(topic string) []string {
	stopwords := map[string]bool{
		"el": true, "la": true, "los": true, "las": true, "un": true, "una": true,
		"de": true, "del": true, "en": true, "y": true, "o": true, "que": true,
		"es": true, "ha": true, "se": true, "con": true, "por": true,
		"para": true, "al": true, "como": true, "su": true, "a": true,
		"the": true, "is": true, "are": true, "in": true, "of": true, "and": true,
	}
	geoterms := map[string]bool{
		"puerto": true, "rico": true, "isla": true, "boricua": true,
	}

	words := strings.Fields(strings.ToLower(topic))
	var keywords []string
	for _, w := range words {
		w = strings.Trim(w, "¿?¡!.,;:\"'()[]")
		if len(w) >= 3 && !stopwords[w] && !geoterms[w] {
			keywords = append(keywords, w)
		}
	}
	return keywords
}

// articleMatchesKeywords checks if an article's title or summary contains
// at least one of the topic keywords (accent-insensitive).
func articleMatchesKeywords(title, summary string, keywords []string) bool {
	if len(keywords) == 0 {
		return true // no keywords = accept all
	}
	combined := strings.ToLower(title + " " + summary)
	// Normalize accents for comparison
	replacer := strings.NewReplacer(
		"á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ñ", "n", "ü", "u",
	)
	combined = replacer.Replace(combined)
	for _, kw := range keywords {
		kw = replacer.Replace(kw)
		if strings.Contains(combined, kw) {
			return true
		}
	}
	return false
}

func targetWordRange(format string) (int, int) {
	switch format {
	case "newsletter":
		return 1200, 2000
	case "memo", "email", "letter":
		return 700, 1400
	case "report":
		return 2200, 3200
	case "article", "op_ed", "auto":
		return 1800, 3000
	default:
		return 1600, 2600
	}
}

func normalizeSectionTargets(sections []ArticleSection, format string) []ArticleSection {
	if len(sections) == 0 {
		return sections
	}

	minTotal, _ := targetWordRange(format)
	total := 0
	for _, section := range sections {
		if section.WordTarget < 120 {
			section.WordTarget = 120
		}
		total += section.WordTarget
	}
	if total >= minTotal {
		return sections
	}

	deficit := minTotal - total
	perSectionBoost := deficit / len(sections)
	if perSectionBoost < 80 {
		perSectionBoost = 80
	}

	out := make([]ArticleSection, len(sections))
	for i, section := range sections {
		out[i] = section
		if strings.Contains(strings.ToLower(section.Heading), "referenc") || strings.Contains(strings.ToLower(section.Heading), "bibliograf") {
			out[i].WordTarget = maxInt(section.WordTarget, 220)
			continue
		}
		out[i].WordTarget = section.WordTarget + perSectionBoost
	}
	return out
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func defaultPlan(topic string, profile editorialProfile) []ArticleSection {
	switch profile.Format {
	case "newsletter":
		return []ArticleSection{
			{Heading: "Apertura", Angle: "gancho inicial y por que este tema importa ahora", WordTarget: 220},
			{Heading: "Lo Mas Importante", Angle: "hallazgos y datos clave en formato escaneable", WordTarget: 360},
			{Heading: "Contexto y Antecedentes", Angle: "de donde viene el tema y por que importa en PR", WordTarget: 320},
			{Heading: "Analisis", Angle: "contexto, implicaciones y lectura estrategica del tema", WordTarget: 380},
			{Heading: "Que Hacer con Esta Informacion", Angle: "recomendaciones, proximos pasos o conclusiones practicas", WordTarget: 260},
			{Heading: "Preguntas Clave", Angle: "FAQ breve para reforzar claridad y SEO", WordTarget: 220},
			{Heading: "Referencias y Recursos", Angle: "bibliografia y enlaces reales en estilo academico", WordTarget: 220},
		}
	case "memo", "report":
		return []ArticleSection{
			{Heading: "Resumen Ejecutivo", Angle: "sintesis del tema y por que importa", WordTarget: 260},
			{Heading: "Hallazgos Principales", Angle: "datos y hechos mas relevantes", WordTarget: 420},
			{Heading: "Contexto Operacional", Angle: "antecedentes, actores y marco de decision", WordTarget: 320},
			{Heading: "Analisis", Angle: "impacto, contexto y riesgos", WordTarget: 420},
			{Heading: "Escenarios y Riesgos", Angle: "posibles desarrollos, tensiones y tradeoffs", WordTarget: 320},
			{Heading: "Recomendaciones", Angle: "acciones o decisiones sugeridas", WordTarget: 260},
			{Heading: "Referencias y Recursos", Angle: "bibliografia y enlaces reales en estilo academico", WordTarget: 220},
		}
	}
	return []ArticleSection{
		{Heading: "Introduccion", Angle: "APP formula: contexto del tema en Puerto Rico", WordTarget: 260},
		{Heading: "Antecedentes y Contexto", Angle: "de donde viene el problema o tendencia y por que importa ahora", WordTarget: 320},
		{Heading: "Estado Actual", Angle: "situacion presente con datos y ejemplos", WordTarget: 420},
		{Heading: "Impacto en Puerto Rico", Angle: "como afecta a la isla y sus comunidades", WordTarget: 420},
		{Heading: "Retos y Obstaculos", Angle: "barreras, riesgos, tensiones o vacios institucionales", WordTarget: 300},
		{Heading: "Perspectivas y Desarrollo", Angle: "iniciativas, proyectos y futuro del tema", WordTarget: 360},
		{Heading: "Preguntas Frecuentes", Angle: "FAQ con 4-6 preguntas comunes sobre " + topic, WordTarget: 300},
		{Heading: "Conclusion", Angle: "resumen y llamado a accion", WordTarget: 220},
		{Heading: "Referencias y Recursos", Angle: "bibliografia con enlaces a fuentes externas reales: Wikipedia, gobierno, universidades, organizaciones", WordTarget: 220},
	}
}

func resolveEditorialProfile(escrito *models.Escrito) editorialProfile {
	profile := editorialProfile{
		Format:        strings.TrimSpace(escrito.OutputFormat),
		Style:         strings.TrimSpace(escrito.WritingStyle),
		Audience:      strings.TrimSpace(escrito.TargetAudience),
		CitationStyle: strings.TrimSpace(escrito.CitationStyle),
	}
	if profile.Format == "" || profile.Format == "auto" {
		profile.Format = inferOutputFormat(escrito.Topic)
	}
	if profile.Style == "" || profile.Style == "auto" {
		profile.Style = inferWritingStyle(profile.Format, escrito.Topic)
	}
	if profile.Audience == "" {
		profile.Audience = "general"
	}
	if profile.CitationStyle == "" {
		profile.CitationStyle = "apa"
	}
	return profile
}

func inferOutputFormat(topic string) string {
	lower := strings.ToLower(topic)
	switch {
	case strings.Contains(lower, "newsletter"):
		return "newsletter"
	case strings.Contains(lower, "memo"), strings.Contains(lower, "memorando"):
		return "memo"
	case strings.Contains(lower, "email"), strings.Contains(lower, "correo"):
		return "email"
	case strings.Contains(lower, "carta"):
		return "letter"
	case strings.Contains(lower, "reporte"), strings.Contains(lower, "informe"):
		return "report"
	default:
		return "article"
	}
}

func inferWritingStyle(format, topic string) string {
	lower := strings.ToLower(topic)
	switch format {
	case "newsletter":
		return "newsletter"
	case "memo", "email":
		return "executive"
	case "letter":
		return "institutional"
	}
	if strings.Contains(lower, "universidad") || strings.Contains(lower, "academ") || strings.Contains(lower, "investig") {
		return "academic"
	}
	return "journalistic"
}
