package intelligence

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// Deps groups dependencies needed by the intelligence package.
type Deps struct {
	Articles *models.ArticleStore
	AI       *ai.OllamaClient
}

// Chat performs AI-powered news chat: searches local DB, runs web search, and
// generates an AI response.
func Chat(ctx context.Context, deps Deps, req ChatRequest) (*ChatResponse, error) {
	// Apply defaults.
	if req.MaxArticles == 0 {
		req.MaxArticles = 15
	}
	if req.Model == "" {
		req.Model = "llama3.2:3b"
	}

	// Step 1: Search for articles matching the question keywords (OR-based ILIKE).
	searched, err := deps.Articles.SearchChat(ctx, req.Question, 10)
	if err != nil {
		slog.Error("chat: search articles", "err", err)
	}

	// Step 2: Also get the most recent articles for general context.
	recent, _ := deps.Articles.ListRecent(ctx, 20)

	// Merge: searched articles first (most relevant), then recent (deduped).
	// Cap at MaxArticles.
	seen := make(map[string]bool)
	var merged []models.Article
	for _, a := range searched {
		if !seen[a.ID.String()] {
			seen[a.ID.String()] = true
			merged = append(merged, a)
		}
	}
	for _, a := range recent {
		if !seen[a.ID.String()] && len(merged) < req.MaxArticles {
			seen[a.ID.String()] = true
			merged = append(merged, a)
		}
	}

	// Step 3: Build 2-3 search queries from the user's question.
	webQueries := buildChatSearchQueries(req.Question)

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

	// Step 5: Build compact context -- local articles + web results.
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
			sb.WriteString("\n   -> " + s)
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
				sb.WriteString("\n   -> " + s)
			}
			sb.WriteString("\n")
		}
	}

	newsContext := sb.String()

	systemPrompt := `Eres analista de noticias de Puerto Rico. Tu trabajo es RESUMIR la informacion de las fuentes que se te proporcionan abajo.

REGLAS ESTRICTAS:
1. LEE TODOS los resultados abajo (locales e internet). La respuesta ESTA en esos resultados.
2. RESUME lo que dicen los titulos y snippets. NO digas "no encontre" si hay resultados relevantes abajo.
3. Menciona los nombres, fechas y hechos especificos que aparecen en los titulos.
4. IGNORA resultados que NO sean sobre Puerto Rico (otros paises, Republica Dominicana, etc.) a menos que mencionen a PR directamente.
5. SOLO di "No encontre informacion" si NINGUNO de los resultados abajo es relevante a la pregunta.
6. Responde en espanol, breve y directo.
7. NO inventes informacion. Solo usa lo que aparece en los resultados.

` + newsContext

	// Use the specified model for interactive chat.
	answer, err := deps.AI.GenerateWithModel(ctx, req.Model, systemPrompt, req.Question)
	if err != nil {
		return nil, fmt.Errorf("chat: AI generate: %w", err)
	}

	// Build web source links with savable flag.
	var webSources []WebSource
	for _, wr := range allWebResults {
		savable := true
		exists, err := deps.Articles.ExistsByURL(ctx, wr.URL)
		if err == nil && exists {
			savable = false
		}
		snippet := wr.Snippet
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		webSources = append(webSources, WebSource{
			Title:   wr.Title,
			Source:  "Internet",
			URL:     wr.URL,
			Snippet: snippet,
			Savable: savable,
		})
	}

	// Build local source links -- only from articles whose titles the AI answer
	// actually references (simple substring check), to avoid showing irrelevant noise.
	var sources []LocalSource
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
			sources = append(sources, LocalSource{Title: a.Title, Source: a.Source, URL: a.URL})
		}
	}

	return &ChatResponse{
		Answer:       answer,
		ArticlesUsed: len(merged),
		Sources:      sources,
		WebSources:   webSources,
	}, nil
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
	"dominicana", "dominicano", "santo domingo", "republica dominicana",
	"mexico", "colombia", "venezuela", "argentina",
	"spain", "paraguay", "chile", "peru",
	"cuba", "panama", "ecuador", "bolivia",
	"google noticias", "news.google.com",
}

// filterPRResults removes web search results that are clearly not about Puerto Rico.
func filterPRResults(results []scraper.WebResult) []scraper.WebResult {
	var filtered []scraper.WebResult
	for _, r := range results {
		lower := strings.ToLower(r.Title + " " + r.Snippet + " " + r.URL)

		// Skip generic homepages/aggregators (not specific articles).
		if strings.HasSuffix(r.URL, ".com/") || strings.HasSuffix(r.URL, ".com") ||
			strings.HasSuffix(r.URL, "/noticias/") || strings.Contains(lower, "ultimas noticias") {
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

// compile-time guard: prRelevantDomains is declared for potential future use in
// domain-based relevance boosting.
var _ = prRelevantDomains
