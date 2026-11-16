package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"log/slog"
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

	"github.com/Saul-Punybz/folio/internal/crawler"
	"github.com/Saul-Punybz/folio/internal/extract"
	"github.com/Saul-Punybz/folio/internal/models"
)

// OpportunityHandler handles grant/RFP opportunity API endpoints.
type OpportunityHandler struct {
	Opportunities *models.OpportunityStore
	Documents     *models.DocumentStore
	// DataDir: carpeta de datos (los PDF están en DataDir/documentos).
	DataDir string
	// Fetch baja un PDF por su enlace (por defecto, la descarga del crawler).
	Fetch func(ctx context.Context, url string, maxBytes int64) (*crawler.FetchResult, error)
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

// Review guarda un fondo (en la interfaz, «Guardar»; el nombre es contrato) y
// conserva su PDF: si solo hay enlace remoto, lo baja ahora. Si la descarga
// falla el fondo se guarda igual. pdf: "kept" | "remote" (sigue solo el enlace) | "none".
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
	pdf := "none"
	switch kerr := h.keepPDF(r.Context(), id); {
	case kerr == nil:
		pdf = "kept"
	case errors.Is(kerr, errNoPDF):
	default:
		slog.Warn("guardar fondo: no se pudo conservar el PDF", "opportunity", id, "err", kerr)
		pdf = "remote"
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "reviewed", "pdf": pdf})
}

// Unreview saca el fondo de Guardados (vuelve a Por revisar).
func (h *OpportunityHandler) Unreview(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if err := h.Opportunities.Unreview(r.Context(), id); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unreviewed"})
}

var errNoPDF = errors.New("el fondo no tiene PDF")

// keepPDF marca como guardado el PDF de un fondo; si no está en disco y hay
// enlace remoto, lo baja primero (con los topes del crawler).
func (h *OpportunityHandler) keepPDF(ctx context.Context, id uuid.UUID) error {
	if h.Documents == nil {
		return errNoPDF
	}
	if ok, err := h.Documents.KeepForOpportunity(ctx, id); err != nil {
		return err
	} else if ok {
		return nil
	}
	opp, err := h.Opportunities.GetByID(ctx, id)
	if err != nil {
		return err
	}
	remote := opp.DocumentRemoteURL
	if remote == "" && opp.SourceKind == "pdf" {
		remote = opp.SourceURL
	}
	if remote == "" {
		return errNoPDF
	}
	fetch := h.Fetch
	if fetch == nil {
		fetch = crawler.FetchPDF
	}
	dctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	doc, err := crawler.DownloadDocumentWith(dctx, h.Documents, h.DataDir, remote, fetch)
	if err != nil {
		return err
	}
	if err := h.Opportunities.SetDocument(ctx, id, doc.ID); err != nil {
		return err
	}
	return h.Documents.SetKept(ctx, doc.ID, true)
}

// KeepDocument guarda solo el PDF del fondo (el fondo no cambia).
func (h *OpportunityHandler) KeepDocument(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	switch err := h.keepPDF(r.Context(), id); {
	case err == nil:
		writeJSON(w, http.StatusOK, map[string]string{"status": "kept"})
	case errors.Is(err, errNoPDF), errors.Is(err, pgx.ErrNoRows):
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
	default:
		slog.Warn("guardar PDF", "opportunity", id, "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "no se pudo bajar el PDF"})
	}
}

// UnkeepDocument deja de guardar el PDF del fondo (la retención vuelve a aplicarle).
func (h *OpportunityHandler) UnkeepDocument(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	if h.Documents == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if ok, err := h.Documents.UnkeepForOpportunity(r.Context(), id); err != nil || !ok {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unkept"})
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

// Document sirve la copia local del PDF de un fondo: inline, o con
// ?download=1 como descarga con un nombre legible.
func (h *OpportunityHandler) Document(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid id"})
		return
	}
	_, sha, err := h.Opportunities.DocumentFor(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	name := sha[:min(12, len(sha))]
	if opp, err := h.Opportunities.GetByID(r.Context(), id); err == nil {
		name = PDFFileName(opp.Reference, opp.Title, name)
	}
	h.servePDF(w, r, sha, name)
}

// DocumentFile sirve un PDF por el id del documento (lista de PDF guardados).
func (h *OpportunityHandler) DocumentFile(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || h.Documents == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	sha, err := h.Documents.SHAFor(r.Context(), id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	name := sha[:min(12, len(sha))]
	if kept, _, err := h.Documents.ListKept(r.Context()); err == nil {
		for _, k := range kept {
			if k.ID == id {
				name = PDFFileName(k.Reference, k.Title, name)
			}
		}
	}
	h.servePDF(w, r, sha, name)
}

// UnkeepDocumentByID suelta un PDF desde la lista de PDF guardados.
func (h *OpportunityHandler) UnkeepDocumentByID(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil || h.Documents == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	if err := h.Documents.SetKept(r.Context(), id, false); err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unkept"})
}

// KeptDocuments lista los PDF guardados y el espacio que ocupan: kept_bytes
// (los guardados) y total_bytes (toda la carpeta de datos de Folio).
func (h *OpportunityHandler) KeptDocuments(w http.ResponseWriter, r *http.Request) {
	if h.Documents == nil {
		writeJSON(w, http.StatusOK, map[string]any{"documents": []models.KeptDoc{}, "kept_bytes": 0, "total_bytes": 0})
		return
	}
	docs, kept, err := h.Documents.ListKept(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal error"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"documents": docs, "count": len(docs),
		"kept_bytes": kept, "total_bytes": dirSize(h.DataDir)})
}

// dirSize suma el tamaño de los archivos de una carpeta (sin seguir enlaces).
func dirSize(root string) int64 {
	if root == "" {
		return 0
	}
	var n int64
	_ = filepath.WalkDir(root, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if info, err := d.Info(); err == nil && info.Mode().IsRegular() {
			n += info.Size()
		}
		return nil
	})
	return n
}

func (h *OpportunityHandler) servePDF(w http.ResponseWriter, r *http.Request, sha, name string) {
	if !reSHA.MatchString(sha) || h.DataDir == "" {
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
	disp := "inline"
	if boolParam(r, "download") != nil && *boolParam(r, "download") {
		disp = "attachment"
	}
	w.Header().Set("Content-Type", "application/pdf")
	w.Header().Set("Content-Disposition", disp+`; filename="`+name+`"`)
	w.Header().Set("Cache-Control", "private, max-age=86400")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, "", info.ModTime(), f)
}

var reNotName = regexp.MustCompile(`[^A-Za-z0-9._-]+`)
var asciiFold = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n",
	"Á", "A", "É", "E", "Í", "I", "Ó", "O", "Ú", "U", "Ü", "U", "Ñ", "N")

// PDFFileName arma «<referencia o título corto>.pdf»: solo letras, números,
// guion y punto; si no queda nada usa fallback.
func PDFFileName(reference, title, fallback string) string {
	base := strings.TrimSpace(reference)
	if base == "" {
		base = strings.TrimSpace(title)
	}
	base = reNotName.ReplaceAllString(asciiFold.Replace(base), "-")
	base = strings.Trim(base, "-._")
	if len(base) > 60 {
		base = strings.Trim(base[:60], "-._")
	}
	if base == "" {
		base = fallback
	}
	return base + ".pdf"
}
