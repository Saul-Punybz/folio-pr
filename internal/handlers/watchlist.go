package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/agents"
	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
)

// WatchlistHandler groups watchlist HTTP handlers.
type WatchlistHandler struct {
	Orgs     *models.WatchlistOrgStore
	Hits     *models.WatchlistHitStore
	Articles *models.ArticleStore
	AI       *ai.OllamaClient
}

// ── Org endpoints ────────────────────────────────────────────────

// ListOrgs handles GET /api/watchlist/orgs.
func (h *WatchlistHandler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	orgs, err := h.Orgs.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list watchlist orgs", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if orgs == nil {
		orgs = []models.WatchlistOrg{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"orgs": orgs, "count": len(orgs)})
}

type createOrgRequest struct {
	Name            string   `json:"name"`
	Website         string   `json:"website"`
	Keywords        []string `json:"keywords"`
	YouTubeChannels []string `json:"youtube_channels"`
}

// CreateOrg handles POST /api/watchlist/orgs.
func (h *WatchlistHandler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.Keywords == nil {
		req.Keywords = []string{}
	}
	if req.YouTubeChannels == nil {
		req.YouTubeChannels = []string{}
	}

	org := &models.WatchlistOrg{
		UserID:          user.ID,
		Name:            req.Name,
		Website:         strings.TrimSpace(req.Website),
		Keywords:        req.Keywords,
		YouTubeChannels: req.YouTubeChannels,
		Active:          true,
	}

	if err := h.Orgs.Create(r.Context(), org); err != nil {
		slog.Error("create watchlist org", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create org"})
		return
	}

	writeJSON(w, http.StatusCreated, org)
}

type updateOrgRequest struct {
	Name            string   `json:"name"`
	Website         string   `json:"website"`
	Keywords        []string `json:"keywords"`
	YouTubeChannels []string `json:"youtube_channels"`
	Active          *bool    `json:"active,omitempty"`
}

// UpdateOrg handles PUT /api/watchlist/orgs/{id}.
func (h *WatchlistHandler) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	var req updateOrgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Name == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "name is required"})
		return
	}

	if req.Keywords == nil {
		req.Keywords = []string{}
	}
	if req.YouTubeChannels == nil {
		req.YouTubeChannels = []string{}
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	org := &models.WatchlistOrg{
		ID:              id,
		Name:            req.Name,
		Website:         strings.TrimSpace(req.Website),
		Keywords:        req.Keywords,
		YouTubeChannels: req.YouTubeChannels,
		Active:          active,
	}

	if err := h.Orgs.Update(r.Context(), org); err != nil {
		slog.Error("update watchlist org", "id", id, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		return
	}

	writeJSON(w, http.StatusOK, org)
}

// DeleteOrg handles DELETE /api/watchlist/orgs/{id}.
func (h *WatchlistHandler) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	if err := h.Orgs.Delete(r.Context(), id); err != nil {
		slog.Error("delete watchlist org", "id", id, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ToggleOrg handles PATCH /api/watchlist/orgs/{id}/toggle.
func (h *WatchlistHandler) ToggleOrg(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.Orgs.ToggleActive(r.Context(), id, body.Active); err != nil {
		slog.Error("toggle watchlist org", "id", id, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

// ── Hit endpoints ────────────────────────────────────────────────

// ListHits handles GET /api/watchlist/hits?limit=50&offset=0&org_id=...&source_type=...
func (h *WatchlistHandler) ListHits(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	orgIDStr := r.URL.Query().Get("org_id")

	var hits []models.WatchlistHit
	var err error

	if orgIDStr != "" {
		orgID, parseErr := uuid.Parse(orgIDStr)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org_id"})
			return
		}
		hits, err = h.Hits.ListByOrg(r.Context(), orgID, limit, offset)
	} else {
		hits, err = h.Hits.ListByUser(r.Context(), user.ID, limit, offset)
	}

	if err != nil {
		slog.Error("list watchlist hits", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if hits == nil {
		hits = []models.WatchlistHit{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"hits": hits, "count": len(hits)})
}

// CountUnseen handles GET /api/watchlist/hits/unseen.
func (h *WatchlistHandler) CountUnseen(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	count, err := h.Hits.CountUnseenByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("count unseen hits", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"unseen": count})
}

// MarkSeen handles POST /api/watchlist/hits/{id}/seen.
func (h *WatchlistHandler) MarkSeen(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid hit id"})
		return
	}

	if err := h.Hits.MarkSeen(r.Context(), id); err != nil {
		slog.Error("mark hit seen", "id", id, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "hit not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "seen"})
}

// MarkAllSeen handles POST /api/watchlist/hits/seen-all.
func (h *WatchlistHandler) MarkAllSeen(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	count, err := h.Hits.MarkAllSeenByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("mark all hits seen", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"status": "done", "marked": count})
}

// DeleteHit handles DELETE /api/watchlist/hits/{id}.
func (h *WatchlistHandler) DeleteHit(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid hit id"})
		return
	}

	if err := h.Hits.Delete(r.Context(), id); err != nil {
		slog.Error("delete hit", "id", id, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "hit not found"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// EnrichOrg handles POST /api/watchlist/orgs/{id}/enrich.
// Uses AI to search for the org's website and extract keywords.
func (h *WatchlistHandler) EnrichOrg(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}

	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	// Find the org.
	orgs, err := h.Orgs.ListByUser(r.Context(), user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	var org *models.WatchlistOrg
	for i := range orgs {
		if orgs[i].ID == id {
			org = &orgs[i]
			break
		}
	}
	if org == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		return
	}

	// Run enrichment (this takes a few seconds — use a background context with timeout).
	keywords, enrichErr := agents.EnrichOrgKeywords(context.Background(), org.Name, org.Website, h.AI)
	if enrichErr != nil {
		slog.Error("enrich org keywords", "id", id, "err", enrichErr)
		writeJSON(w, http.StatusOK, map[string]any{
			"status":   "partial",
			"keywords": org.Keywords,
			"message":  "No se pudieron extraer palabras clave automaticamente.",
		})
		return
	}

	// Merge: keep any user-defined keywords not already in the AI list.
	merged := mergeKeywords(keywords, org.Keywords)

	// Update the org with enriched keywords.
	org.Keywords = merged
	if err := h.Orgs.Update(r.Context(), org); err != nil {
		slog.Error("enrich org update", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to update org"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":   "enriched",
		"keywords": merged,
		"message":  fmt.Sprintf("Se extrajeron %d palabras clave automaticamente.", len(keywords)),
	})
}

// mergeKeywords combines AI-extracted keywords with user-provided ones, deduplicating.
func mergeKeywords(aiKeywords, userKeywords []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, kw := range aiKeywords {
		lower := strings.ToLower(strings.TrimSpace(kw))
		if !seen[lower] && kw != "" {
			seen[lower] = true
			result = append(result, strings.TrimSpace(kw))
		}
	}
	for _, kw := range userKeywords {
		lower := strings.ToLower(strings.TrimSpace(kw))
		if !seen[lower] && kw != "" {
			seen[lower] = true
			result = append(result, strings.TrimSpace(kw))
		}
	}
	return result
}

// TriggerScan handles POST /api/watchlist/scan.
// Launches the watchlist scan in the background and returns immediately.
func (h *WatchlistHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	go agents.RunWatchlistScan(context.Background(), agents.Deps{
		Orgs:     h.Orgs,
		Hits:     h.Hits,
		Articles: h.Articles,
		AI:       h.AI,
	})

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Escaneo iniciado. Los resultados aparecerán en segundos.",
	})
}
