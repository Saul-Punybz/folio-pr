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
	Topic          string   `json:"topic"`
	ArticleIDs     []string `json:"article_ids"`
	OutputFormat   string   `json:"output_format"`
	WritingStyle   string   `json:"writing_style"`
	TargetAudience string   `json:"target_audience"`
	CitationStyle  string   `json:"citation_style"`
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
		UserID:         user.ID,
		Topic:          topic,
		OutputFormat:   normalizeEscritoFormat(req.OutputFormat),
		WritingStyle:   normalizeEscritoStyle(req.WritingStyle),
		TargetAudience: normalizeEscritoAudience(req.TargetAudience),
		CitationStyle:  normalizeEscritoCitation(req.CitationStyle),
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
		Title          *string  `json:"title"`
		MetaDesc       *string  `json:"meta_description"`
		Content        *string  `json:"content"`
		Keywords       []string `json:"keywords"`
		Hashtags       []string `json:"hashtags"`
		PublishStatus  *string  `json:"publish_status"`
		OutputFormat   *string  `json:"output_format"`
		WritingStyle   *string  `json:"writing_style"`
		TargetAudience *string  `json:"target_audience"`
		CitationStyle  *string  `json:"citation_style"`
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

	outputFormat := escrito.OutputFormat
	writingStyle := escrito.WritingStyle
	targetAudience := escrito.TargetAudience
	citationStyle := escrito.CitationStyle
	if req.OutputFormat != nil {
		outputFormat = normalizeEscritoFormat(*req.OutputFormat)
	}
	if req.WritingStyle != nil {
		writingStyle = normalizeEscritoStyle(*req.WritingStyle)
	}
	if req.TargetAudience != nil {
		targetAudience = normalizeEscritoAudience(*req.TargetAudience)
	}
	if req.CitationStyle != nil {
		citationStyle = normalizeEscritoCitation(*req.CitationStyle)
	}
	if req.OutputFormat != nil || req.WritingStyle != nil || req.TargetAudience != nil || req.CitationStyle != nil {
		if err := h.Escritos.UpdateProfile(r.Context(), id, outputFormat, writingStyle, targetAudience, citationStyle); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update escrito profile"})
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

func normalizeEscritoFormat(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "auto":
		return "auto"
	case "article", "newsletter", "report", "memo", "letter", "email", "op_ed":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "auto"
	}
}

func normalizeEscritoStyle(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "auto":
		return "auto"
	case "academic", "journalistic", "executive", "institutional", "persuasive", "newsletter":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "auto"
	}
}

func normalizeEscritoAudience(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "general":
		return "general"
	case "academic", "executive", "public_sector", "community", "donor":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "general"
	}
}

func normalizeEscritoCitation(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "apa":
		return "apa"
	case "mla", "chicago", "inline":
		return strings.ToLower(strings.TrimSpace(v))
	default:
		return "apa"
	}
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
		if err := h.finalizeImprovedEscrito(ctx, id, escrito, improved); err != nil {
			slog.Error("save improved content", "id", id, "err", err)
			_ = h.Escritos.SetFailed(ctx, id, "Error guardando mejoras")
			return
		}
		slog.Info("improve escrito complete", "id", id, "words", generator.CountWords(improved))
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "improving",
		"message": "Mejorando articulo con tus instrucciones.",
	})
}

// ExpandEscrito handles POST /api/escritos/{id}/expand.
// It expands the article with more depth and length, then refreshes metadata and SEO.
func (h *EscritosHandler) ExpandEscrito(w http.ResponseWriter, r *http.Request) {
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
	if strings.TrimSpace(escrito.Content) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no content to expand"})
		return
	}

	var req struct {
		Instructions string `json:"instructions"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)
	_ = h.Escritos.UpdateStatus(r.Context(), id, "improving", 4)

	go func() {
		ctx := context.Background()
		instructions := buildExpansionInstructions(escrito, req.Instructions)
		expanded, err := generator.ExpandContent(ctx, h.AI, escrito, instructions)
		if err != nil {
			slog.Error("expand escrito failed", "id", id, "err", err)
			_ = h.Escritos.SetFailed(ctx, id, "Expansion fallida: "+err.Error())
			return
		}
		if err := h.finalizeImprovedEscrito(ctx, id, escrito, expanded); err != nil {
			slog.Error("save expanded content", "id", id, "err", err)
			_ = h.Escritos.SetFailed(ctx, id, "Error guardando expansion")
			return
		}
		slog.Info("expand escrito complete", "id", id, "words", generator.CountWords(expanded))
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "improving",
		"message": "Expandiendo articulo para aumentar profundidad y longitud.",
	})
}

// OptimizeSEO handles POST /api/escritos/{id}/seo-optimize.
// It rewrites content with SEO-focused instructions, regenerates metadata,
// recalculates score, and stores the improved article.
func (h *EscritosHandler) OptimizeSEO(w http.ResponseWriter, r *http.Request) {
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
	if strings.TrimSpace(escrito.Content) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no content to optimize"})
		return
	}

	var req struct {
		Instructions string `json:"instructions"`
	}
	_ = json.NewDecoder(r.Body).Decode(&req)

	primaryKW := ""
	if len(escrito.Keywords) > 0 {
		primaryKW = escrito.Keywords[0]
	}
	score := generator.ScoreArticle(escrito.Content, escrito.Title, escrito.MetaDescription, primaryKW)
	scoreJSON, _ := json.Marshal(score)
	_ = h.Escritos.UpdateSEOScore(r.Context(), id, scoreJSON)
	_ = h.Escritos.UpdateStatus(r.Context(), id, "improving", 4)

	go func() {
		ctx := context.Background()
		instructions := buildSEOImproveInstructions(escrito, score, req.Instructions)

		improved, err := generator.ImproveContent(ctx, h.AI, escrito, instructions)
		if err != nil {
			slog.Error("seo optimize failed", "id", id, "err", err)
			_ = h.Escritos.SetFailed(ctx, id, "Optimizacion SEO fallida: "+err.Error())
			return
		}

		if err := h.finalizeImprovedEscrito(ctx, id, escrito, improved); err != nil {
			slog.Error("save seo optimized content", "id", id, "err", err)
			_ = h.Escritos.SetFailed(ctx, id, "Error guardando optimizacion SEO")
			return
		}
		slog.Info("seo optimize complete", "id", id, "words", generator.CountWords(improved))
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "improving",
		"message": "Optimizando articulo para SEO y claridad editorial.",
	})
}

func buildSEOImproveInstructions(escrito *models.Escrito, score generator.SEOScore, custom string) string {
	var b strings.Builder
	b.WriteString("Optimiza este articulo para SEO sin volverlo artificial.\n")
	if len(escrito.Keywords) > 0 {
		b.WriteString("Keyword principal: " + escrito.Keywords[0] + "\n")
	}
	b.WriteString("Objetivos:\n")
	b.WriteString("- mejorar claridad, estructura, escaneabilidad y transiciones\n")
	b.WriteString("- fortalecer introduccion y conclusion\n")
	b.WriteString("- mejorar densidad natural de keyword sin keyword stuffing\n")
	b.WriteString("- reforzar headings H2 y subtitulos para legibilidad\n")
	b.WriteString("- conservar y fortalecer citas, datos, hyperlinks y referencias\n")
	b.WriteString("- eliminar repeticiones, relleno y frases debiles\n")
	if len(score.Warnings) > 0 {
		b.WriteString("Problemas detectados actualmente:\n")
		for _, warning := range score.Warnings {
			b.WriteString("- " + warning + "\n")
		}
	}
	custom = strings.TrimSpace(custom)
	if custom != "" {
		b.WriteString("Instrucciones adicionales del usuario:\n")
		b.WriteString(custom)
		b.WriteString("\n")
	}
	b.WriteString("Tambien regenera implicitamente un texto mas publicable y orientado a conversion informativa.")
	return b.String()
}

func buildExpansionInstructions(escrito *models.Escrito, custom string) string {
	var b strings.Builder
	b.WriteString("Expande el articulo para hacerlo mas sustancial y util.\n")
	b.WriteString("Objetivos:\n")
	b.WriteString("- aumentar claramente la longitud total\n")
	b.WriteString("- agregar contexto, antecedentes y consecuencias\n")
	b.WriteString("- fortalecer secciones flojas con mas datos, ejemplos y explicaciones\n")
	b.WriteString("- agregar subsecciones nuevas si hacen falta\n")
	b.WriteString("- mejorar FAQ, conclusion y referencias cuando convenga\n")
	if strings.TrimSpace(custom) != "" {
		b.WriteString("Instrucciones adicionales del usuario:\n")
		b.WriteString(strings.TrimSpace(custom))
		b.WriteString("\n")
	}
	return b.String()
}

func (h *EscritosHandler) finalizeImprovedEscrito(ctx context.Context, id uuid.UUID, escrito *models.Escrito, content string) error {
	title, slug, metaDesc, keywords, hashtags := generator.RegenerateMetadata(ctx, h.AI, escrito, content)
	wordCount := generator.CountWords(content)
	if err := h.Escritos.UpdateContent(ctx, id, title, slug, metaDesc, content, keywords, hashtags, wordCount, nil); err != nil {
		return err
	}

	primaryKW := ""
	if len(keywords) > 0 {
		primaryKW = keywords[0]
	}
	score := generator.ScoreArticle(content, title, metaDesc, primaryKW)
	scoreJSON, _ := json.Marshal(score)
	_ = h.Escritos.UpdateSEOScore(ctx, id, scoreJSON)
	_ = h.Escritos.SetFinished(ctx, id)
	return nil
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
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("title: \"%s\"\n", escapeFrontmatter(e.Title)))
	b.WriteString(fmt.Sprintf("slug: \"%s\"\n", escapeFrontmatter(e.Slug)))
	b.WriteString(fmt.Sprintf("topic: \"%s\"\n", escapeFrontmatter(e.Topic)))
	b.WriteString(fmt.Sprintf("description: \"%s\"\n", escapeFrontmatter(e.MetaDescription)))
	b.WriteString(fmt.Sprintf("output_format: \"%s\"\n", e.OutputFormat))
	b.WriteString(fmt.Sprintf("writing_style: \"%s\"\n", e.WritingStyle))
	b.WriteString(fmt.Sprintf("target_audience: \"%s\"\n", e.TargetAudience))
	b.WriteString(fmt.Sprintf("citation_style: \"%s\"\n", e.CitationStyle))
	b.WriteString(fmt.Sprintf("publish_status: \"%s\"\n", e.PublishStatus))
	b.WriteString(fmt.Sprintf("word_count: %d\n", e.WordCount))
	b.WriteString(fmt.Sprintf("created_at: \"%s\"\n", e.CreatedAt.Format("2006-01-02T15:04:05Z07:00")))
	b.WriteString(fmt.Sprintf("updated_at: \"%s\"\n", e.UpdatedAt.Format("2006-01-02T15:04:05Z07:00")))
	if len(e.Keywords) > 0 {
		b.WriteString("keywords:\n")
		for _, kw := range e.Keywords {
			b.WriteString(fmt.Sprintf("  - \"%s\"\n", escapeFrontmatter(kw)))
		}
	}
	if len(e.Hashtags) > 0 {
		b.WriteString("hashtags:\n")
		for _, tag := range e.Hashtags {
			b.WriteString(fmt.Sprintf("  - \"%s\"\n", escapeFrontmatter(tag)))
		}
	}
	b.WriteString("---\n\n")

	b.WriteString(fmt.Sprintf("# %s\n\n", e.Title))
	if strings.TrimSpace(e.MetaDescription) != "" {
		b.WriteString(fmt.Sprintf("*%s*\n\n", e.MetaDescription))
	}
	b.WriteString(strings.TrimSpace(e.Content))
	b.WriteString("\n\n---\n\n")
	if len(e.Hashtags) > 0 {
		b.WriteString(strings.Join(e.Hashtags, " ") + "\n\n")
	}
	entries := sourceEntries(sources)
	if len(entries) > 0 {
		b.WriteString("## Fuentes Verificadas\n\n")
		for _, entry := range entries {
			if entry.URL != "" {
				b.WriteString(fmt.Sprintf("- %s. [%s](%s)\n", entry.Source, entry.Title, entry.URL))
				continue
			}
			b.WriteString(fmt.Sprintf("- %s. %s\n", entry.Source, entry.Title))
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
	entries := sourceEntries(sources)
	if len(entries) > 0 {
		b.WriteString("<section class=\"sources\">\n<h2>Fuentes Verificadas</h2>\n<ul>\n")
		for _, entry := range entries {
			if entry.URL != "" {
				b.WriteString(fmt.Sprintf("<li><a href=\"%s\">%s</a> — %s</li>\n", entry.URL, entry.Title, entry.Source))
				continue
			}
			b.WriteString(fmt.Sprintf("<li>%s — %s</li>\n", entry.Title, entry.Source))
		}
		b.WriteString("</ul>\n</section>\n")
	}
	b.WriteString("</article>")
	return b.String()
}

type exportSourceEntry struct {
	Title  string
	URL    string
	Source string
}

func sourceEntries(sources []models.EscritoSource) []exportSourceEntry {
	seen := make(map[string]struct{}, len(sources))
	entries := make([]exportSourceEntry, 0, len(sources))
	for _, s := range sources {
		entry := exportSourceEntry{
			Title:  strings.TrimSpace(s.ArticleTitle),
			URL:    strings.TrimSpace(s.ArticleURL),
			Source: strings.TrimSpace(s.ArticleSource),
		}
		if strings.TrimSpace(s.WebTitle) != "" {
			entry.Title = strings.TrimSpace(s.WebTitle)
		}
		if strings.TrimSpace(s.WebURL) != "" {
			entry.URL = strings.TrimSpace(s.WebURL)
		}
		if strings.TrimSpace(s.WebDomain) != "" {
			entry.Source = strings.TrimSpace(s.WebDomain)
		}
		if entry.Title == "" && entry.URL == "" {
			continue
		}
		if entry.Title == "" {
			entry.Title = entry.URL
		}
		if entry.Source == "" {
			entry.Source = "Fuente externa"
		}
		key := entry.URL + "|" + entry.Title
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		entries = append(entries, entry)
	}
	return entries
}

func escapeFrontmatter(v string) string {
	return strings.ReplaceAll(strings.TrimSpace(v), "\"", "\\\"")
}
