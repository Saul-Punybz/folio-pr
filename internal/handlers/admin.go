package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
	"github.com/Saul-Punybz/folio/internal/storage"
)

// AdminHandler groups admin-only HTTP handlers.
type AdminHandler struct {
	Articles     *models.ArticleStore
	Sources      *models.SourceStore
	Fingerprints *models.FingerprintStore
	AI           *ai.OllamaClient
	Scraper      *scraper.Scraper
	Storage      *storage.Client
}

// Reenrich handles POST /api/admin/reenrich.
// Clears garbage AI data, then re-enriches articles with empty summaries.
func (h *AdminHandler) Reenrich(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Step 1: Clear garbage summaries/tags.
	cleared, err := h.Articles.ClearGarbageEnrichment(ctx)
	if err != nil {
		slog.Error("reenrich: clear garbage", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to clear garbage data"})
		return
	}

	slog.Info("reenrich: cleared garbage summaries", "count", cleared)

	// Step 2: List articles needing re-enrichment.
	articles, err := h.Articles.ListNeedingEnrichment(ctx, 100)
	if err != nil {
		slog.Error("reenrich: list needing enrichment", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list articles"})
		return
	}

	// Step 3: Re-enrich in background (don't block the request).
	if len(articles) > 0 {
		go h.reenrichArticles(articles)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"cleared":  cleared,
		"queued":   len(articles),
		"message":  "Re-enrichment started. Articles will be processed in the background.",
	})
}

func (h *AdminHandler) reenrichArticles(articles []models.Article) {
	ctx := context.Background()
	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup

	for i := range articles {
		art := articles[i]
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			text := art.CleanText
			if len(text) > 8000 {
				text = text[:8000]
			}

			slog.Info("reenrich: processing", "id", art.ID, "title", art.Title)

			summary, err := h.AI.Summarize(ctx, text)
			if err != nil {
				slog.Error("reenrich: summarize", "id", art.ID, "err", err)
				return
			}

			tags, err := h.AI.Classify(ctx, text)
			if err != nil {
				slog.Error("reenrich: classify", "id", art.ID, "err", err)
				tags = nil
			}

			embedding, err := h.AI.Embed(ctx, text)
			if err != nil {
				slog.Error("reenrich: embed", "id", art.ID, "err", err)
				embedding = nil
			}

			if err := h.Articles.UpdateEnrichment(ctx, art.ID, summary, tags, embedding); err != nil {
				slog.Error("reenrich: update", "id", art.ID, "err", err)
				return
			}

			slog.Info("reenrich: complete", "id", art.ID)
		}()
	}

	wg.Wait()
	slog.Info("reenrich: all articles processed", "count", len(articles))
}

// TriggerIngest handles POST /api/admin/ingest.
// Manually triggers the RSS/scraper ingestion cycle.
func (h *AdminHandler) TriggerIngest(w http.ResponseWriter, r *http.Request) {
	if h.Sources == nil || h.Fingerprints == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ingestion not configured"})
		return
	}

	stores := scraper.Stores{
		Articles:     h.Articles,
		Sources:      h.Sources,
		Fingerprints: h.Fingerprints,
	}

	go scraper.RunIngestion(context.Background(), stores, h.Scraper, h.AI, h.Storage)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Ingestion started in background. New articles will appear shortly.",
	})
}

// ChatWithNews handles POST /api/admin/chat.
// Uses a multi-step approach: 1) search local DB, 2) multi-engine web search, 3) AI.
func (h *AdminHandler) ChatWithNews(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question is required"})
		return
	}

	ctx := r.Context()

	// Step 1: Search for articles matching the question keywords (OR-based ILIKE).
	searched, err := h.Articles.SearchChat(ctx, body.Question, 10)
	if err != nil {
		slog.Error("chat: search articles", "err", err)
	}

	// Step 2: Also get the most recent articles for general context.
	recent, _ := h.Articles.ListRecent(ctx, 20)

	// Merge: searched articles first (most relevant), then recent (deduped). Cap at 15.
	seen := make(map[string]bool)
	var merged []models.Article
	for _, a := range searched {
		if !seen[a.ID.String()] {
			seen[a.ID.String()] = true
			merged = append(merged, a)
		}
	}
	for _, a := range recent {
		if !seen[a.ID.String()] && len(merged) < 15 {
			seen[a.ID.String()] = true
			merged = append(merged, a)
		}
	}

	// Step 3: Build 2-3 search queries from the user's question.
	webQueries := buildChatSearchQueries(body.Question)

	// Step 4: Run multi-engine search (DDG + Bing) for each query in parallel.
	var allWebResults []scraper.WebResult
	webSeen := make(map[string]bool)

	type queryResult struct {
		results []scraper.WebResult
	}
	qrCh := make(chan queryResult, len(webQueries))

	var wg sync.WaitGroup
	for _, q := range webQueries {
		wg.Add(1)
		go func(query string) {
			defer wg.Done()
			results, err := scraper.MultiWebSearch(ctx, query, 8)
			if err != nil {
				slog.Warn("chat: multi web search failed", "query", query, "err", err)
			}
			qrCh <- queryResult{results}
		}(q)
	}
	wg.Wait()
	close(qrCh)

	for qr := range qrCh {
		for _, wr := range qr.results {
			key := strings.ToLower(wr.URL)
			if !webSeen[key] {
				webSeen[key] = true
				allWebResults = append(allWebResults, wr)
			}
		}
	}

	// Filter out results that are clearly not about Puerto Rico. Cap at 10.
	allWebResults = filterPRResults(allWebResults)
	slog.Info("chat: web search results", "count", len(allWebResults))

	slog.Info("chat: context built", "searched", len(searched), "recent", len(recent), "merged", len(merged), "web", len(allWebResults))

	// Step 5: Build compact context — local articles + web results.
	numSearched := len(searched)
	var sb strings.Builder
	for i, a := range merged {
		if sb.Len() > 4000 {
			break
		}
		sb.WriteString(fmt.Sprintf("%d. %s [%s] %s", i+1, a.Title, a.Source, a.URL))
		if i < numSearched && a.Summary != "" {
			s := a.Summary
			if len(s) > 200 {
				s = s[:200] + "..."
			}
			sb.WriteString("\n   → " + s)
		}
		sb.WriteString("\n")
	}

	// Append web results if we have them.
	if len(allWebResults) > 0 {
		sb.WriteString("\n--- RESULTADOS DE INTERNET ---\n")
		for i, wr := range allWebResults {
			if sb.Len() > 5500 {
				break
			}
			sb.WriteString(fmt.Sprintf("WEB %d. %s %s", i+1, wr.Title, wr.URL))
			if wr.Snippet != "" {
				s := wr.Snippet
				if len(s) > 200 {
					s = s[:200] + "..."
				}
				sb.WriteString("\n   → " + s)
			}
			sb.WriteString("\n")
		}
	}

	newsContext := sb.String()

	systemPrompt := `Eres analista de noticias de Puerto Rico. Tu trabajo es RESUMIR la información de las fuentes que se te proporcionan abajo.

REGLAS ESTRICTAS:
1. LEE TODOS los resultados abajo (locales e internet). La respuesta ESTÁ en esos resultados.
2. RESUME lo que dicen los títulos y snippets. NO digas "no encontré" si hay resultados relevantes abajo.
3. Menciona los nombres, fechas y hechos específicos que aparecen en los títulos.
4. IGNORA resultados que NO sean sobre Puerto Rico (otros países, República Dominicana, etc.) a menos que mencionen a PR directamente.
5. SOLO di "No encontré información" si NINGUNO de los resultados abajo es relevante a la pregunta.
6. Responde en español, breve y directo.
7. NO inventes información. Solo usa lo que aparece en los resultados.

` + newsContext

	// Use the fast 3b model for interactive chat — 8b is too slow (~30s).
	answer, err := h.AI.GenerateWithModel(ctx, "llama3.2:3b", systemPrompt, body.Question)
	if err != nil {
		slog.Error("chat: generate", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI failed to respond"})
		return
	}

	type webSourceLink struct {
		Title   string `json:"title"`
		Source  string `json:"source"`
		URL     string `json:"url"`
		Snippet string `json:"snippet,omitempty"`
		Savable bool   `json:"savable"`
	}

	type sourceLink struct {
		Title  string `json:"title"`
		Source string `json:"source"`
		URL    string `json:"url"`
	}

	// Build web source links with savable flag.
	var webSources []webSourceLink
	for _, wr := range allWebResults {
		savable := true
		exists, err := h.Articles.ExistsByURL(ctx, wr.URL)
		if err == nil && exists {
			savable = false
		}
		snippet := wr.Snippet
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		webSources = append(webSources, webSourceLink{
			Title:   wr.Title,
			Source:  "Internet",
			URL:     wr.URL,
			Snippet: snippet,
			Savable: savable,
		})
	}

	// Build local source links — only from articles whose titles the AI answer
	// actually references (simple substring check), to avoid showing irrelevant noise.
	var sources []sourceLink
	answerLower := strings.ToLower(answer)
	for _, a := range searched {
		if len(sources) >= 5 {
			break
		}
		// Check if the AI answer mentions words from this article's title.
		titleWords := strings.Fields(strings.ToLower(a.Title))
		matches := 0
		for _, w := range titleWords {
			if len(w) > 4 && strings.Contains(answerLower, w) {
				matches++
			}
		}
		if matches >= 2 {
			sources = append(sources, sourceLink{Title: a.Title, Source: a.Source, URL: a.URL})
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"answer":        answer,
		"articles_used": len(merged),
		"sources":       sources,
		"web_sources":   webSources,
	})
}

// buildChatSearchQueries generates 2-3 search queries from the user's question.
func buildChatSearchQueries(question string) []string {
	base := question
	hasPR := strings.Contains(strings.ToLower(question), "puerto rico") ||
		strings.Contains(strings.ToLower(question), "PR")

	if !hasPR {
		base += " Puerto Rico"
	}

	queries := []string{base}

	// Add a time-scoped variant for freshness.
	queries = append(queries, base+" noticias 2026")

	// If the question is long enough, try a shortened version with just key nouns.
	words := strings.Fields(question)
	if len(words) > 4 {
		// Take the last 3 significant words as an additional query.
		short := strings.Join(words[len(words)-3:], " ")
		if !hasPR {
			short += " Puerto Rico"
		}
		queries = append(queries, short)
	}

	return queries
}

// prRelevantDomains are domains known to be Puerto Rico news/government sites.
var prRelevantDomains = []string{
	"elnuevodia.com", "metro.pr", "primerahora.com", "radioisla.tv",
	"esnoticiapr.com", "newsismybusiness.com", "noticel.com", "teleonce.com",
	"periodicoinvestigativo.com", "eyboricua.com", "notiuno.com",
	"gobierno.pr", ".pr/", "puertorico", "puerto rico",
	"fundacionangelramos.org", "gao.gov", "federalregister.gov",
	"grants.gov", "fema.gov", "hud.gov",
}

// prIrrelevantPatterns are patterns that indicate non-PR content to exclude.
var prIrrelevantPatterns = []string{
	"dominicana", "dominicano", "santo domingo", "república dominicana",
	"mexico", "méxico", "colombia", "venezuela", "argentina",
	"españa", "spain", "paraguay", "chile", "perú", "peru",
	"cuba", "panamá", "panama", "ecuador", "bolivia",
	"google noticias", "news.google.com",
}

// filterPRResults removes web search results that are clearly not about Puerto Rico.
func filterPRResults(results []scraper.WebResult) []scraper.WebResult {
	var filtered []scraper.WebResult
	for _, r := range results {
		lower := strings.ToLower(r.Title + " " + r.Snippet + " " + r.URL)

		// Skip generic homepages/aggregators (not specific articles).
		if strings.HasSuffix(r.URL, ".com/") || strings.HasSuffix(r.URL, ".com") ||
			strings.HasSuffix(r.URL, "/noticias/") || strings.Contains(lower, "últimas noticias") {
			continue
		}

		// Check if it contains irrelevant country/location patterns.
		irrelevant := false
		for _, pattern := range prIrrelevantPatterns {
			if strings.Contains(lower, pattern) {
				// Exception: if it also mentions Puerto Rico, keep it.
				if !strings.Contains(lower, "puerto rico") && !strings.Contains(lower, "boricua") {
					irrelevant = true
					break
				}
			}
		}
		if irrelevant {
			continue
		}

		filtered = append(filtered, r)
		if len(filtered) >= 10 {
			break
		}
	}
	return filtered
}
