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

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// ItemsHandler groups article/item-related HTTP handlers.
type ItemsHandler struct {
	Articles *models.ArticleStore
	Scraper  *scraper.Scraper
}

// ListItems handles GET /api/items?status=inbox&limit=50&offset=0&date=2026-03-10.
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

	// Optional date filter: "today", "yesterday", "YYYY-MM-DD", or "YYYY-MM-DD,YYYY-MM-DD" range.
	var dateFrom, dateTo *time.Time
	dateParam := r.URL.Query().Get("date")
	if dateParam != "" {
		now := time.Now()
		switch dateParam {
		case "today":
			d := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			dateFrom = &d
		case "yesterday":
			d := time.Date(now.Year(), now.Month(), now.Day()-1, 0, 0, 0, 0, now.Location())
			d2 := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			dateFrom = &d
			dateTo = &d2
		case "week":
			d := now.AddDate(0, 0, -7)
			dateFrom = &d
		default:
			// Try parsing as single date YYYY-MM-DD in local timezone
			if t, err := time.ParseInLocation("2006-01-02", dateParam, now.Location()); err == nil {
				dateFrom = &t
				end := t.AddDate(0, 0, 1)
				dateTo = &end
			}
		}
	}

	articles, err := h.Articles.ListByStatus(r.Context(), status, limit, offset, dateFrom, dateTo)
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

// TrashItem handles POST /api/items/{id}/trash («Borrar»).
// Oculta la noticia al instante (status 'trashed'); una tarea la elimina de
// verdad pasada una hora y bloquea su URL. Mientras tanto «Deshacer» la devuelve.
func (h *ItemsHandler) TrashItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	n, err := h.Articles.Trash(r.Context(), []uuid.UUID{id})
	if err != nil {
		slog.Error("trash item", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not trash item"})
		return
	}
	if n == 0 {
		// Ya estaba borrada, o no existe.
		if _, gerr := h.Articles.GetByID(r.Context(), id); gerr != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "trashed"})
}

const maxBulkIDs = 500

type bulkIDsRequest struct {
	IDs            []string `json:"ids"`
	PreviousStatus string   `json:"previous_status,omitempty"`
}

// parseBulkIDs lee {"ids":[...]}: de 1 a 500 UUID válidos, sin repetidos.
func parseBulkIDs(w http.ResponseWriter, r *http.Request) ([]uuid.UUID, string, bool) {
	var req bulkIDsRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return nil, "", false
	}
	if len(req.IDs) == 0 || len(req.IDs) > maxBulkIDs {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "ids must have between 1 and 500 items"})
		return nil, "", false
	}
	seen := make(map[uuid.UUID]bool, len(req.IDs))
	ids := make([]uuid.UUID, 0, len(req.IDs))
	for _, raw := range req.IDs {
		id, err := uuid.Parse(raw)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
			return nil, "", false
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids, req.PreviousStatus, true
}

// TrashMany handles POST /api/items/trash with {"ids":[...]} (máx. 500).
func (h *ItemsHandler) TrashMany(w http.ResponseWriter, r *http.Request) {
	ids, _, ok := parseBulkIDs(w, r)
	if !ok {
		return
	}
	n, err := h.Articles.Trash(r.Context(), ids)
	if err != nil {
		slog.Error("trash many", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not trash items"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "trashed", "count": n})
}

// UndoMany handles POST /api/items/undo with {"ids":[...]}: devuelve lo
// borrado a su estado anterior mientras la purga no lo haya eliminado.
func (h *ItemsHandler) UndoMany(w http.ResponseWriter, r *http.Request) {
	ids, prev, ok := parseBulkIDs(w, r)
	if !ok {
		return
	}
	n, err := h.Articles.Restore(r.Context(), ids, prev)
	if err != nil {
		slog.Error("undo many", "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not undo items"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "restored", "count": n})
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
// Deshace un «Borrar» (vuelve al estado anterior) o, si la noticia no estaba
// borrada, la pone en previous_status (inbox, saved o archived).
func (h *ItemsHandler) UndoItem(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	var body struct {
		PreviousStatus string `json:"previous_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.PreviousStatus == "" {
		body.PreviousStatus = "inbox"
	}
	switch body.PreviousStatus {
	case "inbox", "saved", "archived":
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid previous_status"})
		return
	}

	n, err := h.Articles.Restore(r.Context(), []uuid.UUID{id}, body.PreviousStatus)
	if err != nil {
		slog.Error("undo item", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not undo item"})
		return
	}
	if n == 0 {
		// No estaba borrada (p. ej. deshacer un «Guardar»): se pone en el estado pedido.
		if err := h.Articles.UpdateStatus(r.Context(), id, body.PreviousStatus); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "item not found"})
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": body.PreviousStatus})
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
		Summary:        scraper.RuleSummary(req.Snippet),
		EvidencePolicy: "keep",
	}

	if err := h.Articles.Create(r.Context(), article); err != nil {
		slog.Error("collect item", "url", req.URL, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not collect item"})
		return
	}

	// Scrape + enrich in background so the article has full data.
	if h.Scraper != nil {
		go h.enrichCollectedArticle(article.ID, article.URL)
	}

	writeJSON(w, http.StatusCreated, article)
}

// enrichCollectedArticle scrapes the URL for content and image, and fills the
// summary by rules (meta description or first paragraph). No AI.
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
		slog.Warn("collect: no text extracted, skipping enrichment", "id", id, "url", articleURL)
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

	// Step 4: resumen por reglas; no pisa el snippet que mandó el usuario.
	if summary := scraper.RuleSummary(scraper.MetaDescription(scraped.RawHTML), scraper.FirstParagraph(cleanText)); summary != "" {
		if err := h.Articles.SetSummaryIfEmpty(ctx, id, summary); err != nil {
			slog.Warn("collect: update summary", "id", id, "err", err)
		}
	}

	slog.Info("collect: enrichment complete", "id", id)
}
