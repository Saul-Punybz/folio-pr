package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
)

// OpportunityHandler handles grant/RFP opportunity API endpoints.
type OpportunityHandler struct {
	Opportunities *models.OpportunityStore
}

// List returns paginated opportunities with optional filters.
func (h *OpportunityHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	status := q.Get("status")
	category := q.Get("category")
	sort := q.Get("sort")
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	var domainID *uuid.UUID
	if did := q.Get("domain_id"); did != "" {
		if parsed, err := uuid.Parse(did); err == nil {
			domainID = &parsed
		}
	}

	opps, err := h.Opportunities.ListAll(r.Context(), status, category, domainID, sort, limit, offset)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if opps == nil {
		opps = []models.Opportunity{}
	}

	total, _ := h.Opportunities.TotalCount(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{"opportunities": opps, "count": len(opps), "total": total})
}

// Stats returns opportunity counts by status and category.
func (h *OpportunityHandler) Stats(w http.ResponseWriter, r *http.Request) {
	statusCounts, _ := h.Opportunities.CountByStatus(r.Context())
	total, _ := h.Opportunities.TotalCount(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"total":     total,
		"by_status": statusCounts,
	})
}

// Search performs full-text search on opportunities.
func (h *OpportunityHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing q parameter"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	opps, err := h.Opportunities.SearchFTS(r.Context(), query, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if opps == nil {
		opps = []models.Opportunity{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"opportunities": opps, "count": len(opps)})
}

// Get returns a single opportunity by ID.
func (h *OpportunityHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	opp, err := h.Opportunities.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, opp)
}

// Update modifies an opportunity (manual correction).
func (h *OpportunityHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req struct {
		Title        string `json:"title"`
		Agency       string `json:"agency"`
		Summary      string `json:"summary"`
		Amount       string `json:"amount"`
		Deadline     string `json:"deadline"`
		ContactName  string `json:"contact_name"`
		ContactEmail string `json:"contact_email"`
		Category     string `json:"category"`
		Status       string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	opp, err := h.Opportunities.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}

	if req.Title != "" {
		opp.Title = req.Title
	}
	if req.Agency != "" {
		opp.Agency = req.Agency
	}
	if req.Summary != "" {
		opp.Summary = req.Summary
	}
	if req.Amount != "" {
		opp.Amount = req.Amount
	}
	if req.Deadline != "" {
		if t, parseErr := time.Parse("2006-01-02", req.Deadline); parseErr == nil {
			opp.Deadline = &t
		}
	}
	if req.ContactName != "" {
		opp.ContactName = req.ContactName
	}
	if req.ContactEmail != "" {
		opp.ContactEmail = req.ContactEmail
	}
	if req.Category != "" {
		opp.Category = req.Category
	}
	if req.Status != "" {
		opp.Status = req.Status
	}

	if err := h.Opportunities.Update(r.Context(), opp); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// Delete removes an opportunity (false positive).
func (h *OpportunityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.Opportunities.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Review marks an opportunity as reviewed.
func (h *OpportunityHandler) Review(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.Opportunities.MarkReviewed(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reviewed"})
}

// RefreshStatuses updates all opportunity statuses based on deadlines.
func (h *OpportunityHandler) RefreshStatuses(w http.ResponseWriter, r *http.Request) {
	updated, err := h.Opportunities.RefreshStatuses(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "refresh failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "refreshed", "updated": updated})
}
