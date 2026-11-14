package grants

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
