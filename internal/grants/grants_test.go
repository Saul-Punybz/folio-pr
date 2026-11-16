package grants

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestSearchContraRespuestaReal(t *testing.T) {
	fixture, err := os.ReadFile("testdata/search2_puerto_rico.json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("método %s", r.Method)
		}
		b, _ := io.ReadAll(r.Body)
		json.Unmarshal(b, &got)
		w.Header().Set("Content-Type", "application/json")
		w.Write(fixture)
	}))
	defer srv.Close()
	c := New(nil)
	c.endpoint = srv.URL
	items, err := c.Search(context.Background(), "Puerto Rico", 5)
	if err != nil {
		t.Fatal(err)
	}
	if got["keyword"] != "Puerto Rico" || got["oppStatuses"] != "forecasted|posted" {
		t.Errorf("petición inesperada: %v", got)
	}
	if len(items) != 5 {
		t.Fatalf("items = %d", len(items))
	}
	x := items[1]
	if x.ID != "361754" || x.Number != "PA-FPH-27-001" || x.Status != "posted" ||
		x.Title != "Title X Family Planning Services Grants" ||
		x.URL != "https://www.grants.gov/search-results-detail/361754" {
		t.Errorf("item: %+v", x)
	}
	if x.OpenDate == nil || x.OpenDate.Format("2006-01-02") != "2026-04-03" ||
		x.CloseDate == nil || x.CloseDate.Format("2006-01-02") != "2027-01-11" {
		t.Errorf("fechas: %v %v", x.OpenDate, x.CloseDate)
	}
	if items[0].CloseDate != nil || items[0].Status != "forecasted" {
		t.Errorf("forecasted sin cierre: %+v", items[0])
	}
}

func TestSearchErrores(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", 500)
	}))
	defer srv.Close()
	c := New(nil)
	c.endpoint = srv.URL
	if _, err := c.Search(context.Background(), "x", 5); err == nil {
		t.Fatal("esperaba error por HTTP 500")
	}
}

func TestFiltraEstadosCerrados(t *testing.T) {
	items, err := parse([]byte(`{"errorcode":0,"data":{"oppHits":[{"id":"1","oppStatus":"closed"},{"id":"2","oppStatus":"posted","openDate":"13/45/2020"}]}}`))
	if err != nil || len(items) != 1 || items[0].ID != "2" || items[0].OpenDate != nil {
		t.Errorf("%v %+v", err, items)
	}
}

func serve(t *testing.T, body []byte, got *map[string]any) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		if got != nil {
			json.Unmarshal(b, got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(body)
	}))
}

func fetchDetail(t *testing.T, file, id string) *Detail {
	t.Helper()
	fx, err := os.ReadFile("testdata/" + file)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	srv := serve(t, fx, &got)
	defer srv.Close()
	c := New(nil)
	c.detailEndpoint = srv.URL
	d, err := c.Detail(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got["opportunityId"] != json.Number(id) && got["opportunityId"] != float64(mustInt(id)) {
		t.Errorf("opportunityId enviado: %v", got["opportunityId"])
	}
	return d
}

func mustInt(s string) int {
	n := 0
	for _, r := range s {
		n = n*10 + int(r-'0')
	}
	return n
}

func TestSearchQueryEnviaFiltros(t *testing.T) {
	fx, _ := os.ReadFile("testdata/search2_filtrada_nonprofits.json")
	var got map[string]any
	srv := serve(t, fx, &got)
	defer srv.Close()
	c := New(nil)
	c.endpoint = srv.URL
	items, err := c.SearchQuery(context.Background(), Query{Keyword: "youth", Categories: "CD|ED", Rows: 6})
	if err != nil {
		t.Fatal(err)
	}
	if got["eligibilities"] != "12|13|25|99" || got["sortBy"] != "closeDate|asc" ||
		got["fundingCategories"] != "CD|ED" || got["keyword"] != "youth" || got["oppStatuses"] != "forecasted|posted" {
		t.Errorf("petición: %v", got)
	}
	if len(items) != 6 || items[1].ID != "364058" || items[1].CloseDate == nil {
		t.Errorf("items: %+v", items)
	}
}

func TestDetalleElegibleNonprofit(t *testing.T) {
	d := fetchDetail(t, "detalle_elegible_nonprofit.json", "364058")
	if d.Number != "O-OJJDP-2026-172743" || d.AwardCeiling != 2000000 || d.CloseDate == nil || d.CloseDate.Format("2006-01-02") != "2026-11-16" {
		t.Errorf("detalle: %+v", d)
	}
	if d.Agency == "" || d.Description == "" || strings.Contains(d.Description, "<") {
		t.Errorf("agencia/descripcion: %q / %.80q", d.Agency, d.Description)
	}
	r := Assess(d)
	if r.Level != High || r.Confidence < 70 || r.Evidence == "" {
		t.Errorf("relevancia: %+v", r)
	}
}

func TestDetalleSoloTribusSeDescarta(t *testing.T) {
	d := fetchDetail(t, "detalle_solo_tribus.json", "362842")
	if r := Assess(d); r.Level != Discarded {
		t.Errorf("tribus: %+v", r)
	}
}

func TestDetalleSoloUniversidadesSeDescarta(t *testing.T) {
	d := fetchDetail(t, "detalle_solo_universidades.json", "358090")
	if r := Assess(d); r.Level != Discarded {
		t.Errorf("universidades: %+v", r)
	}
}

func TestDetalleTerritoriosEsAlta(t *testing.T) {
	d := fetchDetail(t, "detalle_territorios.json", "363696")
	r := Assess(d)
	if r.Level != High || r.Confidence < 80 {
		t.Errorf("territorios: %+v", r)
	}
}

func TestDetallePronostico(t *testing.T) {
	d := fetchDetail(t, "detalle_pronostico.json", "355824")
	if !d.Forecast || d.AwardCeiling != 600000 || d.CloseDate == nil || d.CloseDate.Year() != 2025 {
		t.Errorf("pronóstico: %+v", d)
	}
}

func TestAssessReglasSinteticas(t *testing.T) {
	base := func() *Detail {
		return &Detail{ApplicantTypes: []string{"12"}, EligibilityText: "Nonprofit organizations", Title: "Programa", Description: "Servicios."}
	}
	cases := []struct {
		name string
		mod  func(*Detail)
		want Level
	}{
		{"nonprofit", func(d *Detail) {}, High},
		{"menciona Puerto Rico", func(d *Detail) { d.EligibilityText = "Nonprofits in Puerto Rico" }, High},
		{"solo estados", func(d *Detail) { d.EligibilityText = "Nonprofits located in the State of Ohio only" }, Discarded},
		{"estado pero PR incluido", func(d *Detail) { d.EligibilityText = "Nonprofits located in the state of Ohio or Puerto Rico" }, High},
		{"sin nonprofit", func(d *Detail) { d.ApplicantTypes = []string{"00", "06"} }, Discarded},
		{"otros sin texto nonprofit", func(d *Detail) { d.ApplicantTypes = []string{"25"}; d.EligibilityText = "Federally recognized tribes" }, Discarded},
		{"otros con nonprofit", func(d *Detail) {
			d.ApplicantTypes = []string{"25"}
			d.EligibilityText = "Nonprofit and nongovernmental organizations"
		}, High},
		{"investigación", func(d *Detail) { d.Agency = "National Institutes of Health" }, Low},
		{"embajada", func(d *Detail) { d.Title = "U.S. Embassy Cairo Public Diplomacy" }, Discarded},
		{"fellowship", func(d *Detail) { d.Title = "Graduate Fellowship Program" }, Discarded},
	}
	for _, c := range cases {
		d := base()
		c.mod(d)
		if r := Assess(d); r.Level != c.want {
			t.Errorf("%s: %+v, esperaba %s", c.name, r, c.want)
		}
	}
}

func TestPlainTextYTruncate(t *testing.T) {
	if got := PlainText("<p>Hola&nbsp;<b>mundo</b> &amp; más</p>"); got != "Hola mundo & más" {
		t.Errorf("PlainText = %q", got)
	}
	if got := Truncate("uno dos tres cuatro", 10); got != "uno dos…" {
		t.Errorf("Truncate = %q", got)
	}
	if got := Truncate("corto", 10); got != "corto" {
		t.Errorf("Truncate corto = %q", got)
	}
}
