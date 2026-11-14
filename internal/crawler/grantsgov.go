package crawler

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/Saul-Punybz/folio/internal/grants"
	"github.com/Saul-Punybz/folio/internal/models"
)

// DefaultGrantsKeywords: palabras clave de Grants.gov si la tabla settings no
// tiene "grants_keywords" (una por línea).
var DefaultGrantsKeywords = []string{"Puerto Rico", "nonprofit Puerto Rico", "community development Puerto Rico"}

// GrantsMaxPerRun: tope de oportunidades de Grants.gov por corrida.
const GrantsMaxPerRun = 60

// GrantsStats resume una corrida de Grants.gov.
type GrantsStats struct {
	Seen, Created, Updated, Dismissed, HighConfidence, Errors int
}

// GrantsKeywords lee la lista configurable (settings.grants_keywords).
func GrantsKeywords(ctx context.Context, settings *models.SettingsStore) []string {
	if settings == nil {
		return DefaultGrantsKeywords
	}
	v, ok, err := settings.Get(ctx, "grants_keywords")
	if err != nil || !ok {
		return DefaultGrantsKeywords
	}
	var out []string
	for _, l := range strings.Split(v, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return DefaultGrantsKeywords
	}
	return out
}

// RunGrants busca en Grants.gov (solo posted/forecasted) y guarda cada
// oportunidad como posible fondo federal (source_kind=api), deduplicada por
// su ID. La búsqueda es de texto libre: confidence alto solo si «Puerto
// Rico» sale en el título o la agencia; el resto queda bajo (ella decide).
func RunGrants(ctx context.Context, opps *models.OpportunityStore, client *grants.Client, keywords []string) GrantsStats {
	var st GrantsStats
	if client == nil {
		client = grants.New(nil)
	}
	seen := map[string]bool{}
	for _, kw := range keywords {
		if ctx.Err() != nil || len(seen) >= GrantsMaxPerRun {
			break
		}
		items, err := client.Search(ctx, kw, 25)
		if err != nil {
			slog.Warn("grants: search failed", "keyword", kw, "err", err)
			st.Errors++
			continue
		}
		for _, it := range items {
			if len(seen) >= GrantsMaxPerRun {
				break
			}
			if it.ID == "" || seen[it.ID] {
				continue
			}
			seen[it.ID] = true
			st.Seen++
			o := grantToOpportunity(it, kw)
			if o.Confidence >= 70 {
				st.HighConfidence++
			}
			created, err := opps.Upsert(ctx, o)
			switch {
			case err == models.ErrDismissed:
				st.Dismissed++
			case err != nil:
				slog.Warn("grants: upsert", "id", it.ID, "err", err)
				st.Errors++
			case created:
				st.Created++
			default:
				st.Updated++
			}
		}
	}
	slog.Info("grants: done", "seen", st.Seen, "created", st.Created, "updated", st.Updated,
		"high_confidence", st.HighConfidence, "dismissed_skipped", st.Dismissed, "errors", st.Errors)
	return st
}

func grantToOpportunity(it grants.Item, keyword string) *models.Opportunity {
	conf := 25
	if strings.Contains(fold(it.Title+" "+it.Agency), "puerto rico") {
		conf = 80
	}
	var deadline *time.Time
	if it.CloseDate != nil {
		d := *it.CloseDate
		deadline = &d
	}
	status := models.OpportunityStatus(deadline, time.Now())
	raw, _ := json.Marshal(map[string]any{"source": "grants.gov", "id": it.ID, "status": it.Status, "keyword": keyword})
	ev := map[string]string{}
	if it.Number != "" {
		ev["reference"] = "Opportunity Number " + it.Number
	}
	if it.CloseDate != nil {
		ev["deadline"] = "Close Date " + it.CloseDate.Format("01/02/2006")
	}
	ev["title"] = it.Title
	return &models.Opportunity{
		Title:         it.Title,
		Agency:        it.Agency,
		Reference:     it.Number,
		Deadline:      deadline,
		Category:      "grant",
		Status:        status,
		SourceURL:     it.URL,
		Confidence:    conf,
		RawExtraction: raw,
		Level:         "federal",
		SourceKind:    "api",
		Evidence:      ev,
		DedupeKey:     "grantsgov:" + it.ID,
	}
}
