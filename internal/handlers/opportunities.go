package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Saul-Punybz/folio/internal/extract"
	"github.com/Saul-Punybz/folio/internal/models"
)

// OpportunityHandler handles grant/RFP opportunity API endpoints.
type OpportunityHandler struct {
	Opportunities *models.OpportunityStore
	// DataDir: carpeta de datos (los PDF están en DataDir/documentos).
	DataDir string
}

func boolParam(r *http.Request, name string) *bool {
	switch strings.ToLower(r.URL.Query().Get(name)) {
	case "true", "1":
		b := true
		return &b
	case "false", "0":
		b := false
		return &b
	}
	return nil
}

// List returns paginated opportunities with optional filters
// (status, category, level, domain_id, reviewed, sort, limit, offset).
func (h *OpportunityHandler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f := models.ListFilter{
		Status:   q.Get("status"),
		Category: q.Get("category"),
		Level:    q.Get("level"),
		Reviewed: boolParam(r, "reviewed"),
		Sort:     q.Get("sort"),
	}
	f.Limit, _ = strconv.Atoi(q.Get("limit"))
	f.Offset, _ = strconv.Atoi(q.Get("offset"))
	if did := q.Get("domain_id"); did != "" {
		if parsed, err := uuid.Parse(did); err == nil {
			f.DomainID = &parsed
		}
	}

	opps, err := h.Opportunities.List(r.Context(), f)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if opps == nil {
		opps = []models.Opportunity{}
	}
	total, _ := h.Opportunities.Count(r.Context(), models.ListFilter{
		Status: f.Status, Category: f.Category, Level: f.Level, Reviewed: f.Reviewed, DomainID: f.DomainID,
	})
	writeJSON(w, http.StatusOK, map[string]any{"opportunities": opps, "count": len(opps), "total": total})
}

// Stats returns opportunity counts by status, plus pending (posibles fondos
// sin revisar y no cerrados) and closing_soon (confirmados que cierran en ≤14 días).
func (h *OpportunityHandler) Stats(w http.ResponseWriter, r *http.Request) {
	statusCounts, _ := h.Opportunities.CountByStatus(r.Context())
	total, _ := h.Opportunities.TotalCount(r.Context())
	pending, closingSoon, _ := h.Opportunities.PendingAndClosingSoon(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"total":        total,
		"by_status":    statusCounts,
		"pending":      pending,
		"closing_soon": closingSoon,
	})
}

// Search performs full-text search on opportunities (respeta reviewed).
func (h *OpportunityHandler) Search(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing q parameter"})
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	opps, err := h.Opportunities.SearchFTS(r.Context(), query, boolParam(r, "reviewed"), limit)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	if opps == nil {
		opps = []models.Opportunity{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"opportunities": opps, "count": len(opps)})
}

// Get returns a single opportunity by ID.
func (h *OpportunityHandler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	opp, err := h.Opportunities.GetByID(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, opp)
}

var validCategories = map[string]bool{"rfp": true, "grant": true, "funding": true, "proposal": true,
	"contract": true, "fellowship": true, "scholarship": true, "other": true}
var validStatuses = map[string]bool{"open": true, "closing_soon": true, "closed": true, "unknown": true}

// Update modifica un fondo (corrección manual). Solo cambia las llaves que
// vienen en el cuerpo, y una llave con "" (o null) vacía el campo: así se
// distingue «no enviado» de «vacío». deadline: "YYYY-MM-DD", o ""/null para
// quitarla. Los campos corregidos quedan protegidos del siguiente crawl.
func (h *OpportunityHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}

	var raw map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	p, msg := parsePatch(raw)
	if msg != "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": msg})
		return
	}
	if err := h.Opportunities.ApplyPatch(r.Context(), id, p); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
			return
		}
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "update failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

// parsePatch lee el cuerpo del PUT. Devuelve un mensaje de error si algo no vale.
func parsePatch(raw map[string]json.RawMessage) (models.Patch, string) {
	var p models.Patch
	str := func(key string) (*string, string) {
		v, ok := raw[key]
		if !ok {
			return nil, ""
		}
		if bytes.Equal(bytes.TrimSpace(v), []byte("null")) {
			s := ""
			return &s, ""
		}
		var s string
		if err := json.Unmarshal(v, &s); err != nil {
			return nil, key + " debe ser texto"
		}
		return &s, ""
	}
	fields := []struct {
		key string
		dst **string
	}{
		{"title", &p.Title}, {"agency", &p.Agency}, {"summary", &p.Summary}, {"amount", &p.Amount},
		{"contact_name", &p.ContactName}, {"contact_email", &p.ContactEmail}, {"contact_phone", &p.ContactPhone},
		{"reference", &p.Reference}, {"category", &p.Category}, {"status", &p.Status},
	}
	for _, f := range fields {
		v, msg := str(f.key)
		if msg != "" {
			return p, msg
		}
		*f.dst = v
	}
	if p.Category != nil && !validCategories[*p.Category] {
		return p, "categoría inválida"
	}
	if p.Status != nil && !validStatuses[*p.Status] {
		return p, "estado inválido"
	}
	if p.Title != nil && strings.TrimSpace(*p.Title) == "" {
		return p, "el título no puede quedar vacío"
	}
	if p.Amount != nil {
		c := extract.AmountCents(*p.Amount)
		p.AmountCents = &c
	}
	dl, msg := str("deadline")
	if msg != "" {
		return p, msg
	}
	if dl != nil {
		s := strings.TrimSpace(*dl)
		if s == "" {
			p.ClearDeadline = true
		} else {
			t, err := time.Parse("2006-01-02", s)
			if err != nil {
				return p, "deadline debe ser AAAA-MM-DD"
			}
			p.Deadline = &t
		}
	}
	return p, ""
}

// Delete descarta un fondo (falso positivo): guarda su huella para que el
// próximo crawl no lo vuelva a crear, y lo borra.
func (h *OpportunityHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.Opportunities.Dismiss(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// Review marks an opportunity as reviewed (confirmado).
func (h *OpportunityHandler) Review(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.Opportunities.MarkReviewed(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reviewed"})
}

// RefreshStatuses updates all opportunity statuses based on deadlines.
func (h *OpportunityHandler) RefreshStatuses(w http.ResponseWriter, r *http.Request) {
	updated, err := h.Opportunities.RefreshStatuses(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "refresh failed"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "refreshed", "updated": updated})
}

var reSHA = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Document sirve la copia local del PDF de un fondo (inline).
func (h *OpportunityHandler) Document(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	_, sha, err := h.Opportunities.DocumentFor(r.Context(), id)
	if err != nil || !reSHA.MatchString(sha) || h.DataDir == "" {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	// La ruta se arma con la huella (no con lo guardado): nada de "../".
	f, err := os.Open(filepath.Join(h.DataDir, "documentos", sha+".pdf"))
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", `inline; filename="`+sha[:12]+`.pdf"`)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", info.ModTime(), f)
}
