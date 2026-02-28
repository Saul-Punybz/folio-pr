package handlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
)

// SearchHandler groups search-related HTTP handlers.
type SearchHandler struct {
	Articles *models.ArticleStore
}

// Search handles GET /api/search?q=&from=&to=&region=&status=&tag=&limit=&offset=.
func (h *SearchHandler) Search(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	fromStr := r.URL.Query().Get("from")
	toStr := r.URL.Query().Get("to")
	region := r.URL.Query().Get("region")
	status := r.URL.Query().Get("status")
	tag := r.URL.Query().Get("tag")
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 {
		limit = 50
	}

	var from, to time.Time
	if fromStr != "" {
		parsed, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			// Try date-only format.
			parsed, err = time.Parse("2006-01-02", fromStr)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'from' date, use RFC3339 or YYYY-MM-DD"})
				return
			}
		}
		from = parsed
	}
	if toStr != "" {
		parsed, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			parsed, err = time.Parse("2006-01-02", toStr)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid 'to' date, use RFC3339 or YYYY-MM-DD"})
				return
			}
		}
		to = parsed
	}

	articles, err := h.Articles.Search(r.Context(), q, from, to, region, status, tag, limit, offset)
	if err != nil {
		slog.Error("search", "query", q, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "search failed"})
		return
	}

	if articles == nil {
		articles = []models.Article{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": articles,
		"count":   len(articles),
		"query":   q,
		"limit":   limit,
		"offset":  offset,
	})
}

// Similar handles GET /api/items/{id}/similar?limit=5.
// Returns articles similar to the given article based on embedding cosine distance.
func (h *SearchHandler) Similar(w http.ResponseWriter, r *http.Request) {
	idStr := chi.URLParam(r, "id")
	id, err := uuid.Parse(idStr)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}

	articles, err := h.Articles.SimilarArticles(r.Context(), id, limit)
	if err != nil {
		slog.Error("similar articles", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not find similar articles"})
		return
	}

	if articles == nil {
		articles = []models.Article{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"results": articles,
		"count":   len(articles),
	})
}
