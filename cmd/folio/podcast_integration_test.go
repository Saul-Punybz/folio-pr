package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/podcasts"
	"github.com/Saul-Punybz/folio/internal/watch"
)

// Podcasts en Marcas: guarda solo episodios que nombran la marca, con
// source_type='podcast' (pasa el CHECK de la migración 036), deduplica por
// marca y respeta el tope de 10 por corrida.
func TestMarcasPodcasts(t *testing.T) {
	pool := testPG(t)
	ctx := context.Background()
	st := newStores(pool, t.TempDir())
	u := &models.User{Email: "podcast@folio.local", PasswordHash: "x", Role: "admin"}
	if err := st.users.Create(ctx, u); err != nil {
		t.Fatal(err)
	}
	org := &models.WatchlistOrg{UserID: u.ID, Name: "Vimenti", Kind: models.KindMarca, Active: true, Aliases: []string{"Vimenti"}}
	if err := st.watchOrgs.Create(ctx, org); err != nil {
		t.Fatal(err)
	}
	deps := watch.Deps{Orgs: st.watchOrgs, Hits: st.watchHits, Articles: st.articles}

	real := os.Getenv("FOLIO_PODCAST_REAL") != ""
	if !real {
		old := watch.PodcastSearch
		t.Cleanup(func() { watch.PodcastSearch = old })
		watch.PodcastSearch = func(_ context.Context, term string, _ int) ([]podcasts.Episode, error) {
			eps := []podcasts.Episode{{Title: "Otro tema", Show: "Radio", URL: "https://podcasts.apple.com/ruido", Description: "nada que ver"}}
			for i := 0; i < 14; i++ {
				eps = append(eps, podcasts.Episode{
					Title: "Charla con Vimenti " + string(rune('A'+i)), Show: "Voces de PR",
					URL: "https://podcasts.apple.com/ep" + string(rune('a'+i)), Released: time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC),
					Description: "Hablamos de la escuela.",
				})
			}
			return eps, nil
		}
	}
	n := watch.ScanPodcasts(ctx, *org, deps)
	t.Logf("episodios guardados para %s: %d", org.Name, n)
	if real {
		return
	}
	if n != 10 {
		t.Fatalf("tope por corrida: guardó %d, quería 10", n)
	}
	// Segunda corrida: los 10 ya existen (dedupe por marca) y entran los 4 restantes.
	if n2 := watch.ScanPodcasts(ctx, *org, deps); n2 != 4 {
		t.Fatalf("segunda corrida guardó %d, quería 4", n2)
	}
	var src, status, site string
	if err := pool.QueryRow(ctx, `SELECT source_type, archive_status, site_name FROM watchlist_hits WHERE org_id=$1 LIMIT 1`, org.ID).Scan(&src, &status, &site); err != nil {
		t.Fatal(err)
	}
	if src != "podcast" || status != "copia" || !strings.Contains(site, "Voces") {
		t.Fatalf("mención mal guardada: %s %s %s", src, status, site)
	}
}
