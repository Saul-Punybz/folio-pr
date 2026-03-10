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

	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/models"
)

// CrawlerHandler handles FolioBot crawler API endpoints.
type CrawlerHandler struct {
	Domains  *models.CrawlDomainStore
	Queue    *models.CrawlQueueStore
	Pages    *models.CrawledPageStore
	Links    *models.CrawlLinkStore
	Runs     *models.CrawlRunStore
	Entities *models.EntityStore
	PageEnts *models.PageEntityStore
	Rels     *models.EntityRelationshipStore
	CrawlDeps crawler.Deps
}

// Stats returns aggregate crawler statistics.
func (h *CrawlerHandler) Stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	totalPages, _ := h.Pages.TotalCount(ctx)
	totalLinks, _ := h.Links.TotalCount(ctx)
	queueCounts, _ := h.Queue.CountsByStatus(ctx)
	domains, _ := h.Domains.List(ctx)
	latestRun, _ := h.Runs.GetLatest(ctx)

	writeJSON(w, http.StatusOK, map[string]any{
		"total_pages":  totalPages,
		"total_links":  totalLinks,
		"total_domains": len(domains),
		"queue":        queueCounts,
		"latest_run":   latestRun,
	})
}

// ListRuns returns recent crawl runs.
func (h *CrawlerHandler) ListRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 20
	}

	runs, err := h.Runs.ListRecent(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"runs": runs, "count": len(runs)})
}

// QueueStats returns queue counts by status.
func (h *CrawlerHandler) QueueStats(w http.ResponseWriter, r *http.Request) {
	counts, err := h.Queue.CountsByStatus(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"queue": counts})
}

// ListPages returns paginated crawled pages, optionally filtered by domain.
func (h *CrawlerHandler) ListPages(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	domainIDStr := r.URL.Query().Get("domain_id")

	var pages []models.CrawledPage
	var err error

	if domainIDStr != "" {
		domainID, parseErr := uuid.Parse(domainIDStr)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid domain_id"})
			return
		}
		pages, err = h.Pages.ListByDomain(r.Context(), domainID, limit, offset)
	} else {
		pages, err = h.Pages.ListAll(r.Context(), limit, offset)
	}

	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages, "count": len(pages)})
}

// SearchPages performs full-text search on crawled pages.
func (h *CrawlerHandler) SearchPages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing q parameter"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	pages, err := h.Pages.SearchFTS(r.Context(), query, limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages, "count": len(pages)})
}

// ListChangedPages returns recently changed pages.
func (h *CrawlerHandler) ListChangedPages(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	pages, err := h.Pages.ListChanged(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pages": pages, "count": len(pages)})
}

// GetPage returns a single page with its outbound and inbound links.
func (h *CrawlerHandler) GetPage(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	page, err := h.Pages.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "page not found"})
		return
	}

	outLinks, _ := h.Links.ListOutbound(r.Context(), id)
	inLinks, _ := h.Links.ListInbound(r.Context(), id)
	entities, _ := h.PageEnts.EntitiesForPage(r.Context(), id)

	writeJSON(w, http.StatusOK, map[string]any{
		"page":      page,
		"links_out": outLinks,
		"links_in":  inLinks,
		"entities":  entities,
	})
}

// GetGraph returns the knowledge graph (entities + edges).
func (h *CrawlerHandler) GetGraph(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	nodes, edges, err := h.Rels.GetGraph(r.Context(), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"nodes": nodes, "edges": edges})
}

// GetEntityRelations returns relationships for a single entity.
func (h *CrawlerHandler) GetEntityRelations(w http.ResponseWriter, r *http.Request) {
	entityID, err := uuid.Parse(chi.URLParam(r, "entityId"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid entityId"})
		return
	}

	rels, entities, err := h.Rels.GetEntityRelations(r.Context(), entityID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"relationships": rels, "entities": entities})
}

// ListDomains returns all crawler domains.
func (h *CrawlerHandler) ListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := h.Domains.List(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"domains": domains, "count": len(domains)})
}

// CreateDomain adds a new crawler domain.
func (h *CrawlerHandler) CreateDomain(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Domain       string `json:"domain"`
		Label        string `json:"label"`
		Category     string `json:"category"`
		MaxDepth     int    `json:"max_depth"`
		RecrawlHours int    `json:"recrawl_hours"`
		Priority     int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.Domain == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "domain is required"})
		return
	}
	if req.MaxDepth <= 0 {
		req.MaxDepth = 3
	}
	if req.RecrawlHours <= 0 {
		req.RecrawlHours = 168
	}
	if req.Priority <= 0 || req.Priority > 10 {
		req.Priority = 5
	}
	if req.Category == "" {
		req.Category = "other"
	}

	d := &models.CrawlDomain{
		Domain:       req.Domain,
		Label:        req.Label,
		Category:     req.Category,
		MaxDepth:     req.MaxDepth,
		RecrawlHours: req.RecrawlHours,
		Priority:     req.Priority,
	}
	if err := h.Domains.Create(r.Context(), d); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to create domain"})
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

// UpdateDomain updates a crawler domain.
func (h *CrawlerHandler) UpdateDomain(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req struct {
		Label        string `json:"label"`
		Category     string `json:"category"`
		MaxDepth     int    `json:"max_depth"`
		RecrawlHours int    `json:"recrawl_hours"`
		Priority     int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	d := &models.CrawlDomain{
		ID:           id,
		Label:        req.Label,
		Category:     req.Category,
		MaxDepth:     req.MaxDepth,
		RecrawlHours: req.RecrawlHours,
		Priority:     req.Priority,
	}
	if err := h.Domains.Update(r.Context(), d); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// ToggleDomain toggles a domain's active status.
func (h *CrawlerHandler) ToggleDomain(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var req struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if err := h.Domains.ToggleActive(r.Context(), id, req.Active); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

// DeleteDomain removes a crawler domain.
func (h *CrawlerHandler) DeleteDomain(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	if err := h.Domains.Delete(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "domain not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// TriggerCrawl manually triggers a crawl run in a background goroutine.
func (h *CrawlerHandler) TriggerCrawl(w http.ResponseWriter, r *http.Request) {
	go func() {
		slog.Info("crawler: manual trigger started")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
		defer cancel()
		crawler.RunCrawl(ctx, h.CrawlDeps, 100)
		slog.Info("crawler: manual trigger finished")
	}()
	writeJSON(w, http.StatusOK, map[string]string{"status": "triggered", "message": "Crawl started in background"})
}
