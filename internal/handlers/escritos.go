package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/generator"
	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
)

// EscritosHandler groups SEO article generation HTTP handlers.
type EscritosHandler struct {
	Escritos *models.EscritoStore
	Sources  *models.EscritoSourceStore
	Articles *models.ArticleStore
	AI       *ai.OllamaClient
}

type createEscritoRequest struct {
	Topic      string   `json:"topic"`
	ArticleIDs []string `json:"article_ids"`
}

// CreateEscrito handles POST /api/escritos.
func (h *EscritosHandler) CreateEscrito(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req createEscritoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	topic := strings.TrimSpace(req.Topic)
	if topic == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "topic is required"})
		return
	}

	escrito := &models.Escrito{
		UserID: user.ID,
		Topic:  topic,
	}

	if err := h.Escritos.Create(r.Context(), escrito); err != nil {
		slog.Error("create escrito", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create escrito"})
		return
	}

	// Parse article IDs
	var articleIDs []uuid.UUID
	for _, idStr := range req.ArticleIDs {
		if id, err := uuid.Parse(idStr); err == nil {
			articleIDs = append(articleIDs, id)
		}
	}

	// Auto-trigger generation immediately
	go generator.RunGeneration(context.Background(), generator.Deps{
		Escritos: h.Escritos,
		Sources:  h.Sources,
		Articles: h.Articles,
		AI:       h.AI,
	}, escrito.ID, articleIDs)

	writeJSON(w, http.StatusCreated, escrito)
}

// ListEscritos handles GET /api/escritos.
func (h *EscritosHandler) ListEscritos(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	escritos, err := h.Escritos.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list escritos", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if escritos == nil {
		escritos = []models.Escrito{}
	}

	writeJSON(w, http.StatusOK, map[string]any{"escritos": escritos, "count": len(escritos)})
}

// GetEscrito handles GET /api/escritos/{id}.
func (h *EscritosHandler) GetEscrito(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid escrito id"})
		return
	}

	escrito, err := h.Escritos.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "escrito not found"})
		return
	}
	if escrito.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "escrito not found"})
		return
	}

	sources, _ := h.Sources.ListByEscrito(r.Context(), escrito.ID)
	if sources == nil {
		sources = []models.EscritoSource{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"escrito": escrito,
		"sources": sources,
	})
}

// UpdateEscrito handles PUT /api/escritos/{id}.
func (h *EscritosHandler) UpdateEscrito(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid escrito id"})
		return
	}

	escrito, err := h.Escritos.GetByID(r.Context(), id)
	if err != nil || escrito.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "escrito not found"})
		return
	}

	var req struct {
		Title         *string  `json:"title"`
		MetaDesc      *string  `json:"meta_description"`
		Content       *string  `json:"content"`
		Keywords      []string `json:"keywords"`
		Hashtags      []string `json:"hashtags"`
		PublishStatus *string  `json:"publish_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Handle publish_status update separately
	if req.PublishStatus != nil {
		ps := *req.PublishStatus
		if ps != "draft" && ps != "reviewing" && ps != "published" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid publish_status"})
			return
		}
		if err := h.Escritos.UpdatePublishStatus(r.Context(), id, ps); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update status"})
			return
		}
	}

	// Handle content edits
	title := escrito.Title
	metaDesc := escrito.MetaDescription
	content := escrito.Content
	keywords := escrito.Keywords
	hashtags := escrito.Hashtags

	if req.Title != nil {
		title = *req.Title
	}
	if req.MetaDesc != nil {
		metaDesc = *req.MetaDesc
	}
	if req.Content != nil {
		content = *req.Content
	}
	if req.Keywords != nil {
		keywords = req.Keywords
	}
	if req.Hashtags != nil {
		hashtags = req.Hashtags
	}

	if req.Title != nil || req.MetaDesc != nil || req.Content != nil || req.Keywords != nil || req.Hashtags != nil {
		if err := h.Escritos.UpdateEdited(r.Context(), id, title, metaDesc, content, keywords, hashtags); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update escrito"})
			return
		}
	}

	// Re-fetch updated
	updated, err := h.Escritos.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not re-fetch escrito"})
		return
	}

	writeJSON(w, http.StatusOK, updated)
}

// RegenerateEscrito handles POST /api/escritos/{id}/regenerate.
func (h *EscritosHandler) RegenerateEscrito(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid escrito id"})
		return
	}

	escrito, err := h.Escritos.GetByID(r.Context(), id)
	if err != nil || escrito.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "escrito not found"})
		return
	}

	// Clear old sources first, then reset status
	_ = h.Sources.DeleteByEscrito(r.Context(), id)

	if err := h.Escritos.UpdateStatus(r.Context(), id, "queued", 0); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not reset escrito"})
		return
	}

	// Re-generate from scratch (nil articleIDs = discover fresh sources)
	go generator.RunGeneration(context.Background(), generator.Deps{
		Escritos: h.Escritos,
		Sources:  h.Sources,
		Articles: h.Articles,
		AI:       h.AI,
	}, id, nil)

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "regenerating",
		"message": "Regenerando articulo desde cero.",
	})
}

// ImproveEscrito handles POST /api/escritos/{id}/improve.
// Accepts {"instructions": "..."} and rewrites the content using AI.
func (h *EscritosHandler) ImproveEscrito(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid escrito id"})
		return
	}

	escrito, err := h.Escritos.GetByID(r.Context(), id)
	if err != nil || escrito.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "escrito not found"})
		return
	}

	if escrito.Content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no content to improve"})
		return
	}

	var req struct {
		Instructions string `json:"instructions"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || strings.TrimSpace(req.Instructions) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "instructions required"})
		return
	}

	// Set status to improving
	_ = h.Escritos.UpdateStatus(r.Context(), id, "improving", 3)

	go func() {
		ctx := context.Background()
		improved, err := generator.ImproveContent(ctx, h.AI, escrito, req.Instructions)
		if err != nil {
			slog.Error("improve escrito failed", "id", id, "err", err)
			_ = h.Escritos.SetFailed(ctx, id, "Mejora fallida: "+err.Error())
			return
		}

		// Save improved content
		wordCount := generator.CountWords(improved)
		if err := h.Escritos.UpdateContent(ctx, id, escrito.Title, escrito.Slug,
			escrito.MetaDescription, improved, escrito.Keywords, escrito.Hashtags,
			wordCount, nil); err != nil {
			slog.Error("save improved content", "err", err)
			_ = h.Escritos.SetFailed(ctx, id, "Error guardando mejoras")
			return
		}

		// Recalc SEO
		primaryKW := ""
		if len(escrito.Keywords) > 0 {
			primaryKW = escrito.Keywords[0]
		}
		score := generator.ScoreArticle(improved, escrito.Title, escrito.MetaDescription, primaryKW)
		scoreJSON, _ := json.Marshal(score)
		_ = h.Escritos.UpdateSEOScore(ctx, id, scoreJSON)
		_ = h.Escritos.SetFinished(ctx, id)

		slog.Info("improve escrito complete", "id", id, "words", wordCount)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "improving",
		"message": "Mejorando articulo con tus instrucciones.",
	})
}

// RecalcSEO handles POST /api/escritos/{id}/seo-score.
func (h *EscritosHandler) RecalcSEO(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid escrito id"})
		return
	}

	escrito, err := h.Escritos.GetByID(r.Context(), id)
	if err != nil || escrito.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "escrito not found"})
		return
	}

	primaryKW := ""
	if len(escrito.Keywords) > 0 {
		primaryKW = escrito.Keywords[0]
	}

	score := generator.ScoreArticle(escrito.Content, escrito.Title, escrito.MetaDescription, primaryKW)
	scoreJSON, err := json.Marshal(score)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not calculate score"})
		return
	}

	if err := h.Escritos.UpdateSEOScore(r.Context(), id, scoreJSON); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save score"})
		return
	}

	writeJSON(w, http.StatusOK, score)
}

// DeleteEscrito handles DELETE /api/escritos/{id}.
func (h *EscritosHandler) DeleteEscrito(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid escrito id"})
		return
	}

	escrito, err := h.Escritos.GetByID(r.Context(), id)
	if err != nil || escrito.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "escrito not found"})
		return
	}

	if err := h.Escritos.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not delete escrito"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ExportEscrito handles POST /api/escritos/{id}/export.
func (h *EscritosHandler) ExportEscrito(w http.ResponseWriter, r *http.Request) {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid escrito id"})
		return
	}

	escrito, err := h.Escritos.GetByID(r.Context(), id)
	if err != nil || escrito.UserID != user.ID {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "escrito not found"})
		return
	}

	var req struct {
		Format string `json:"format"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.Format = "markdown"
	}
	if req.Format == "" {
		req.Format = "markdown"
	}

	sources, _ := h.Sources.ListByEscrito(r.Context(), escrito.ID)

	switch req.Format {
	case "html":
		html := exportHTML(escrito, sources)
		writeJSON(w, http.StatusOK, map[string]string{"format": "html", "content": html})
	default:
		md := exportMarkdown(escrito, sources)
		writeJSON(w, http.StatusOK, map[string]string{"format": "markdown", "content": md})
	}
}

func exportMarkdown(e *models.Escrito, sources []models.EscritoSource) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("# %s\n\n", e.Title))
	b.WriteString(fmt.Sprintf("*%s*\n\n", e.MetaDescription))
	if len(e.Keywords) > 0 {
		b.WriteString("**Keywords:** " + strings.Join(e.Keywords, ", ") + "\n\n")
	}
	b.WriteString("---\n\n")
	b.WriteString(e.Content)
	b.WriteString("\n\n---\n\n")
	if len(e.Hashtags) > 0 {
		b.WriteString(strings.Join(e.Hashtags, " ") + "\n\n")
	}
	if len(sources) > 0 {
		b.WriteString("## Fuentes\n\n")
		for _, s := range sources {
			b.WriteString(fmt.Sprintf("- [%s](%s) — %s\n", s.ArticleTitle, s.ArticleURL, s.ArticleSource))
		}
	}
	return b.String()
}

func exportHTML(e *models.Escrito, sources []models.EscritoSource) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("<article>\n<h1>%s</h1>\n", e.Title))
	b.WriteString(fmt.Sprintf("<meta name=\"description\" content=\"%s\">\n", e.MetaDescription))
	if len(e.Keywords) > 0 {
		b.WriteString(fmt.Sprintf("<meta name=\"keywords\" content=\"%s\">\n", strings.Join(e.Keywords, ", ")))
	}
	// Convert markdown to basic HTML
	content := e.Content
	content = strings.ReplaceAll(content, "\n## ", "\n<h2>")
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "<h2>") {
			line = line + "</h2>"
		} else if strings.TrimSpace(line) == "" {
			line = ""
		} else if !strings.HasPrefix(line, "<") {
			line = "<p>" + line + "</p>"
		}
		lines[i] = line
	}
	b.WriteString(strings.Join(lines, "\n"))
	if len(e.Hashtags) > 0 {
		b.WriteString("\n<div class=\"hashtags\">")
		for _, h := range e.Hashtags {
			b.WriteString(fmt.Sprintf("<span>%s</span> ", h))
		}
		b.WriteString("</div>\n")
	}
	if len(sources) > 0 {
		b.WriteString("<section class=\"sources\">\n<h2>Fuentes</h2>\n<ul>\n")
		for _, s := range sources {
			b.WriteString(fmt.Sprintf("<li><a href=\"%s\">%s</a> — %s</li>\n", s.ArticleURL, s.ArticleTitle, s.ArticleSource))
		}
		b.WriteString("</ul>\n</section>\n")
	}
	b.WriteString("</article>")
	return b.String()
}
