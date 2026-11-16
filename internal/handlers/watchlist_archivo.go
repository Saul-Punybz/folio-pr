package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/archivo"
	"github.com/Saul-Punybz/folio/internal/models"
	"github.com/Saul-Punybz/folio/internal/watch"
)

// Archivo permanente de menciones: copia del contenido, cita APA, resumen,
// bibliografía y exportación.

// archivoItem es una mención con lo que la pantalla necesita para la ficha.
type archivoItem struct {
	models.WatchlistHit
	OriginalURL string       `json:"original_url"`
	Medio       string       `json:"medio"`
	Fecha       string       `json:"fecha"` // 2025-10-19 (hora de Puerto Rico) o ""
	APA         archivo.Cita `json:"apa"`
}

var puertoRico = time.FixedZone("AST", -4*3600)

// metaFor arma los metadatos de la cita de una mención.
func metaFor(h models.WatchlistHit) archivo.Meta {
	u := h.CanonicalURL
	if u == "" {
		u = h.ResolvedURL
	}
	if u == "" {
		u = h.URL
	}
	site := h.SiteName
	if site == "" {
		site = archivo.SiteFromURL(h.URL)
	}
	text := ""
	if archivo.KindFor(h.SourceType) == archivo.KindPost {
		text = h.Snippet
		if text == "" {
			_, text = splitPrefix(h.Title)
		}
	}
	return archivo.Meta{
		Kind: archivo.KindFor(h.SourceType), Title: h.Title, Site: site, Authors: h.Authors,
		Published: h.PublishedAt, URL: u, Text: text, Platform: archivo.PlatformFor(h.SourceType),
	}
}

func splitPrefix(title string) (string, string) {
	if i := strings.Index(title, ": "); i > 0 && i <= 80 {
		return title[:i], title[i+2:]
	}
	return "", title
}

func buildItem(h models.WatchlistHit) archivoItem {
	it := archivoItem{WatchlistHit: h, APA: archivo.APA(metaFor(h))}
	it.OriginalURL = h.ResolvedURL
	if it.OriginalURL == "" {
		it.OriginalURL = h.URL
	}
	it.Medio = h.SiteName
	if it.Medio == "" {
		it.Medio = archivo.SiteFromURL(h.URL)
	}
	if h.PublishedAt != nil {
		it.Fecha = h.PublishedAt.In(puertoRico).Format("2006-01-02")
	}
	return it
}

func parseDay(s string, plusDay bool) *time.Time {
	t, err := time.ParseInLocation("2006-01-02", strings.TrimSpace(s), puertoRico)
	if err != nil {
		return nil
	}
	if plusDay {
		t = t.AddDate(0, 0, 1)
	}
	return &t
}

func filterFromQuery(r *http.Request) models.ArchiveFilter {
	q := r.URL.Query()
	f := models.ArchiveFilter{
		Query:  q.Get("q"),
		From:   parseDay(q.Get("desde"), false),
		To:     parseDay(q.Get("hasta"), true),
		Source: q.Get("fuente"),
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	return f
}

func orgIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid org id"})
		return uuid.Nil, false
	}
	return id, true
}

// ListArchivo handles GET /api/watchlist/orgs/{id}/archivo
// ?q=texto&desde=YYYY-MM-DD&hasta=YYYY-MM-DD&fuente=noticias&limit=&offset=
// Devuelve TODAS las menciones de la marca (nuevas y vistas), más recientes
// primero, con la cita de cada una, la bibliografía (orden alfabético APA) de
// lo que cumple el filtro y los conteos de la copia.
func (h *WatchlistHandler) ListArchivo(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	orgID, ok := orgIDParam(w, r)
	if !ok {
		return
	}
	org, err := h.Orgs.GetForUser(r.Context(), orgID, user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		return
	}
	hits, total, err := h.Hits.ListArchive(r.Context(), orgID, user.ID, filterFromQuery(r))
	if err != nil {
		slog.Error("list archivo", "org", orgID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	stats, err := h.Hits.ArchiveStatsByOrg(r.Context(), orgID, user.ID)
	if err != nil {
		slog.Error("archivo stats", "org", orgID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	items := make([]archivoItem, 0, len(hits))
	biblio := make([]archivo.Entrada, 0, len(hits))
	for _, hit := range hits {
		it := buildItem(hit)
		items = append(items, it)
		biblio = append(biblio, archivo.Entrada{Cita: it.APA, Published: hit.PublishedAt})
	}
	archivo.Ordenar(biblio)
	writeJSON(w, http.StatusOK, map[string]any{
		"org":   map[string]any{"id": org.ID, "name": org.Name},
		"total": total,
		"stats": stats,
		"hits":  items,
		"bibliografia": map[string]string{
			"plain":    archivo.Bibliografia(biblio, false),
			"markdown": archivo.Bibliografia(biblio, true),
		},
	})
}

// ExportArchivo handles GET /api/watchlist/orgs/{id}/archivo/exportar
// ?formato=md|csv + los mismos filtros. Baja un archivo.
func (h *WatchlistHandler) ExportArchivo(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	orgID, ok := orgIDParam(w, r)
	if !ok {
		return
	}
	org, err := h.Orgs.GetForUser(r.Context(), orgID, user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "org not found"})
		return
	}
	f := filterFromQuery(r)
	f.Limit = 5000
	hits, _, err := h.Hits.ListArchive(r.Context(), orgID, user.ID, f)
	if err != nil {
		slog.Error("export archivo", "org", orgID, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	filas := make([]archivo.Fila, 0, len(hits))
	for _, hit := range hits {
		it := buildItem(hit)
		filas = append(filas, archivo.Fila{
			Titulo: it.Title, Medio: it.Medio, Autores: strings.Join(it.Authors, "; "), Fecha: it.Fecha,
			URL: it.OriginalURL, Resumen: it.Summary, Cita: it.APA, Copia: it.WaybackURL,
		})
	}
	base := "archivo-" + slug(org.Name) + "-" + time.Now().In(puertoRico).Format("2006-01-02")
	switch r.URL.Query().Get("formato") {
	case "csv":
		b, err := archivo.CSV(filas)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.csv"`, base))
		_, _ = w.Write(b)
	case "md":
		w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.md"`, base))
		_, _ = w.Write([]byte(archivo.Markdown("Archivo de "+org.Name, filas)))
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "formato debe ser md o csv"})
	}
}

func slug(s string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(s) {
		switch {
		case r >= 'a' && r <= 'z' || r >= '0' && r <= '9':
			b.WriteRune(r)
			dash = false
		case r == 'á':
			b.WriteByte('a')
		case r == 'é':
			b.WriteByte('e')
		case r == 'í':
			b.WriteByte('i')
		case r == 'ó':
			b.WriteByte('o')
		case r == 'ú', r == 'ü':
			b.WriteByte('u')
		case r == 'ñ':
			b.WriteByte('n')
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func hitIDParam(w http.ResponseWriter, r *http.Request) (uuid.UUID, bool) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid hit id"})
		return uuid.Nil, false
	}
	return id, true
}

// GetCopia handles GET /api/watchlist/hits/{id}/copia: la mención y el texto
// archivado dentro de Folio («Ver copia guardada»).
func (h *WatchlistHandler) GetCopia(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, ok := hitIDParam(w, r)
	if !ok {
		return
	}
	hit, err := h.Hits.GetForUser(r.Context(), id, user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "hit not found"})
		return
	}
	text, err := h.Hits.GetText(r.Context(), id, user.ID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "hit not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"hit": buildItem(*hit), "text": text})
}

type metaRequest struct {
	Title        string   `json:"title"`
	SiteName     string   `json:"site_name"`
	Authors      []string `json:"authors"`
	PublishedAt  string   `json:"published_at"` // YYYY-MM-DD o vacío
	Summary      string   `json:"summary"`
	CanonicalURL string   `json:"canonical_url"`
}

// UpdateMeta handles PATCH /api/watchlist/hits/{id}/meta: la usuaria corrige
// título, medio, autores, fecha, resumen o URL canónica; la cita se regenera.
func (h *WatchlistHandler) UpdateMeta(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, ok := hitIDParam(w, r)
	if !ok {
		return
	}
	var req metaRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	title := strings.Join(strings.Fields(req.Title), " ")
	if title == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Falta el título."})
		return
	}
	var published *time.Time
	if d := strings.TrimSpace(req.PublishedAt); d != "" {
		t, err := time.ParseInLocation("2006-01-02", d, puertoRico)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "La fecha tiene que ser AAAA-MM-DD."})
			return
		}
		t = t.Add(12 * time.Hour).UTC() // mediodía: la fecha no cambia de día al convertir
		published = &t
	}
	canon := strings.TrimSpace(req.CanonicalURL)
	if canon != "" && !validHTTPURL(canon) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "La URL tiene que empezar con https://"})
		return
	}
	var authors []string
	for _, a := range req.Authors {
		if a = strings.Join(strings.Fields(a), " "); a != "" && len(authors) < 20 {
			authors = append(authors, a)
		}
	}
	err := h.Hits.UpdateMeta(r.Context(), id, user.ID, models.MetaEdit{
		Title: truncRunes(title, 300), SiteName: truncRunes(strings.Join(strings.Fields(req.SiteName), " "), 120),
		Authors: authors, PublishedAt: published, Summary: truncRunes(strings.TrimSpace(req.Summary), 1500), CanonicalURL: canon,
	})
	if errors.Is(err, models.ErrHitNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "hit not found"})
		return
	}
	if err != nil {
		slog.Error("update meta", "id", id, "err", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	hit, err := h.Hits.GetForUser(r.Context(), id, user.ID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, buildItem(*hit))
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

var archiveRunning atomic.Bool

// RetryCopia handles POST /api/watchlist/hits/{id}/reintentar: vuelve a
// intentar la copia de una mención «sin copia» o «parcial».
func (h *WatchlistHandler) RetryCopia(w http.ResponseWriter, r *http.Request) {
	user := currentUser(w, r)
	if user == nil {
		return
	}
	id, ok := hitIDParam(w, r)
	if !ok {
		return
	}
	if err := h.Hits.RetryArchive(r.Context(), id, user.ID); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "hit not found"})
		return
	}
	h.startArchive(5)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "started"})
}

// RunArchivo handles POST /api/watchlist/archivo/run: archiva las pendientes
// ahora (en segundo plano).
func (h *WatchlistHandler) RunArchivo(w http.ResponseWriter, r *http.Request) {
	if currentUser(w, r) == nil {
		return
	}
	started := h.startArchive(watch.ArchiveRunCap)
	status := "started"
	if !started {
		status = "already_running"
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"status": status})
}

func (h *WatchlistHandler) startArchive(n int) bool {
	if !archiveRunning.CompareAndSwap(false, true) {
		return false
	}
	go func() {
		defer archiveRunning.Store(false)
		watch.RunArchive(contextBackground(), h.Hits, n)
	}()
	return true
}
