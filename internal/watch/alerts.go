package watch

import (
	"context"
	"log/slog"

	"github.com/Saul-Punybz/folio/internal/models"
)

// ScanAlerts lee los feeds RSS de alertas que la usuaria pegó en la ficha
// (Google Alerts → «Entregar a: Feed RSS»). La alerta ya filtró por su
// búsqueda; aquí solo se aplican las exclusiones de la marca y se sacan los
// enlaces de rastreo de Google.
func ScanAlerts(ctx context.Context, org models.WatchlistOrg, m Matcher, deps Deps) int {
	saved := 0
	for _, feedURL := range org.AlertFeeds {
		if saved >= maxResultsPerAgent || ctx.Err() != nil {
			break
		}
		items, err := parseFeed(ctx, feedURL)
		if err != nil {
			slog.Warn("watchlist/alertas: parse feed", "org", org.Name, "err", err)
			continue
		}
		for _, it := range items {
			if saved >= maxResultsPerAgent {
				break
			}
			text := stripHTML(it.Title) + " " + stripHTML(it.Description)
			if it.Link == "" || Excluded(text, org) {
				continue
			}
			mt, _ := matchItem(text, org, m)
			if saveHit(ctx, deps, org, "alertas", mt, stripHTML(it.Title), it.Link, stripHTML(it.Description), it.Published) {
				saved++
			}
		}
	}
	if saved > 0 {
		slog.Info("watchlist/alertas: done", "org", org.Name, "new_hits", saved)
	}
	return saved
}
