// Package generator implements the Escritos SEO article generation pipeline.
// It runs a 4-phase process: source gathering, planning, generation, scoring.
package generator

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
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

// RunGeneration orchestrates all 4 phases for a single escrito.
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

	// ── Phase 1: Source Gathering ──
	slog.Info("generator: phase 1 — source gathering", "topic", escrito.Topic)
	sources, err := runPhase1(ctx, deps, escrito, articleIDs)
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

	slog.Info("generator: phase 2 — article planning", "topic", escrito.Topic, "sources", len(sources))
	sections, err := runPhase2(ctx, deps, escrito, sources)
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

	slog.Info("generator: phase 3 — generating content", "topic", escrito.Topic, "sections", len(sections))
	err = runPhase3(ctx, deps, escrito, sources, sections)
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
	err = runPhase4(ctx, deps, escrito)
	if err != nil {
		slog.Error("generator: phase 4 failed", "err", err)
		_ = deps.Escritos.SetFailed(ctx, escrito.ID, "Phase 4 failed: "+err.Error())
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

func runPhase1(ctx context.Context, deps Deps, escrito *models.Escrito, articleIDs []uuid.UUID) ([]sourceArticle, error) {
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

	// Cap at 15 sources
	if len(articles) > 15 {
		articles = articles[:15]
		relevances = relevances[:15]
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
		_ = deps.Sources.Create(ctx, &models.EscritoSource{
			EscritoID:     escrito.ID,
			ArticleID:     a.ID,
			Relevance:     rel,
			UsedInSection: "",
		})
	}

	return articles, nil
}

// ── Phase 2: Article Planning ──────────────────────────────────────

func runPhase2(ctx context.Context, deps Deps, escrito *models.Escrito, sources []sourceArticle) ([]ArticleSection, error) {
	// Build context from sources (cap at 8000 chars)
	var contextBuf strings.Builder
	for _, s := range sources {
		entry := fmt.Sprintf("- %s: %s\n", s.Title, truncate(s.Summary, 200))
		if contextBuf.Len()+len(entry) > 8000 {
			break
		}
		contextBuf.WriteString(entry)
	}

	systemPrompt := `Eres un editor SEO experto en contenido en español para Puerto Rico.
Tu tarea es crear un plan de articulo SEO basado en las fuentes proporcionadas.

REGLAS ESTRICTAS:
- Responde SOLO con un JSON array valido, sin texto adicional
- Cada elemento tiene: heading, angle, word_target
- Estructura obligatoria:
  1. Introduccion (formula APP: Agree-Promise-Preview) — 150-200 palabras
  2. 3-5 secciones H2 con angulos especificos — 200-400 palabras cada una
  3. Preguntas Frecuentes (FAQ) con 3-5 preguntas — 200-300 palabras
  4. Conclusion con resumen y llamado a accion — 150-200 palabras
  5. Referencias y Recursos — lista de fuentes externas con enlaces reales
- Total target: 1200-2000 palabras
- Todos los headings en español
- La seccion final SIEMPRE debe ser "Referencias y Recursos"`

	userPrompt := fmt.Sprintf(`Tema: %s

Fuentes disponibles:
%s

Genera el plan de articulo como JSON array. Ejemplo de formato:
[{"heading":"Introduccion","angle":"APP formula: contexto del tema en PR","word_target":180},{"heading":"Estado Actual de [Tema]","angle":"situacion presente con datos","word_target":300}]`, escrito.Topic, contextBuf.String())

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
		sections = defaultPlan(escrito.Topic)
	}

	if len(sections) < 3 {
		sections = defaultPlan(escrito.Topic)
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

	return sections, nil
}

// ── Phase 3: Section Generation ────────────────────────────────────

func runPhase3(ctx context.Context, deps Deps, escrito *models.Escrito, sources []sourceArticle, sections []ArticleSection) error {
	// Build source context (cap at 6000 chars)
	var srcContext strings.Builder
	for _, s := range sources {
		entry := fmt.Sprintf("Fuente: %s (%s)\nResumen: %s\n\n", s.Title, s.Source, truncate(s.Summary, 300))
		if srcContext.Len()+len(entry) > 6000 {
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

		sectionContent, err := generateSection(ctx, deps.AI, escrito.Topic, section, srcContext.String())
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
	title, slug, metaDesc, keywords, hashtags := generateMetadata(ctx, deps.AI, escrito.Topic, content)

	wordCount := countWordsInText(content)
	planJSON, _ := json.Marshal(sections)

	err := deps.Escritos.UpdateContent(ctx, escrito.ID, title, slug, metaDesc, content, keywords, hashtags, wordCount, planJSON)
	if err != nil {
		return fmt.Errorf("save content: %w", err)
	}

	return nil
}

func generateSection(ctx context.Context, aiClient *ai.OllamaClient, topic string, section ArticleSection, srcContext string) (string, error) {
	isReferences := strings.Contains(strings.ToLower(section.Heading), "referencia") ||
		strings.Contains(strings.ToLower(section.Heading), "recurso") ||
		strings.Contains(strings.ToLower(section.Heading), "bibliograf")

	var systemPrompt string
	if isReferences {
		systemPrompt = fmt.Sprintf(`Eres un investigador experto que compila bibliografias para articulos en español sobre Puerto Rico.
Genera la seccion "%s" para un articulo sobre "%s".

REGLAS ESTRICTAS:
- Compila una lista de 8-15 fuentes externas REALES y verificables
- Formato markdown con hyperlinks: [Nombre de la Fuente](URL)
- PRIORIZA estas fuentes reales:
  * Wikipedia en español (es.wikipedia.org)
  * Agencias de gobierno de PR (.pr.gov, energia.pr.gov, aee.pr.gov)
  * Gobierno federal de EE.UU. (.gov - DOE, EPA, EIA, FEMA)
  * Universidades de PR (UPR, Inter, Sagrado)
  * Organizaciones internacionales (ONU, IRENA, IEA)
  * Medios de PR verificados (elnuevodia.com, primerahora.com, metro.pr)
- Las URLs deben ser de dominios reales que existen
- Agrupa por tipo: Gobierno, Academicas, Organizaciones, Medios
- Comienza con "## %s"
- Responde SOLO con el contenido de la seccion`, section.Heading, topic, section.Heading)
	} else {
		systemPrompt = fmt.Sprintf(`Eres un escritor SEO experto en español para Puerto Rico.
Escribe la seccion "%s" de un articulo sobre "%s".

REGLAS ESTRICTAS:
- Escribe EXACTAMENTE en español
- NO uses frases como "es importante destacar", "cabe señalar", "sin duda alguna"
- NO uses lenguaje artificial o generico
- Escribe con voz activa, datos concretos, ejemplos locales de PR
- Cuando cites un dato, estadistica o hecho, incluye la fuente entre parentesis
  Ejemplo: "La generacion solar aumento un 45%% en 2024 (Autoridad de Energia Electrica de PR)"
- Incluye 1-2 hyperlinks por seccion a fuentes externas reales en formato markdown
  Ejemplo: segun [IRENA](https://www.irena.org/), la capacidad instalada...
  Fuentes validas: Wikipedia, .gov, .pr.gov, universidades, organizaciones internacionales
- Target: aproximadamente %d palabras
- Si es la primera seccion (Introduccion), NO incluyas heading H2
- Para las demas secciones, comienza con "## %s"
- NO incluyas saludo, despedida ni meta-comentarios
- Responde SOLO con el contenido de la seccion`, section.Heading, topic, section.WordTarget, section.Heading)
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

func generateMetadata(ctx context.Context, aiClient *ai.OllamaClient, topic, content string) (title, slug, metaDesc string, keywords, hashtags []string) {
	// Defaults
	title = topic
	slug = slugify(topic)
	metaDesc = truncate(content, 155)
	keywords = []string{strings.ToLower(topic)}
	hashtags = []string{"#" + strings.ReplaceAll(strings.ToLower(topic), " ", "")}

	if aiClient == nil {
		return
	}

	systemPrompt := `Eres un especialista SEO. Genera metadatos para un articulo en español.

REGLAS:
- Responde SOLO con JSON valido, sin texto adicional
- Formato exacto: {"title":"...","slug":"...","meta_description":"...","keywords":["..."],"hashtags":["..."]}
- Title: 50-60 caracteres, incluir keyword principal
- Slug: solo minusculas, guiones, sin acentos
- Meta description: 150-160 caracteres, incluir keyword
- Keywords: 5-8 keywords relevantes
- Hashtags: 5-8 hashtags con # prefix, en español`

	userPrompt := fmt.Sprintf("Tema: %s\n\nPrimeros 500 caracteres del articulo:\n%s",
		topic, truncate(content, 500))

	result, err := aiClient.Generate(ctx, systemPrompt, userPrompt)
	if err != nil {
		return
	}

	result = extractJSON(result)

	var meta struct {
		Title   string   `json:"title"`
		Slug    string   `json:"slug"`
		Meta    string   `json:"meta_description"`
		KW      []string `json:"keywords"`
		HT      []string `json:"hashtags"`
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

// ── Improve Content ──────────────────────────────────────────────

// ImproveContent takes existing escrito content and user instructions,
// then asks the AI to rewrite/improve the article accordingly.
func ImproveContent(ctx context.Context, aiClient *ai.OllamaClient, escrito *models.Escrito, instructions string) (string, error) {
	systemPrompt := `Eres un editor experto en contenido SEO en español para Puerto Rico.
Tu tarea es MEJORAR un articulo existente segun las instrucciones del usuario.

REGLAS:
- Mantén la estructura H2 existente (puedes agregar secciones si las instrucciones lo piden)
- Conserva todos los hyperlinks y referencias existentes
- Si el usuario pide agregar fuentes, usa URLs reales de sitios verificables
- Mantén el idioma en español
- NO uses frases genericas de IA ("es importante destacar", "cabe señalar")
- Devuelve SOLO el articulo completo mejorado en markdown, sin comentarios adicionales`

	// Cap content to avoid exceeding model context
	content := escrito.Content
	if len(content) > 12000 {
		content = content[:12000]
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

// CountWords is the exported version of countWordsInText.
func CountWords(text string) int {
	return countWordsInText(text)
}

// ── Phase 4: SEO Scoring ──────────────────────────────────────────

func runPhase4(ctx context.Context, deps Deps, escrito *models.Escrito) error {
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
	scoreJSON, err := json.Marshal(score)
	if err != nil {
		return fmt.Errorf("marshal seo score: %w", err)
	}

	return deps.Escritos.UpdateSEOScore(ctx, escrito.ID, scoreJSON)
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

func defaultPlan(topic string) []ArticleSection {
	return []ArticleSection{
		{Heading: "Introduccion", Angle: "APP formula: contexto del tema en Puerto Rico", WordTarget: 180},
		{Heading: "Estado Actual", Angle: "situacion presente con datos y ejemplos", WordTarget: 300},
		{Heading: "Impacto en Puerto Rico", Angle: "como afecta a la isla y sus comunidades", WordTarget: 300},
		{Heading: "Perspectivas y Desarrollo", Angle: "iniciativas, proyectos y futuro del tema", WordTarget: 300},
		{Heading: "Preguntas Frecuentes", Angle: "FAQ con 3-5 preguntas comunes sobre " + topic, WordTarget: 250},
		{Heading: "Conclusion", Angle: "resumen y llamado a accion", WordTarget: 170},
		{Heading: "Referencias y Recursos", Angle: "bibliografia con enlaces a fuentes externas reales: Wikipedia, gobierno, universidades, organizaciones", WordTarget: 200},
	}
}
