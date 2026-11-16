package main

import (
	"context"
	"log/slog"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/scraper"
)

// runNoticiasMaintenance: lo del Inbox con más de 7 días pasa a Archivo, y lo
// borrado hace más de 1 h se elimina de verdad (con sus notas) dejando su URL
// y su huella de contenido bloqueadas. Lo guardado y lo archivado no se toca.
func runNoticiasMaintenance(ctx context.Context, arts *models.ArticleStore) {
	archived, err := arts.ArchiveOld(ctx)
	if err != nil {
		slog.Error("noticias: archivar", "err", err)
	}
	purged, err := arts.PurgeTrashed(ctx, models.TrashGrace, scraper.HashURL, scraper.HashContent)
	if err != nil {
		slog.Error("noticias: purgar", "err", err)
	}
	slog.Info("noticias: mantenimiento", "archivadas", archived, "eliminadas", purged)
}
