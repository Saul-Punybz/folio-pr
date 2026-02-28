package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/ai"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// ItemsHandler groups article/item-related HTTP handlers.
type ItemsHandler struct {
	Articles *models.ArticleStore
	Scraper  *scraper.Scraper
	AI       *ai.OllamaClient
}

// ListItems handles GET /api/items?status=inbox&limit=50&offset=0.
func (h *ItemsHandler) ListItems(w http.ResponseWriter, r *http.Request) {
	status := r.URL.Query().Get("status")
	if status == "" {
		status = "inbox"
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 200
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	articles, err := h.Articles.ListByStatus(r.Context(), status, limit, offset)
	if err != nil {
		slog.Error("list items", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}

	// Return empty array rather than null for zero results.
	if articles == nil {
		articles = []models.Article{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":  articles,
		"count":  len(articles),
		"limit":  limit,
		"offset": offset,
	})
}

// SaveItem handles POST /api/items/{id}/save.
func (h *ItemsHandler) SaveItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	if err := h.Articles.UpdateStatus(r.Context(), id, "saved"); err != nil {
		slog.Error("save item", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not save item"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "saved"})
}

// TrashItem handles POST /api/items/{id}/trash.
// Sets status to trashed and applies the default 3-month retention policy.
func (h *ItemsHandler) TrashItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	// Get the article so we can modify it.
	article, err := h.Articles.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("trash item: get", "id", id, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}

	article.Status = "trashed"
	article.EvidencePolicy = "ret_3m"
	expiresAt := time.Now().Add(90 * 24 * time.Hour) // 3 months
	article.EvidenceExpiresAt = &expiresAt

	if err := h.Articles.UpdateStatus(r.Context(), id, "trashed"); err != nil {
		slog.Error("trash item: update status", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not trash item"})
		return
	}

	// Update evidence policy and expiry separately.
	_, dbErr := h.Articles.Pool().Exec(r.Context(), `
		UPDATE articles SET evidence_policy = $1, evidence_expires_at = $2 WHERE id = $3
	`, article.EvidencePolicy, article.EvidenceExpiresAt, id)
	if dbErr != nil {
		slog.Error("trash item: update evidence", "id", id, "err", dbErr)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "trashed"})
}

// PinItem handles POST /api/items/{id}/pin.
// Toggles the pinned state of an article.
func (h *ItemsHandler) PinItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	article, err := h.Articles.GetByID(r.Context(), id)
	if err != nil {
		slog.Error("pin item: get", "id", id, "err", err)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
		return
	}

	newPinned := !article.Pinned
	if err := h.Articles.SetPinned(r.Context(), id, newPinned); err != nil {
		slog.Error("pin item: set", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not pin item"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"pinned": newPinned})
}

// UndoItem handles POST /api/items/{id}/undo.
// Restores an article to its previous status (inbox).
func (h *ItemsHandler) UndoItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	var body struct {
		PreviousStatus string `json:"previous_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		body.PreviousStatus = "inbox"
	}
	if body.PreviousStatus == "" {
		body.PreviousStatus = "inbox"
	}

	if err := h.Articles.UpdateStatus(r.Context(), id, body.PreviousStatus); err != nil {
		slog.Error("undo item", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not undo item"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": body.PreviousStatus})
}

type updateRetentionRequest struct {
	Policy string `json:"policy"`
}

// UpdateRetention handles PUT /api/items/{id}/retention.
// Body: { "policy": "ret_6m" | "ret_12m" | "keep" }
func (h *ItemsHandler) UpdateRetention(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	var req updateRetentionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	validPolicies := map[string]bool{"ret_6m": true, "ret_12m": true, "keep": true}
	if !validPolicies[req.Policy] {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "policy must be ret_6m, ret_12m, or keep"})
		return
	}

	if err := h.Articles.UpdateRetention(r.Context(), id, req.Policy); err != nil {
		slog.Error("update retention", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not update retention"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated", "policy": req.Policy})
}

type collectRequest struct {
	URL     string `json:"url"`
	Title   string `json:"title,omitempty"`
	Region  string `json:"region,omitempty"`
	Snippet string `json:"snippet,omitempty"`
}

// CollectItem handles POST /api/collect.
// Creates a new article from a manually provided URL.
func (h *ItemsHandler) CollectItem(w http.ResponseWriter, r *http.Request) {
	var req collectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if req.URL == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "url is required"})
		return
	}

	region := req.Region
	if region == "" {
		region = "PR"
	}
	title := req.Title
	if title == "" {
		title = req.URL // placeholder, worker will extract real title later
	}

	article := &models.Article{
		Title:          title,
		Source:         "manual",
		URL:            req.URL,
		CanonicalURL:   req.URL,
		Region:         region,
		Status:         "inbox",
		Summary:        req.Snippet,
		EvidencePolicy: "ret_3m",
	}

	if err := h.Articles.Create(r.Context(), article); err != nil {
		slog.Error("collect item", "url", req.URL, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not collect item"})
		return
	}

	// Scrape + enrich in background so the article has full data.
	if h.Scraper != nil && h.AI != nil {
		go h.enrichCollectedArticle(article.ID, article.URL)
	}

	writeJSON(w, http.StatusCreated, article)
}

// enrichCollectedArticle scrapes the URL for content, image, then runs AI
// summarization, classification, and embedding to fill in all missing data.
func (h *ItemsHandler) enrichCollectedArticle(id uuid.UUID, articleURL string) {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	slog.Info("collect: enriching", "id", id, "url", articleURL)

	// Step 1: Extract og:image (always try, independent of text scraping).
	imageURL := h.Scraper.ExtractImageURL(ctx, articleURL)
	if imageURL != "" {
		if err := h.Articles.SetImageURL(ctx, id, imageURL); err != nil {
			slog.Warn("collect: set image", "id", id, "err", err)
		}
	}

	// Step 2: Try multiple selector strategies to extract text.
	selectorSets := []scraper.SourceSelectors{
		{TitleSelector: "h1", BodySelector: "article p"},
		{TitleSelector: "h1", BodySelector: ".article-body p, .entry-content p, .post-content p"},
		{TitleSelector: "h1", BodySelector: "main p"},
		{TitleSelector: "h1", BodySelector: ".content p, #content p, .story-body p, .nota-body p"},
		{TitleSelector: "h1", BodySelector: "p"},
	}

	var scraped *scraper.ScrapedArticle
	for _, sel := range selectorSets {
		result, err := h.Scraper.ScrapeArticle(ctx, articleURL, sel)
		if err != nil {
			slog.Warn("collect: scrape attempt failed", "id", id, "selector", sel.BodySelector, "err", err)
			break // Site is unreachable, no point trying more selectors.
		}
		if result != nil && len(result.CleanText) > 100 {
			scraped = result
			slog.Info("collect: scraped text", "id", id, "selector", sel.BodySelector, "len", len(result.CleanText))
			break
		}
	}

	if scraped == nil || len(scraped.CleanText) < 50 {
		slog.Warn("collect: no text extracted, skipping AI enrichment", "id", id, "url", articleURL)
		return
	}

	// Step 3: Update title and clean_text.
	cleanText := scraped.CleanText
	title := scraped.Title

	var pubAt *time.Time
	if !scraped.PublishedAt.IsZero() {
		pubAt = &scraped.PublishedAt
	}

	_, err := h.Articles.Pool().Exec(ctx, `
		UPDATE articles
		SET clean_text = CASE WHEN clean_text = '' OR clean_text IS NULL THEN $1 ELSE clean_text END,
		    title = CASE WHEN $2 != '' THEN $2 ELSE title END,
		    published_at = COALESCE(published_at, $3)
		WHERE id = $4
	`, cleanText, title, pubAt, id)
	if err != nil {
		slog.Warn("collect: update content", "id", id, "err", err)
	}

	// Step 4: AI enrichment — summarize, classify, embed.
	text := cleanText
	if len(text) > 8000 {
		text = text[:8000]
	}

	summary, err := h.AI.Summarize(ctx, text)
	if err != nil {
		slog.Warn("collect: summarize", "id", id, "err", err)
		return
	}

	tags, err := h.AI.Classify(ctx, text)
	if err != nil {
		slog.Warn("collect: classify", "id", id, "err", err)
		tags = nil
	}

	embedding, err := h.AI.Embed(ctx, text)
	if err != nil {
		slog.Warn("collect: embed", "id", id, "err", err)
		embedding = nil
	}

	// Only overwrite summary if we got a better one from AI (don't clobber snippet).
	if summary != "" {
		if err := h.Articles.UpdateEnrichment(ctx, id, summary, tags, embedding); err != nil {
			slog.Warn("collect: update enrichment", "id", id, "err", err)
		}
	}

	slog.Info("collect: enrichment complete", "id", id)
}
