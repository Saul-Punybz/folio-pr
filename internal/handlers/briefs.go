package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// BriefHandler groups daily brief HTTP handlers.
type BriefHandler struct {
	Briefs   *models.BriefStore
	Articles *models.ArticleStore
	AI       *ai.OllamaClient
}

// GetLatestBrief handles GET /api/briefs/latest.
// Returns the most recent daily brief.
func (h *BriefHandler) GetLatestBrief(w http.ResponseWriter, r *http.Request) {
	brief, err := h.Briefs.GetLatest(r.Context())
	if err != nil {
		slog.Error("get latest brief", "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "no briefs available"})
		return
	}

	writeJSON(w, http.StatusOK, brief)
}

// GenerateBrief handles POST /api/briefs/generate.
// Manually triggers daily brief generation.
func (h *BriefHandler) GenerateBrief(w http.ResponseWriter, r *http.Request) {
	if h.Articles == nil || h.AI == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "AI not configured"})
		return
	}

	go scraper.GenerateDailyBrief(context.Background(), h.Articles, h.Briefs, h.AI)

	writeJSON(w, http.StatusAccepted, map[string]string{"status": "generating"})
}

// ListBriefs handles GET /api/briefs?limit=7.
// Returns recent daily briefs.
func (h *BriefHandler) ListBriefs(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 7
	}

	briefs, err := h.Briefs.List(r.Context(), limit)
	if err != nil {
		slog.Error("list briefs", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	if briefs == nil {
		briefs = []models.Brief{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"briefs": briefs,
		"count":  len(briefs),
	})
}
