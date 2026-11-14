package handlers

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// AdminHandler groups admin-only HTTP handlers.
type AdminHandler struct {
	Articles     *models.ArticleStore
	Sources      *models.SourceStore
	Fingerprints *models.FingerprintStore
	Scraper      *scraper.Scraper
}

// TriggerIngest handles POST /api/admin/ingest.
// Manually triggers the RSS/scraper ingestion cycle (rule-based, no AI).
func (h *AdminHandler) TriggerIngest(w http.ResponseWriter, r *http.Request) {
	if h.Sources == nil || h.Fingerprints == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "ingestion not configured"})
		return
	}

	stores := scraper.Stores{
		Articles:     h.Articles,
		Sources:      h.Sources,
		Fingerprints: h.Fingerprints,
	}

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Hour)
		defer cancel()
		n := scraper.RunIngestion(ctx, stores, h.Scraper)
		slog.Info("admin: manual ingestion finished", "articles", n)
	}()

	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Ingestion started in background. New articles will appear shortly.",
	})
}
