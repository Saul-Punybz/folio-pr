package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/middleware"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/watch"
)

// WatchlistHandler groups watchlist HTTP handlers (pantalla «Marcas»).
type WatchlistHandler struct {
	Orgs     *models.WatchlistOrgStore
	Hits     *models.WatchlistHitStore
	Articles *models.ArticleStore
}

// Redes que se pueden guardar en un perfil.
var allowedNetworks = map[string]bool{
	"instagram": true, "facebook": true, "x": true, "linkedin": true, "tiktok": true,
	"youtube": true, "bluesky": true, "threads": true, "mastodon": true,
}

func currentUser(w http.ResponseWriter, r *http.Request) *models.User {
	user := middleware.UserFromContext(r.Context())
	if user == nil {
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}
	return user
}

// ── Org endpoints ────────────────────────────────────────────────

// ListOrgs handles GET /api/watchlist/orgs.
func (h *WatchlistHandler) ListOrgs(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	// Por si la primera admin se creó después del arranque.
	watch.SeedBrandTemplates(r.Context(), h.Orgs)

	orgs, err := h.Orgs.ListByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("list watchlist orgs", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if orgs == nil {
		orgs = []models.WatchlistOrg{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"orgs": orgs, "count": len(orgs), "max_brands": models.MaxActiveBrands})
}

type orgRequest struct {
	Name            string          `json:"name"`
	Kind            string          `json:"kind"`
	Website         string          `json:"website"`
	Keywords        []string        `json:"keywords"`
	YouTubeChannels []string        `json:"youtube_channels"`
	Aliases         []string        `json:"aliases"`
	ExcludeTerms    []string        `json:"exclude_terms"`
	Hashtags        []string        `json:"hashtags"`
	Socials         []models.Social `json:"socials"`
	AlertFeeds      []string        `json:"alert_feeds"`
	ContextAliases  []string        `json:"context_aliases"`
	People          []models.Person `json:"people"`
	ToConfirm       *bool           `json:"to_confirm,omitempty"`
	Active          *bool           `json:"active,omitempty"`
}

// cleanList recorta, quita vacíos y repetidos (sin distinguir mayúsculas).
func cleanList(in []string, max int, transform func(string) string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if transform != nil {
			s = transform(s)
		}
		k := strings.ToLower(s)
		if s == "" || seen[k] || len([]rune(s)) > 120 {
			continue
		}
		seen[k] = true
		out = append(out, s)
		if len(out) >= max {
			break
		}
	}
	return out
}

func validHTTPURL(s string) bool {
	u, err := url.Parse(s)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && u.Host != ""
}

// toOrg valida la petición y la convierte. Devuelve un mensaje para la usuaria.
func (req orgRequest) toOrg() (*models.WatchlistOrg, string) {
	name := strings.TrimSpace(req.Name)
	if name == "" {
		return nil, "Falta el nombre."
	}
	if len([]rune(name)) > 120 {
		return nil, "El nombre es demasiado largo."
	}
	kind := req.Kind
	if kind == "" {
		kind = models.KindTema
	}
	if kind != models.KindMarca && kind != models.KindTema && kind != models.KindPersona {
		return nil, "Tipo no válido."
	}
	website := strings.TrimSpace(req.Website)
	if website != "" && !validHTTPURL(website) {
		return nil, "La página web tiene que empezar con https://"
	}
	socials := []models.Social{}
	for _, s := range req.Socials {
		s.Network = strings.ToLower(strings.TrimSpace(s.Network))
		s.Handle = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s.Handle), "@"))
		s.URL = strings.TrimSpace(s.URL)
		if s.Handle == "" && s.URL == "" {
			continue
		}
		if !allowedNetworks[s.Network] {
			return nil, "Red no válida: " + s.Network
		}
		if s.URL != "" && !validHTTPURL(s.URL) {
			return nil, "El enlace de " + s.Network + " tiene que empezar con https://"
		}
		socials = append(socials, s)
		if len(socials) >= 20 {
			break
		}
	}
	people := []models.Person{}
	for _, pp := range req.People {
		pp.Name = strings.TrimSpace(pp.Name)
		pp.Role = strings.TrimSpace(pp.Role)
		pp.URL = strings.TrimSpace(pp.URL)
		if pp.Name == "" {
			continue
		}
		if len([]rune(pp.Name)) < 5 || !strings.Contains(pp.Name, " ") {
			return nil, "Escribe nombre y apellido de cada persona."
		}
		if pp.URL != "" && !validHTTPURL(pp.URL) {
			return nil, "El perfil de " + pp.Name + " tiene que empezar con https://"
		}
		people = append(people, pp)
		if len(people) >= 10 {
			break
		}
	}
	feeds := cleanList(req.AlertFeeds, 10, nil)
	for _, f := range feeds {
		if !validHTTPURL(f) {
			return nil, "Cada feed de alertas tiene que empezar con https://"
		}
	}
	o := &models.WatchlistOrg{
		AlertFeeds:      feeds,
		ContextAliases:  cleanList(req.ContextAliases, 15, nil),
		People:          people,
		Name:            name,
		Kind:            kind,
		Website:         website,
		Keywords:        cleanList(req.Keywords, 20, nil),
		YouTubeChannels: cleanList(req.YouTubeChannels, 10, nil),
		Aliases:         cleanList(req.Aliases, 15, nil),
		ExcludeTerms:    cleanList(req.ExcludeTerms, 40, nil),
		Hashtags:        cleanList(req.Hashtags, 15, func(s string) string { return strings.TrimLeft(s, "#") }),
		Socials:         socials,
		Active:          true,
	}
	if req.ToConfirm != nil {
		o.ToConfirm = *req.ToConfirm
	}
	if req.Active != nil {
		o.Active = *req.Active
	}
	return o, ""
}

func brandLimitMsg() string {
	return fmt.Sprintf("Ya tienes %d marcas activas. Pausa una para añadir otra.", models.MaxActiveBrands)
}

// CreateOrg handles POST /api/watchlist/orgs.
func (h *WatchlistHandler) CreateOrg(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	var req orgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	org, msg := req.toOrg()
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	org.UserID = user.ID
	if err := h.Orgs.Create(r.Context(), org); err != nil {
		if errors.Is(err, models.ErrBrandLimit) {
			writeJSON(w, http.StatusConflict, map[string]string{"error": brandLimitMsg()})
			return
		}
		slog.Error("create watchlist org", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "could not create org"})
		return
	}
	writeJSON(w, http.StatusCreated, org)
}

// UpdateOrg handles PUT /api/watchlist/orgs/{id}. Guardar la ficha la da por
// confirmada salvo que venga to_confirm=true.
func (h *WatchlistHandler) UpdateOrg(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}
	var req orgRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	org, msg := req.toOrg()
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	org.ID = id
	org.UserID = user.ID
	if err := h.Orgs.Update(r.Context(), org); err != nil {
		switch {
		case errors.Is(err, models.ErrBrandLimit):
			writeJSON(w, http.StatusConflict, map[string]string{"error": brandLimitMsg()})
		case errors.Is(err, models.ErrOrgNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		default:
			slog.Error("update watchlist org", "id", id, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}
	writeJSON(w, http.StatusOK, org)
}

// DeleteOrg handles DELETE /api/watchlist/orgs/{id}.
func (h *WatchlistHandler) DeleteOrg(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}
	if err := h.Orgs.Delete(r.Context(), id, user.ID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// ToggleOrg handles PATCH /api/watchlist/orgs/{id}/toggle.
func (h *WatchlistHandler) ToggleOrg(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}
	var body struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if err := h.Orgs.ToggleActive(r.Context(), id, user.ID, body.Active); err != nil {
		switch {
		case errors.Is(err, models.ErrBrandLimit):
			writeJSON(w, http.StatusConflict, map[string]string{"error": brandLimitMsg()})
		case errors.Is(err, models.ErrOrgNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		default:
			slog.Error("toggle watchlist org", "id", id, "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

// MarkOrgSeen handles POST /api/watchlist/orgs/{id}/seen-all.
func (h *WatchlistHandler) MarkOrgSeen(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return
	}
	n, err := h.Hits.MarkAllSeenByOrg(r.Context(), id, user.ID)
	if err != nil {
		slog.Error("mark org hits seen", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "done", "marked": n})
}

// ── Hit endpoints ────────────────────────────────────────────────

// ListHits handles GET /api/watchlist/hits?limit=50&offset=0&org_id=...
func (h *WatchlistHandler) ListHits(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 {
		limit = 50
	}
	if limit > 300 {
		limit = 300
	}
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	var hits []models.WatchlistHit
	var err error
	if orgIDStr := r.URL.Query().Get("org_id"); orgIDStr != "" {
		orgID, parseErr := uuid.Parse(orgIDStr)
		if parseErr != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org_id"})
			return
		}
		hits, err = h.Hits.ListByOrg(r.Context(), orgID, user.ID, limit, offset)
	} else {
		hits, err = h.Hits.ListByUser(r.Context(), user.ID, limit, offset)
	}
	if err != nil {
		slog.Error("list watchlist hits", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if hits == nil {
		hits = []models.WatchlistHit{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"hits": hits, "count": len(hits)})
}

// CountUnseen handles GET /api/watchlist/hits/unseen.
func (h *WatchlistHandler) CountUnseen(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	count, err := h.Hits.CountUnseenByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("count unseen hits", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"unseen": count})
}

// MarkSeen handles POST /api/watchlist/hits/{id}/seen.
func (h *WatchlistHandler) MarkSeen(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid hit id"})
		return
	}
	if err := h.Hits.MarkSeen(r.Context(), id, user.ID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "hit not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "seen"})
}

// MarkAllSeen handles POST /api/watchlist/hits/seen-all.
func (h *WatchlistHandler) MarkAllSeen(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	count, err := h.Hits.MarkAllSeenByUser(r.Context(), user.ID)
	if err != nil {
		slog.Error("mark all hits seen", "user_id", user.ID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "done", "marked": count})
}

// DeleteHit handles DELETE /api/watchlist/hits/{id}.
func (h *WatchlistHandler) DeleteHit(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid hit id"})
		return
	}
	if err := h.Hits.Delete(r.Context(), id, user.ID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "hit not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// TriggerScan handles POST /api/watchlist/scan.
// Launches the watchlist scan in the background and returns immediately.
func (h *WatchlistHandler) TriggerScan(w http.ResponseWriter, r *http.Request) {
	go watch.RunWatchlistScan(context.Background(), watch.Deps{
		Orgs:     h.Orgs,
		Hits:     h.Hits,
		Articles: h.Articles,
	})
	writeJSON(w, http.StatusAccepted, map[string]string{
		"status":  "started",
		"message": "Buscando menciones.",
	})
}
