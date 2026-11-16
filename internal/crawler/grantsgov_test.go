package crawler

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/Saul-Punybz/folio/internal/grants"
)

func TestPlanFromKeywords(t *testing.T) {
	p := planFromKeywords([]string{"Puerto Rico", "youth", "categoria:CD|ED", "categoria:HL"})
	if len(p.keywords) != 2 || len(p.categories) != 3 || p.categories[2] != "HL" {
		t.Errorf("plan: %+v", p)
	}
	d := planFromKeywords(nil)
	if len(d.keywords) != len(DefaultGrantsKeywords) || len(d.categories) != len(DefaultGrantsCategories) {
		t.Errorf("defaults: %+v", d)
	}
	if kw := GrantsKeywords(context.Background(), nil); kw[len(kw)-1] != "categoria:CD|ED|HL|HO|AR|ENV|ISS" {
		t.Errorf("GrantsKeywords: %v", kw)
	}
}

func TestCollectGrantsFiltraYOrdena(t *testing.T) {
	fx := func(f string) []byte {
		b, err := os.ReadFile("../grants/testdata/" + f)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	byID := map[string][]byte{
		"364058": fx("detalle_elegible_nonprofit.json"),
		"362842": fx("detalle_solo_tribus.json"),
		"358090": fx("detalle_solo_universidades.json"),
		"363696": fx("detalle_territorios.json"),
	}
	// búsqueda con ids que mezclan elegible, tribus, universidades y territorios
	search := `{"errorcode":0,"msg":"ok","data":{"hitCount":4,"oppHits":[
	 {"id":"363696","number":"AC-08-28-26","title":"AmeriCorps","agency":"AmeriCorps","openDate":"08/28/2026","closeDate":"01/13/2027","oppStatus":"posted"},
	 {"id":"362842","number":"T","title":"Native Entities Grant Program","agency":"X","openDate":"01/01/2026","closeDate":"12/01/2026","oppStatus":"posted"},
	 {"id":"358090","number":"U","title":"HEAL","agency":"NIH","openDate":"01/01/2026","closeDate":"12/01/2026","oppStatus":"posted"},
	 {"id":"364058","number":"O","title":"OJJDP Mentoring","agency":"OJJDP","openDate":"01/01/2026","closeDate":"11/16/2026","oppStatus":"posted"}]}}`
	var mu sync.Mutex
	var searches, details int
	var firstQuery map[string]any
	mux := http.NewServeMux()
	mux.HandleFunc("/search", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		mu.Lock()
		if searches == 0 {
			json.Unmarshal(b, &firstQuery)
		}
		searches++
		mu.Unlock()
		w.Write([]byte(search))
	})
	mux.HandleFunc("/fetch", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			ID int `json:"opportunityId"`
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &req)
		mu.Lock()
		details++
		mu.Unlock()
		id := strings.TrimSpace(strings.Trim(string(json.Number(itoa(req.ID))), `"`))
		body, ok := byID[id]
		if !ok {
			http.Error(w, "no", 404)
			return
		}
		w.Write(body)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()
	client := grants.NewWithEndpoints(nil, srv.URL+"/search", srv.URL+"/fetch")

	now := time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)
	got, st := CollectGrants(context.Background(), client, []string{"Puerto Rico", "categoria:CD|ED"}, 0, now)
	if firstQuery["keyword"] != "Puerto Rico" || firstQuery["eligibilities"] != "12|13|25|99" {
		t.Errorf("primera consulta: %v", firstQuery)
	}
	if searches != 3 || details != 4 { // 1 keyword + 2 categorías; 4 ids únicos
		t.Errorf("searches=%d details=%d", searches, details)
	}
	if st.Candidates != 4 || st.DetailsFetched != 4 || st.HighConfidence != 2 || st.Discarded != 2 || st.Errors != 0 {
		t.Errorf("stats: %+v", st)
	}
	if len(got) != 2 {
		t.Fatalf("elegidos = %d", len(got))
	}
	// orden por cierre: 16 nov 2026 antes que 13 ene 2027
	a, b := got[0].Opp, got[1].Opp
	if a.Reference != "O-OJJDP-2026-172743" || b.Reference != "AC-08-28-26" {
		t.Errorf("orden/refs: %q %q", a.Reference, b.Reference)
	}
	if a.Amount != "$2,000,000" || a.AmountCents != 200000000 || b.Amount != "" || b.AmountCents != 0 {
		t.Errorf("montos: %q/%d %q/%d", a.Amount, a.AmountCents, b.Amount, b.AmountCents)
	}
	if n := utf8.RuneCountInString(a.Summary); n == 0 || n > 301 || strings.Contains(a.Summary, "<") {
		t.Errorf("summary (%d): %q", n, a.Summary)
	}
	if a.Agency == "" || a.Deadline == nil || a.Confidence < 70 || a.Evidence["eligibility"] == "" ||
		a.DedupeKey != "grantsgov:364058" || a.SourceKind != "api" || a.Category != "grant" {
		t.Errorf("opp: %+v", a)
	}
	if b.Confidence < 80 {
		t.Errorf("territorios confidence = %d", b.Confidence)
	}
}

func itoa(n int) string {
	b, _ := json.Marshal(n)
	return string(b)
}
