package handlers

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/storage"
)

// ExportHandler groups export-related HTTP handlers.
type ExportHandler struct {
	Articles *models.ArticleStore
	Notes    *models.NoteStore
	Storage  *storage.Client
}

// ExportArticle handles GET /api/items/{id}/export.
// Returns a ZIP file containing the article data, notes, and evidence.
func (h *ExportHandler) ExportArticle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid article id"})
		return
	}

	article, err := h.Articles.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "article not found"})
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="folio-export-%s.zip"`, id))

	zw := zip.NewWriter(w)
	defer zw.Close()

	if err := h.writeArticleToZip(zw, "", article, r); err != nil {
		slog.Error("export article", "id", id, "err", err)
		return
	}
}

type bulkExportRequest struct {
	IDs []string `json:"ids"`
}

// ExportBulk handles POST /api/export.
// Body: { "ids": ["uuid1", "uuid2", ...] }
// Returns a ZIP with folders for each article.
func (h *ExportHandler) ExportBulk(w http.ResponseWriter, r *http.Request) {
	var req bulkExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	if len(req.IDs) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "at least one id is required"})
		return
	}

	if len(req.IDs) > 100 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "maximum 100 articles per export"})
		return
	}

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="folio-bulk-export.zip"`)

	zw := zip.NewWriter(w)
	defer zw.Close()

	for _, idStr := range req.IDs {
		id, err := uuid.Parse(idStr)
		if err != nil {
			slog.Warn("export bulk: invalid id", "id", idStr)
			continue
		}

		article, err := h.Articles.GetByID(r.Context(), id)
		if err != nil {
			slog.Warn("export bulk: article not found", "id", id)
			continue
		}

		prefix := fmt.Sprintf("%s/", id)
		if err := h.writeArticleToZip(zw, prefix, article, r); err != nil {
			slog.Error("export bulk: write article", "id", id, "err", err)
			continue
		}
	}
}

// writeArticleToZip writes a single article's data into the zip writer.
func (h *ExportHandler) writeArticleToZip(zw *zip.Writer, prefix string, article *models.Article, r *http.Request) error {
	// Article metadata.
	articleJSON, err := json.MarshalIndent(article, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal article: %w", err)
	}

	aw, err := zw.Create(prefix + "article.json")
	if err != nil {
		return fmt.Errorf("create article.json: %w", err)
	}
	if _, err := aw.Write(articleJSON); err != nil {
		return fmt.Errorf("write article.json: %w", err)
	}

	// Notes.
	notes, err := h.Notes.ListByArticle(r.Context(), article.ID)
	if err != nil {
		slog.Warn("export: notes fetch failed", "article_id", article.ID, "err", err)
	} else {
		if notes == nil {
			notes = []models.Note{}
		}
		notesJSON, err := json.MarshalIndent(notes, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal notes: %w", err)
		}
		nw, err := zw.Create(prefix + "notes.json")
		if err != nil {
			return fmt.Errorf("create notes.json: %w", err)
		}
		if _, err := nw.Write(notesJSON); err != nil {
			return fmt.Errorf("write notes.json: %w", err)
		}
	}

	// Evidence from S3 (if configured and available).
	if h.Storage != nil && h.Storage.Configured() {
		evidence, err := h.Storage.GetEvidence(r.Context(), article.ID)
		if err == nil {
			// Raw HTML.
			if len(evidence.RawHTML) > 0 {
				rw, err := zw.Create(prefix + "evidence/raw.html")
				if err == nil {
					rw.Write(evidence.RawHTML)
				}
			}
			// Extracted text.
			if len(evidence.Extracted) > 0 {
				ew, err := zw.Create(prefix + "evidence/extracted.json")
				if err == nil {
					ew.Write(evidence.Extracted)
				}
			}
		} else {
			slog.Debug("export: no evidence available", "article_id", article.ID)
		}
	}

	return nil
}
