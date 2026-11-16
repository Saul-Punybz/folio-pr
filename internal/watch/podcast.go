package watch

import (
	"context"
	"log/slog"
	"strings"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/podcasts"
)

const (
	// maxPodcastResults: tope de menciones nuevas de podcast por marca por corrida.
	maxPodcastResults = 10
	// maxPodcastTerms: búsquedas por marca (Apple limita a ~20 por minuto; el
	// cliente de podcasts ya espacia 3 s entre peticiones).
	maxPodcastTerms = 3
	// podcastFetch: episodios pedidos por búsqueda (la búsqueda es difusa; luego se filtra).
	podcastFetch = 50
)

// PodcastSearch es la búsqueda de episodios (las pruebas la reemplazan).
var PodcastSearch = podcasts.SearchEpisodes

// podcastTerms: nombre y alias de la marca que sirven de frase exacta. Se
// descartan los cortos y los que necesitan contexto («PSI»), y dominios o handles.
func podcastTerms(org models.WatchlistOrg) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range matchTerms(org) {
		k := fold(t)
		if seen[k] || weakTerm(t) || isContextAlias(t, org) || strings.ContainsAny(t, "._@") {
			continue
		}
		seen[k] = true
		out = append(out, t)
		if len(out) >= maxPodcastTerms {
			break
		}
	}
	return out
}

// ScanPodcasts busca episodios de podcast que nombran la marca (frase exacta,
// Apple Podcasts) y guarda hasta maxPodcastResults menciones nuevas.
func ScanPodcasts(ctx context.Context, org models.WatchlistOrg, deps Deps) int {
	if !org.IsBrand() {
		return 0
	}
	saved := 0
	for _, term := range podcastTerms(org) {
		if saved >= maxPodcastResults || ctx.Err() != nil {
			break
		}
		eps, err := PodcastSearch(ctx, term, podcastFetch)
		if err != nil {
			slog.Warn("watchlist/podcasts: search", "org", org.Name, "term", term, "err", err)
			if err == podcasts.ErrRateLimited {
				break
			}
			continue
		}
		for _, ep := range podcasts.FilterMentions(eps, term) {
			if saved >= maxPodcastResults {
				break
			}
			if Excluded(ep.Title+" "+ep.Show+" "+ep.Description, org) {
				continue
			}
			if saveHitWith(ctx, deps, org, "podcast", match{}, ep.Title, ep.URL, ep.Description, ep.Released, withPodcast(ep)) {
				saved++
			}
		}
	}
	if saved > 0 {
		slog.Info("watchlist/podcasts: done", "org", org.Name, "new_hits", saved)
	}
	return saved
}

// withPodcast guarda además la descripción del episodio y el programa: ya
// vienen de la API, así que la mención nace archivada (sin bajar la página).
func withPodcast(ep podcasts.Episode) func(*models.WatchlistHit) {
	return func(h *models.WatchlistHit) {
		h.ArchiveStatus = models.ArchiveCopia
		h.ContentText = ep.Description
		h.Summary = truncateStr(strings.Join(strings.Fields(ep.Description), " "), 600)
		h.SiteName = strings.TrimSpace(ep.Show) // el programa: va en la cita APA
	}
}
