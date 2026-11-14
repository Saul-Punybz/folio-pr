package crawler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/extract"
	"github.com/Saul-Punybz/folio/internal/models"
)

// ExtractionStats resume una pasada de extracción.
type ExtractionStats struct {
	Pages, Failed, Candidates, Created, Updated, Dismissed int
	ScannedDocs                                            int
}

// RunExtraction revisa las páginas y PDF nuevos o cambiados y crea los
// «posibles fondos» (reviewed=false). Una página se marca revisada solo si
// su extracción terminó sin error; si falla, se reintenta en la próxima.
func RunExtraction(ctx context.Context, deps Deps) ExtractionStats {
	var st ExtractionStats
	domains, err := deps.Domains.List(ctx)
	if err != nil {
		slog.Error("extraction: list domains", "err", err)
		return st
	}
	byID := make(map[uuid.UUID]*models.CrawlDomain, len(domains))
	for i := range domains {
		byID[domains[i].ID] = &domains[i]
	}

	failedIDs := map[uuid.UUID]bool{}
	for ctx.Err() == nil {
		pages, err := deps.Pages.ListPendingExtraction(ctx, 100)
		if err != nil {
			slog.Error("extraction: list pages", "err", err)
			break
		}
		progressed := false
		for _, p := range pages {
			if ctx.Err() != nil {
				break
			}
			if failedIDs[p.ID] {
				continue
			}
			progressed = true
			st.Pages++
			if err := extractPage(ctx, deps, byID[p.DomainID], p, &st); err != nil {
				slog.Warn("extraction: page failed, will retry", "url", p.URL, "err", err)
				st.Failed++
				failedIDs[p.ID] = true
				continue
			}
			if err := deps.Pages.MarkOpportunityChecked(ctx, p.ID); err != nil {
				st.Failed++
				failedIDs[p.ID] = true
			}
		}
		if !progressed {
			break
		}
	}
	slog.Info("extraction: done", "pages", st.Pages, "failed", st.Failed, "candidates", st.Candidates,
		"created", st.Created, "updated", st.Updated, "dismissed_skipped", st.Dismissed, "scanned_docs", st.ScannedDocs)
	return st
}

// source: de dónde sale un posible fondo.
type source struct {
	domain      *models.CrawlDomain
	pageID      uuid.UUID
	pageURL     string // página donde ella lo puede ver
	kind        string // pagina | pdf
	documentID  *uuid.UUID
	documentURL string // enlace remoto al PDF
	scanned     bool
	agencyHint  string // "Agencia: …" leído del HTML completo de la página
}

func extractPage(ctx context.Context, deps Deps, dom *models.CrawlDomain, p models.PageForExtraction, st *ExtractionStats) error {
	now := time.Now()
	src := source{domain: dom, pageID: p.ID, pageURL: p.URL, kind: "pagina"}

	pageCtx := p.Title
	if p.DocumentID != nil {
		src.kind = "pdf"
		src.documentID = p.DocumentID
		src.documentURL = p.URL
		if p.SourcePageID != nil {
			if title, u, err := deps.Pages.TitleAndURL(ctx, *p.SourcePageID); err == nil {
				pageCtx = strings.TrimSpace(p.AnchorText + "\n" + title)
				src.pageURL = u
			}
		}
		doc, err := deps.Documents.Get(ctx, *p.DocumentID)
		if err != nil {
			return err
		}
		src.scanned = doc.Scanned
		if doc.Scanned {
			return scannedCandidate(ctx, deps, src, p, pageCtx, st)
		}
	}

	ctxLine := contextLine(dom, pageCtx)

	// 1. El texto entero (un aviso suelto o un PDF de una convocatoria).
	whole := p.CleanText
	if src.kind == "pagina" {
		whole = deepCleanText(whole)
		if p.RawHTML != "" && len([]rune(whole)) < 200 {
			whole = HTMLText(p.RawHTML)
		}
	}
	if f := extract.Opportunity(whole+"\n"+ctxLine, now); f.IsCandidate {
		if t := stripSiteSuffix(extract.Opportunity(whole, now).Title); t != "" {
			f.Title, f.Evidence["title"] = t, t
		}
		if src.kind == "pagina" && p.RawHTML != "" && agencyFromText(whole) == "" {
			src.agencyHint = agencyFromText(HTMLText(p.RawHTML))
		}
		link := src.pageURL
		if src.kind == "pagina" {
			link = p.URL
		}
		return save(ctx, deps, src, f, whole, link, p.Title, st)
	}

	// 2. Un listado: cada fila por separado.
	var segments []Row
	if src.kind == "pagina" && p.RawHTML != "" {
		segments = SplitHTMLRows(p.RawHTML, p.URL)
	} else {
		for _, b := range SplitTextBlocks(whole) {
			segments = append(segments, Row{Text: b})
		}
	}
	var firstErr error
	for _, c := range RowCandidates(segments, ctxLine, now) {
		r, f := c.Row, c.Found
		rowSrc := src
		link := r.Link
		if link == "" || LooksLikePDF(link) {
			if LooksLikePDF(link) && rowSrc.documentURL == "" {
				rowSrc.documentURL = link // el PDF de la fila, como enlace remoto
			}
			link = src.pageURL
		}
		if err := save(ctx, deps, rowSrc, f, r.Text, link, r.LinkText, st); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// RowCandidate es una fila de un listado que parece convocatoria.
type RowCandidate struct {
	Row   Row
	Found extract.Found
}

// RowCandidates pasa cada fila por extract.Opportunity (con el contexto
// mínimo: dominio y título de la página) y devuelve las candidatas. Salta
// las filas que el propio listado marca como adjudicadas, canceladas,
// anuladas, desiertas o celebradas. El título sale de la fila sola.
func RowCandidates(rows []Row, ctxLine string, now time.Time) []RowCandidate {
	var out []RowCandidate
	for _, r := range rows {
		if closedStatus.MatchString(fold(r.Text)) {
			continue
		}
		f := extract.Opportunity(r.Text+"\n"+ctxLine, now)
		if !f.IsCandidate {
			continue
		}
		f.Title = extract.Opportunity(r.Text, now).Title
		if f.Title == "" {
			delete(f.Evidence, "title")
		} else {
			f.Evidence["title"] = f.Title
		}
		out = append(out, RowCandidate{Row: r, Found: f})
	}
	return out
}

// closedStatus: filas de un listado que ya no están abiertas (RUS).
var closedStatus = regexp.MustCompile(`estatus\s*:\s*(?:adjudicad|cancelad|anulad|desiert|celebrad)`)

// notNewCall: el enlace dice que no es una convocatoria nueva (adenda,
// adjudicación, enmienda…): sin texto que leer, no se ofrece como fondo.
var notNewCall = regexp.MustCompile(`addend|adenda|enmienda|amendment|adjudicaci|award|notificaci|cancelaci|resultado|minuta|acta\b|reglamento|formulario|planilla`)

// cleanTitle quita glifos de íconos (área de uso privado de Unicode, p. ej.
// los de Font Awesome) y espacios repetidos.
func cleanTitle(s string) string {
	s = strings.Map(func(r rune) rune {
		if (r >= 0xE000 && r <= 0xF8FF) || r == 0xFFFD {
			return -1
		}
		return r
	}, s)
	return strings.Join(strings.Fields(s), " ")
}

// convocatoriaHint: el enlace o la página parecen de una convocatoria.
var convocatoriaHint = regexp.MustCompile(`subasta|licitaci|\brfp|\brfq|\brfi\b|\bifb\b|propuesta|convocatoria|\baviso\b|solicitud|\bgrant|fondos|\bbecas?\b|\bnofo\b|\bbids?\b|proposal|cotizaci`)

func contextLine(dom *models.CrawlDomain, pageCtx string) string {
	parts := []string{}
	if dom != nil && dom.Label != "" {
		parts = append(parts, dom.Label)
	}
	if pageCtx = strings.TrimSpace(pageCtx); pageCtx != "" {
		parts = append(parts, pageCtx)
	}
	return strings.Join(parts, "\n")
}

// scannedCandidate: un PDF escaneado (sin texto) enlazado desde algo que
// parece convocatoria queda como posible fondo, sin datos inventados: título
// = texto del enlace, el PDF a la vista y document_scanned=true.
func scannedCandidate(ctx context.Context, deps Deps, src source, p models.PageForExtraction, pageCtx string, st *ExtractionStats) error {
	st.ScannedDocs++
	// Solo el enlace y el nombre del archivo: el título de la página
	// ("Avisos", "Documentos") hacía pasar reglamentos y formularios.
	hint := fold(p.AnchorText + " " + pdfFileTitle(p.URL))
	if !convocatoriaHint.MatchString(hint) || notNewCall.MatchString(hint) {
		return nil
	}
	title := cleanTitle(p.AnchorText)
	if name := pdfFileTitle(p.URL); len([]rune(title)) < 12 && name != "" && !strings.EqualFold(name, title) {
		if title != "" {
			title += " — " + name
		} else {
			title = name
		}
	}
	o := &models.Opportunity{
		PageID:            &src.pageID,
		Title:             title,
		Agency:            agencyName(src.domain, ""),
		Category:          "other",
		Status:            "unknown",
		SourceURL:         src.pageURL,
		Confidence:        20,
		Level:             levelFor(src.domain, ""),
		SourceKind:        "pdf",
		DocumentID:        src.documentID,
		DocumentRemoteURL: p.URL,
		DocScanned:        true,
		RawExtraction:     json.RawMessage(`{"source":"pdf-escaneado"}`),
	}
	if src.domain != nil {
		o.DomainID = &src.domain.ID
	}
	o.DedupeKey = DedupeKey("", "", "", nil, p.URL)
	return upsert(ctx, deps, o, st)
}

var (
	reAgencyLabel = regexp.MustCompile(`(?im)^\s*agencia\s*:[ \t]*(\S[^\n]{2,150})$`)
	reAgencyBelow = regexp.MustCompile(`(?im)^\s*agencia\s*:?\s*\n\s*(\S[^\n]{2,150})$`)
)

// agencyFromText: la agencia que el propio texto declara ("Agencia: …").
func agencyFromText(text string) string {
	if m := reAgencyLabel.FindStringSubmatch(text); m != nil {
		return strings.TrimSpace(m[1])
	}
	if m := reAgencyBelow.FindStringSubmatch(text); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func agencyName(dom *models.CrawlDomain, fromText string) string {
	if fromText != "" {
		return fromText
	}
	if dom != nil {
		if dom.Label != "" {
			return dom.Label
		}
		return dom.Domain
	}
	return ""
}

// levelFor: el nivel del dominio; una fila del RUS de un municipio es municipal.
func levelFor(dom *models.CrawlDomain, agency string) string {
	if a := fold(agency); strings.HasPrefix(a, "municipio") || strings.HasPrefix(a, "gobierno municipal") {
		return "municipal"
	}
	if dom != nil && models.ValidLevel(dom.Level) {
		return dom.Level
	}
	return "estatal"
}

// stripSiteSuffix: "Licitación 26J-16246-R1 | ASG Puerto Rico" → "Licitación 26J-16246-R1".
func stripSiteSuffix(t string) string {
	if i := strings.LastIndex(t, " | "); i > 0 && len([]rune(t[i+3:])) <= 40 {
		return strings.TrimSpace(t[:i])
	}
	return t
}

var reParenSuffix = regexp.MustCompile(`\s*\(([^()]*)\)\s*$`)

// stripRefSuffix quita "(27-SP-019)" del final de un título si es la referencia.
func stripRefSuffix(title, ref string) string {
	title = strings.TrimSpace(title)
	if m := reParenSuffix.FindStringSubmatch(title); m != nil && ref != "" &&
		NormalizeReference(m[1]) == NormalizeReference(ref) {
		return strings.TrimSpace(title[:len(title)-len(m[0])])
	}
	return title
}

func truncate(s string, n int) string {
	r := []rune(strings.Join(strings.Fields(s), " "))
	if len(r) <= n {
		return string(r)
	}
	return strings.TrimSpace(string(r[:n])) + "…"
}

func firstLine(s string) string {
	for _, l := range strings.Split(s, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

func save(ctx context.Context, deps Deps, src source, f extract.Found, text, link, fallbackTitle string, st *ExtractionStats) error {
	st.Candidates++
	fromText := agencyFromText(text)
	if fromText == "" {
		fromText = src.agencyHint
	}
	agency := agencyName(src.domain, fromText)
	title := f.Title
	if title == "" {
		title = truncate(stripRefSuffix(fallbackTitle, f.Reference), 160)
	}
	if title == "" {
		title = truncate(firstLine(text), 160)
	}
	title = cleanTitle(title)
	raw, _ := json.Marshal(map[string]any{"source": src.kind, "score": f.Score, "text": truncate(text, 600)})
	o := &models.Opportunity{
		PageID:            &src.pageID,
		Title:             title,
		Agency:            agency,
		Summary:           truncate(text, 400),
		Amount:            f.Amount,
		AmountCents:       f.AmountCents,
		Deadline:          f.Deadline,
		ContactEmail:      f.ContactEmail,
		ContactPhone:      f.ContactPhone,
		Category:          f.Category,
		Status:            models.OpportunityStatus(f.Deadline, time.Now()),
		SourceURL:         link,
		Confidence:        f.Score,
		RawExtraction:     raw,
		Reference:         f.Reference,
		Level:             levelFor(src.domain, agency),
		SourceKind:        src.kind,
		DocumentID:        src.documentID,
		DocumentRemoteURL: src.documentURL,
		DocScanned:        false,
		Evidence:          f.Evidence,
	}
	if src.domain != nil {
		o.DomainID = &src.domain.ID
	}
	o.DedupeKey = DedupeKey(f.Reference, agency, title, f.Deadline, link)
	return upsert(ctx, deps, o, st)
}

func upsert(ctx context.Context, deps Deps, o *models.Opportunity, st *ExtractionStats) error {
	created, err := deps.Opportunities.Upsert(ctx, o)
	switch {
	case errors.Is(err, models.ErrDismissed):
		st.Dismissed++
		return nil
	case err != nil:
		return fmt.Errorf("guardar fondo %q: %w", o.Title, err)
	case created:
		st.Created++
	default:
		st.Updated++
	}
	return nil
}
