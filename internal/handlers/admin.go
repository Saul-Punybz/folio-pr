package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/intelligence"
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
func (h *AdminHandler) ChatWithNews(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question is required"})
		return
	}

	resp, err := intelligence.Chat(r.Context(), intelligence.Deps{
		Articles: h.Articles,
		AI:       h.AI,
	}, intelligence.ChatRequest{
		Question: body.Question,
	})
	if err != nil {
		slog.Error("chat: generate", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "AI failed to respond"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"answer":        resp.Answer,
		"articles_used": resp.ArticlesUsed,
		"sources":       resp.Sources,
		"web_sources":   resp.WebSources,
	})
}
