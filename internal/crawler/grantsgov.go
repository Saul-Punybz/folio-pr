package crawler

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"log/slog"
	"sort"
	"strings"
	"time"

	"github.com/Saul-Punybz/folio/internal/grants"
	"github.com/Saul-Punybz/folio/internal/models"
)

// Flujo de Grants.gov (sin IA):
//  1. Búsquedas filtradas por elegibilidad de nonprofits (eligibilities
//     "12|13|25|99"), una por palabra clave y una por categoría, 1 s entre
//     llamadas. La primera consulta es siempre «Puerto Rico».
//  2. De los candidatos únicos (tope GrantsMaxDetails, intercalados entre
//     consultas) se pide el detalle (fetchOpportunity, 1 s entre llamadas).
//  3. grants.Assess clasifica: alta / baja / descartada. Lo descartado no se
//     inserta; lo bajo solo entra si sobra lugar.
//  4. Tope GrantsMaxPerRun (25), ordenados por fecha de cierre más próxima.
//
// Configuración en la tabla settings (clave/valor; no hace falta migración):
//
//	grants_keywords    una palabra clave por línea. Por defecto: Puerto Rico,
//	                   community development, youth, education, health,
//	                   housing, arts, environment. Una línea «categoria:CD|ED»
//	                   equivale a grants_categories (ver abajo).
//	grants_categories  códigos fundingCategories de Grants.gov separados por
//	                   coma, espacio o línea (CD ED HL HO AR ENV ISS ELT FN
//	                   DPR HU RT...). Por defecto: CD, ED, HL, HO, AR, ENV, ISS.
//
// Compatibilidad: cmd/folio llama RunGrants(ctx, opps, client,
// GrantsKeywords(ctx, settings)); GrantsKeywords agrega las categorías como
// una línea «categoria:...» para que esa firma no cambie.

// DefaultGrantsKeywords: temas típicos de una nonprofit comunitaria.
var DefaultGrantsKeywords = []string{"Puerto Rico", "community development", "youth", "education", "health", "housing", "arts", "environment"}

// DefaultGrantsCategories: fundingCategories por defecto.
var DefaultGrantsCategories = []string{"CD", "ED", "HL", "HO", "AR", "ENV", "ISS"}

const (
	// GrantsMaxPerRun: tope de oportunidades de Grants.gov por corrida.
	GrantsMaxPerRun = 25
	// GrantsMaxDetails: tope de peticiones de detalle por corrida.
	GrantsMaxDetails = 60
	grantsRowsPerQ   = 20
	categoryPrefix   = "categoria:"
)

// GrantsStats resume una corrida de Grants.gov.
type GrantsStats struct {
	Searches, Candidates, DetailsFetched      int
	Seen, Created, Updated, Dismissed, Errors int
	HighConfidence                            int // relevancia alta
	Low, Discarded                            int // baja (evaluada) / descartada
}

// GrantsKeywords lee settings.grants_keywords y settings.grants_categories.
// Devuelve las palabras clave y, al final, «categoria:CD|ED|...».
func GrantsKeywords(ctx context.Context, settings *models.SettingsStore) []string {
	kws, cats := DefaultGrantsKeywords, DefaultGrantsCategories
	if settings != nil {
		if v, ok, err := settings.Get(ctx, "grants_keywords"); err == nil && ok {
			if l := splitList(v, "\n"); len(l) > 0 {
				kws = l
			}
		}
		if v, ok, err := settings.Get(ctx, "grants_categories"); err == nil && ok {
			if l := splitList(v, "\n,; "); len(l) > 0 {
				cats = l
			}
		}
	}
	out := append([]string(nil), kws...)
	return append(out, categoryPrefix+strings.Join(cats, "|"))
}

func splitList(v, seps string) []string {
	var out []string
	for _, l := range strings.FieldsFunc(v, func(r rune) bool { return strings.ContainsRune(seps, r) }) {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	return out
}

// grantsPlan: consultas ya resueltas.
type grantsPlan struct {
	keywords   []string
	categories []string
}

func planFromKeywords(in []string) grantsPlan {
	var p grantsPlan
	for _, k := range in {
		if strings.HasPrefix(strings.ToLower(k), categoryPrefix) {
			p.categories = append(p.categories, splitList(k[len(categoryPrefix):], "|,; ")...)
			continue
		}
		p.keywords = append(p.keywords, k)
	}
	if len(p.keywords) == 0 {
		p.keywords = DefaultGrantsKeywords
	}
	if len(p.categories) == 0 {
		p.categories = DefaultGrantsCategories
	}
	return p
}

// GrantCandidate es una oportunidad evaluada, lista para insertar.
type GrantCandidate struct {
	Opp *models.Opportunity
	Rel grants.Relevance
}

type grantsRunner struct {
	client *grants.Client
	delay  time.Duration
	now    time.Time
}

func (r *grantsRunner) pause(ctx context.Context) {
	if r.delay <= 0 {
		return
	}
	select {
	case <-ctx.Done():
	case <-time.After(r.delay):
	}
}

// CollectGrants hace las búsquedas, trae detalles y evalúa; NO toca la base
// de datos. Devuelve los elegidos (máx. GrantsMaxPerRun, por cierre más
// próximo) y estadísticas parciales. Sirve también para probar en vivo.
func CollectGrants(ctx context.Context, client *grants.Client, keywords []string, delay time.Duration, now time.Time) ([]GrantCandidate, GrantsStats) {
	var st GrantsStats
	if client == nil {
		client = grants.New(nil)
	}
	r := &grantsRunner{client: client, delay: delay, now: now}
	plan := planFromKeywords(keywords)

	// 1. búsquedas
	var queries []grants.Query
	for _, k := range plan.keywords {
		queries = append(queries, grants.Query{Keyword: k, Rows: grantsRowsPerQ})
	}
	for _, c := range plan.categories {
		queries = append(queries, grants.Query{Categories: c, Rows: grantsRowsPerQ})
	}
	var lists [][]grants.Item
	for i, q := range queries {
		if ctx.Err() != nil {
			break
		}
		if i > 0 {
			r.pause(ctx)
		}
		items, err := client.SearchQuery(ctx, q)
		st.Searches++
		if err != nil {
			slog.Warn("grants: search failed", "query", q, "err", err)
			st.Errors++
			continue
		}
		lists = append(lists, items)
	}
	// intercala resultados entre consultas y deduplica
	seen := map[string]bool{}
	var cands []grants.Item
	for i := 0; len(cands) < GrantsMaxDetails; i++ {
		any := false
		for _, l := range lists {
			if i >= len(l) {
				continue
			}
			any = true
			it := l[i]
			if it.ID == "" || seen[it.ID] || len(cands) >= GrantsMaxDetails {
				continue
			}
			if it.CloseDate != nil && it.CloseDate.Before(dayStart(now)) {
				continue
			}
			seen[it.ID] = true
			cands = append(cands, it)
		}
		if !any {
			break
		}
	}
	st.Candidates = len(cands)

	// 2-3. detalle y relevancia
	var high, low []GrantCandidate
	for i, it := range cands {
		if ctx.Err() != nil {
			break
		}
		if i > 0 {
			r.pause(ctx)
		}
		d, err := client.Detail(ctx, it.ID)
		st.DetailsFetched++
		if err != nil {
			slog.Warn("grants: detail failed", "id", it.ID, "err", err)
			st.Errors++
			continue
		}
		if d.CloseDate != nil && d.CloseDate.Before(dayStart(now)) {
			st.Discarded++ // ya cerró o el pronóstico quedó viejo
			continue
		}
		if d.Agency == "" {
			d.Agency = it.Agency
		}
		rel := grants.Assess(d)
		switch rel.Level {
		case grants.High:
			st.HighConfidence++
			high = append(high, GrantCandidate{grantOpportunity(it, d, rel), rel})
		case grants.Low:
			st.Low++
			low = append(low, GrantCandidate{grantOpportunity(it, d, rel), rel})
		default:
			st.Discarded++
		}
	}

	// 4. tope: primero las altas, luego las bajas hasta completar; el orden
	// final es por cierre más próximo (sin fecha, al final).
	byClose := func(s []GrantCandidate) {
		sort.SliceStable(s, func(i, j int) bool { return closeBefore(s[i].Opp.Deadline, s[j].Opp.Deadline) })
	}
	byScore := func(s []GrantCandidate) {
		sort.SliceStable(s, func(i, j int) bool {
			if s[i].Rel.Confidence != s[j].Rel.Confidence {
				return s[i].Rel.Confidence > s[j].Rel.Confidence
			}
			return closeBefore(s[i].Opp.Deadline, s[j].Opp.Deadline)
		})
	}
	byScore(high)
	byScore(low)
	chosen := append([]GrantCandidate(nil), high...)
	chosen = append(chosen, low...)
	if len(chosen) > GrantsMaxPerRun {
		chosen = chosen[:GrantsMaxPerRun]
	}
	byClose(chosen)
	return chosen, st
}

func closeBefore(a, b *time.Time) bool {
	switch {
	case a == nil:
		return false
	case b == nil:
		return true
	}
	return a.Before(*b)
}

func dayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// RunGrants busca en Grants.gov, evalúa la relevancia con el detalle de cada
// oportunidad y guarda como posible fondo federal (source_kind=api,
// deduplicado por ID) solo las que no se descartan, máx. GrantsMaxPerRun.
func RunGrants(ctx context.Context, opps *models.OpportunityStore, client *grants.Client, keywords []string) GrantsStats {
	chosen, st := CollectGrants(ctx, client, keywords, time.Second, time.Now())
	for _, c := range chosen {
		if ctx.Err() != nil {
			break
		}
		st.Seen++
		created, err := opps.Upsert(ctx, c.Opp)
		switch {
		case err == models.ErrDismissed:
			st.Dismissed++
		case err != nil:
			slog.Warn("grants: upsert", "dedupe", c.Opp.DedupeKey, "err", err)
			st.Errors++
		case created:
			st.Created++
		default:
			st.Updated++
		}
	}
	slog.Info("grants: done", "searches", st.Searches, "candidates", st.Candidates, "details", st.DetailsFetched,
		"high", st.HighConfidence, "low", st.Low, "discarded", st.Discarded,
		"created", st.Created, "updated", st.Updated, "dismissed_skipped", st.Dismissed, "errors", st.Errors)
	return st
}

func grantOpportunity(it grants.Item, d *grants.Detail, rel grants.Relevance) *models.Opportunity {
	var deadline *time.Time
	switch {
	case d.CloseDate != nil:
		t := *d.CloseDate
		deadline = &t
	case it.CloseDate != nil:
		t := *it.CloseDate
		deadline = &t
	}
	status := models.OpportunityStatus(deadline, time.Now())
	number := firstNonEmptyStr(d.Number, it.Number)
	title := html.UnescapeString(firstNonEmptyStr(d.Title, it.Title))
	var amount string
	var cents int64
	if d.AwardCeiling > 0 {
		amount = "$" + commas(d.AwardCeiling)
		cents = d.AwardCeiling * 100
	}
	raw, _ := json.Marshal(map[string]any{
		"source": "grants.gov", "id": it.ID, "status": it.Status, "forecast": d.Forecast,
		"relevance": rel.Level.String(), "reason": rel.Reason, "applicant_types": d.ApplicantTypes,
		"award_floor": d.AwardFloor, "estimated_funding": d.EstimatedFunding,
	})
	ev := map[string]string{"title": title, "relevance": rel.Reason}
	if number != "" {
		ev["reference"] = "Opportunity Number " + number
	}
	if deadline != nil {
		lbl := "Close Date "
		if d.Forecast {
			lbl = "Estimated Close Date "
		}
		ev["deadline"] = lbl + deadline.Format("01/02/2006")
	}
	if rel.Evidence != "" {
		ev["eligibility"] = rel.Evidence
	}
	if amount != "" {
		ev["amount"] = "Award Ceiling " + amount
	}
	return &models.Opportunity{
		Title:         title,
		Agency:        html.UnescapeString(firstNonEmptyStr(d.Agency, it.Agency)),
		Summary:       grants.Truncate(d.Description, 300),
		Amount:        amount,
		AmountCents:   cents,
		Reference:     number,
		Deadline:      deadline,
		Category:      "grant",
		Status:        status,
		SourceURL:     it.URL,
		Confidence:    rel.Confidence,
		RawExtraction: raw,
		Level:         "federal",
		SourceKind:    "api",
		Evidence:      ev,
		DedupeKey:     "grantsgov:" + it.ID,
	}
}

func firstNonEmptyStr(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func commas(n int64) string {
	s := fmt.Sprint(n)
	for i := len(s) - 3; i > 0; i -= 3 {
		s = s[:i] + "," + s[i:]
	}
	return s
}
