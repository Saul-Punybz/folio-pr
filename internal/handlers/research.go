package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/research"
)

// ResearchHandler groups deep research HTTP handlers.
type ResearchHandler struct {
	Projects *models.ResearchProjectStore
	Findings *models.ResearchFindingStore
	Articles *models.ArticleStore
	AI       *ai.OllamaClient
}

type createResearchRequest struct {
	Topic    string   `json:"topic"`
	Keywords []string `json:"keywords"`
}

// CreateProject handles POST /api/research.
func (h *ResearchHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createResearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "topic is required"})
		return
	}

	if req.Keywords == nil {
		req.Keywords = []string{}
	}

	project := &models.ResearchProject{
		UserID:   user.ID,
		Topic:    topic,
		Keywords: req.Keywords,
	}

	if err := h.Projects.Create(r.Context(), project); err != nil {
		slog.Error("create research project", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create project"})
		return
	}

	// Auto-trigger research immediately (don't wait for cron)
	go research.RunProject(context.Background(), research.Deps{
		Projects: h.Projects,
		Findings: h.Findings,
		Articles: h.Articles,
		AI:       h.AI,
	}, project.ID)

	writeJSON(w, http.StatusCreated, project)
}

// ListProjects handles GET /api/research.
func (h *ResearchHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	projects, err := h.Projects.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list research projects", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if projects == nil {
		projects = []models.ResearchProject{}
	}

	// Get finding counts for each project
	type projectWithCount struct {
		models.ResearchProject
		FindingsCount int `json:"findings_count"`
	}
	var result []projectWithCount
	for _, p := range projects {
		count, _ := h.Findings.CountByProject(r.Context(), p.ID)
		result = append(result, projectWithCount{ResearchProject: p, FindingsCount: count})
	}

	writeJSON(w, http.StatusOK, map[string]any{"projects": result, "count": len(result)})
}

// GetProject handles GET /api/research/{id}.
func (h *ResearchHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}

	project, err := h.Projects.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}
	if project.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	count, _ := h.Findings.CountByProject(r.Context(), project.ID)

	writeJSON(w, http.StatusOK, map[string]any{
		"project":        project,
		"findings_count": count,
	})
}

// GetFindings handles GET /api/research/{id}/findings?source_type=...&limit=50&offset=0.
func (h *ResearchHandler) GetFindings(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}

	// Verify ownership
	project, err := h.Projects.GetByID(r.Context(), id)
	if err != nil || project.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	sourceType := r.URL.Query().Get("source_type")

	var findings []models.ResearchFinding

	if sourceType != "" {
		findings, err = h.Findings.ListByProjectAndSource(r.Context(), id, sourceType, limit, offset)
	} else {
		findings, err = h.Findings.ListByProject(r.Context(), id, limit, offset)
	}

	if err != nil {
		slog.Error("list research findings", "project_id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if findings == nil {
		findings = []models.ResearchFinding{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"findings": findings, "count": len(findings)})
}

// StopProject handles POST /api/research/{id}/stop.
func (h *ResearchHandler) StopProject(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}

	// Verify ownership
	project, err := h.Projects.GetByID(r.Context(), id)
	if err != nil || project.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if err := h.Projects.Cancel(r.Context(), id); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// DeleteProject handles DELETE /api/research/{id}.
func (h *ResearchHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}

	// Verify ownership
	project, err := h.Projects.GetByID(r.Context(), id)
	if err != nil || project.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if err := h.Projects.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete project"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// TriggerResearch handles POST /api/research/{id}/run — runs research in background.
func (h *ResearchHandler) TriggerResearch(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid project id"})
		return
	}

	project, err := h.Projects.GetByID(r.Context(), id)
	if err != nil || project.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "project not found"})
		return
	}

	if project.Status != "queued" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "project is not in queued state"})
		return
	}

	go research.RunProject(context.Background(), research.Deps{
		Projects: h.Projects,
		Findings: h.Findings,
		Articles: h.Articles,
		AI:       h.AI,
	}, project.ID)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Investigacion iniciada. Los resultados apareceran en minutos.",
	})
}
