package crawler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Saul-Punybz/folio/internal/models"
)

var nov16 = time.Date(2026, 11, 16, 9, 0, 0, 0, ast)

// HTML real (recortado) de la lista de avisos de Bayamón: un encabezado en
// negrita y un párrafo por aviso; no hay <li> ni <tr>, el aviso es el texto
// del enlace.
func TestSplitBayamonLinkRows(t *testing.T) {
	raw, err := os.ReadFile("testdata/bayamon_avisos.html")
	if err != nil {
		t.Fatal(err)
	}
	rows := SplitHTMLRows(string(raw), "https://www.municipiodebayamon.com/administracion/avisos-publicos/")
	var subastas, propuestas, ordenanzas int
	for _, r := range rows {
		if !LooksLikePDF(r.Link) {
			t.Errorf("fila sin PDF: %q -> %q", r.Text, r.Link)
		}
		switch {
		case strings.Contains(r.LinkText, "Subasta #"):
			subastas++
			if !strings.Contains(r.Text, "Aviso de Subasta") || !strings.Contains(r.Text, "Publicado en el Periódico") {
				t.Errorf("la fila perdió su encabezado: %q", r.Text)
			}
		case strings.Contains(r.LinkText, "SP-2026-27"):
			propuestas++
			if !strings.Contains(r.Text, "Solicitud de Propuesta") {
				t.Errorf("la propuesta perdió su encabezado: %q", r.Text)
			}
		case strings.Contains(r.LinkText, "Ordenanza"):
			ordenanzas++
		}
	}
	if subastas != 5 || propuestas != 2 || ordenanzas == 0 {
		for _, r := range rows {
			t.Logf("fila: %.90q", r.Text)
		}
		t.Fatalf("subastas=%d propuestas=%d ordenanzas=%d (quería 5, 2, >0)", subastas, propuestas, ordenanzas)
	}
	// Las ordenanzas no se bajan; las subastas y la propuesta sí.
	for _, r := range rows {
		v := ClassifyPDFLink(r.LinkText, r.Link, nov16)
		isOrd := strings.Contains(r.LinkText, "Ordenanza") || strings.Contains(r.LinkText, "Edicto")
		if isOrd && v.Download {
			t.Errorf("se iba a bajar una ordenanza: %q", r.LinkText)
		}
		if !isOrd && !v.Download {
			t.Errorf("se descartó una convocatoria: %q (%s)", r.LinkText, v.Reason)
		}
	}
}

// Caguas: una tabla con una fila por aviso; el enlace dice solo «Aviso», el
// aviso es la fila.
func TestCaguasTablaFilaPorAviso(t *testing.T) {
	raw, err := os.ReadFile("testdata/caguas_tabla.html")
	if err != nil {
		t.Fatal(err)
	}
	rows := SplitHTMLRows(string(raw), "https://caguas.gov.pr/avisos-publicos-y-subastas/")
	if len(rows) != 5 {
		t.Fatalf("filas = %d, se esperaban 5", len(rows))
	}
	if rows[0].LinkText != "Aviso" || !strings.HasSuffix(rows[0].Link, "Aviso-RFP-S-2027_018.pdf") {
		t.Errorf("fila 0: %+v", rows[0])
	}
	if !strings.Contains(rows[0].Text, "2027-018") || !strings.Contains(rows[0].Text, "Piscinas") {
		t.Errorf("la fila perdió el número o el nombre: %q", rows[0].Text)
	}
	// Una fila de PDF sin fecha límite no se ofrece como fondo: manda el PDF.
	for _, c := range RowCandidates(rows, "Municipio Autónomo de Caguas", nov16) {
		if c.Found.Deadline == nil && ClassifyPDFLink(c.Row.LinkText+" "+c.Row.Text, c.Row.Link, nov16).Download {
			continue // saveRows la salta
		}
		t.Logf("candidata con fecha o sin PDF bajable: %.60q", c.Row.Text)
	}
}

// FetchPage le suma al enlace «Aviso» el texto de su fila, para el filtro y el título.
func TestAnchorConTextoDeLaFila(t *testing.T) {
	raw, err := os.ReadFile("testdata/caguas_tabla.html")
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	res, err := FetchPage(context.Background(), srv.URL+"/avisos", map[string]bool{})
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, l := range res.Links {
		if strings.HasSuffix(l.URL, "Aviso-RFP-S-2027_018.pdf") {
			found = true
			if !strings.Contains(l.AnchorText, "Piscinas") || !strings.Contains(l.AnchorText, "2027-018") {
				t.Errorf("anchor sin contexto: %q", l.AnchorText)
			}
		}
	}
	if !found {
		t.Fatal("no salió el enlace al PDF")
	}
}

func TestGranicusFilas(t *testing.T) {
	raw, err := os.ReadFile("testdata/guaynabo_granicus.json")
	if err != nil {
		t.Fatal(err)
	}
	if !isGranicusJSON(string(raw)) {
		t.Fatal("no se reconoció la rejilla")
	}
	rows := granicusRows(string(raw), "https://portalsuplidores.guaynabocity.gov.pr/Auction/FillAuctionGrid")
	// Del recorte: 27-F-050 y 27-F-044 y 27-F-039 y 27-RFP-001 publicadas; 27-F-043 y 27-F-038 en evaluación.
	if len(rows) != 4 {
		for _, r := range rows {
			t.Logf("fila: %.70q", r.Text)
		}
		t.Fatalf("filas = %d, se esperaban 4 (las «En evaluación» no)", len(rows))
	}
	dom := &models.CrawlDomain{Label: "Municipio Autónomo de Guaynabo", Level: "municipal"}
	cands := RowCandidates(rows, contextLine(dom, ""), nov16)
	byRef := map[string]RowCandidate{}
	for _, c := range cands {
		byRef[NormalizeReference(c.Found.Reference)] = c
	}
	for ref, dl := range map[string]string{"27-f-50": "2026-11-24", "27-rfp-1": "2026-12-04"} {
		c, ok := byRef[ref]
		if !ok {
			t.Errorf("falta %s; candidatas: %v", ref, len(cands))
			continue
		}
		if c.Found.Deadline == nil || c.Found.Deadline.Format("2006-01-02") != dl {
			t.Errorf("%s: fecha límite %v, quería %s", ref, c.Found.Deadline, dl)
		}
		if c.Row.Link != "https://portalsuplidores.guaynabocity.gov.pr/Auction/AuctionSearch" {
			t.Errorf("%s: enlace %q", ref, c.Row.Link)
		}
	}
	if c, ok := byRef["27-f-50"]; ok && !strings.Contains(c.Row.LinkText, "SLOPE STABILIZATION") {
		t.Errorf("nombre de la subasta %q", c.Row.LinkText)
	}
	// 27-F-039 cerró el 18 nov y 27-F-044 el 24 nov: abiertas hoy. Con fecha de
	// después del cierre (1 dic) ya no es candidata ninguna de las de nov.
	late := RowCandidates(rows, contextLine(dom, ""), time.Date(2026, 12, 1, 9, 0, 0, 0, ast))
	for _, c := range late {
		if NormalizeReference(c.Found.Reference) != "27-rfp-1" {
			t.Errorf("vencida que sigue como candidata: %s", c.Found.Reference)
		}
	}
	if txt, ok := granicusText(string(raw)); !ok || !strings.Contains(txt, "27-F-050") {
		t.Errorf("granicusText: %v %q", ok, txt)
	}
}

func TestLevelPorEntidad(t *testing.T) {
	rus := &models.CrawlDomain{Domain: "subastas.pr.gov", Level: "estatal"}
	caguas := &models.CrawlDomain{Domain: "caguas.gov.pr", Level: "municipal"}
	fed := &models.CrawlDomain{Domain: "grants.gov", Level: "federal"}
	tests := []struct {
		dom    *models.CrawlDomain
		agency string
		want   string
	}{
		{rus, "Municipio de Aguadilla", "municipal"},
		{rus, "Municipio Autónomo de Guaynabo", "municipal"},
		{rus, "Gobierno Municipal de Ponce", "municipal"},
		{rus, "Autoridad de Acueductos y Alcantarillados (AAA)", "estatal"},
		{caguas, "Autoridad de Energía Eléctrica", "estatal"},
		{caguas, "Departamento de Educación (DE)", "estatal"},
		{caguas, "Administración de Servicios Generales (ASG)", "estatal"},
		{caguas, "Municipio Autónomo de Caguas", "municipal"},
		{caguas, "", "municipal"},
		{rus, "Compañía de Turismo", "estatal"},
		{rus, "Fundación Comunitaria", "estatal"},
		{fed, "Municipio de San Juan", "federal"},
		{nil, "Municipio de Ponce", "municipal"},
		{nil, "", "estatal"},
	}
	for _, tt := range tests {
		if got := levelFor(tt.dom, tt.agency); got != tt.want {
			t.Errorf("levelFor(%q) = %q, quería %q", tt.agency, got, tt.want)
		}
	}
}

func TestClassifyPDFLink(t *testing.T) {
	type c struct {
		anchor, url string
		download    bool
		call        bool
	}
	tests := []c{
		{"Aviso de Subasta 2027-016", "https://caguas.gov.pr/wp-content/uploads/2026/10/Aviso-Subasta-2027_016.pdf", true, true},
		{"Aviso", "https://caguas.gov.pr/wp-content/uploads/2026/10/Aviso-RFP-S-2027_018.pdf", true, true},
		{"Documento", "https://ejemplo.pr.gov/docs/archivo.pdf", true, false},
		{"Reglamento de Subastas", "https://x.pr.gov/reglamento-subastas.pdf", false, false},
		{"Adjudicación de Subasta 5", "https://x.pr.gov/adj.pdf", false, false},
		{"Ordenanza Núm. 17 – Serie 2026-2027", "https://www.municipiodebayamon.com/wp-content/uploads/2026/10/pub-ordenanza-17.pdf", false, false},
		{"Informe Anual 2025", "https://x.pr.gov/informe-anual-2025.pdf", false, false},
		{"Presupuesto aprobado", "https://x.pr.gov/presupuesto.pdf", false, false},
		{"Acta de la reunión de junta", "https://x.pr.gov/acta.pdf", false, false},
		{"Organigrama", "https://x.pr.gov/organigrama.pdf", false, false},
		{"Solicitud de empleo", "https://x.pr.gov/empleo.pdf", false, false},
		{"Comunicado de prensa", "https://x.pr.gov/comunicado.pdf", false, false},
		{"Formulario de registro de licitadores", "https://x.pr.gov/form.pdf", false, false},
		{"Subasta 4 conforme a la Ley 107-2020", "https://x.pr.gov/s4.pdf", true, true},
		{"Aviso viejo", "https://x.pr.gov/wp-content/uploads/2023/02/aviso-subasta.pdf", false, false},
		{"Ley 107-2020 Código Municipal", "https://x.pr.gov/ley-107.pdf", false, false},
		{"Solicitud de Propuesta SP-2026-27-04", "https://x.pr.gov/sol-propuesta-sp2026-27-04.pdf", true, true},
	}
	for _, tt := range tests {
		v := ClassifyPDFLink(tt.anchor, tt.url, nov16)
		if v.Download != tt.download || (tt.download && v.LooksCall != tt.call) {
			t.Errorf("%q: Download=%v LooksCall=%v (%s), quería %v/%v", tt.anchor, v.Download, v.LooksCall, v.Reason, tt.download, tt.call)
		}
		if v.Download {
			want := maxPDFBytes
			if v.LooksCall {
				want = maxCallPDFBytes
			}
			if v.MaxBytes != want {
				t.Errorf("%q: tope %d, quería %d", tt.anchor, v.MaxBytes, want)
			}
		}
	}
}

func TestProbePDFUnchanged(t *testing.T) {
	cases := []struct {
		p        PDFProbe
		etag, lm string
		size     int64
		want     bool
	}{
		{PDFProbe{ETag: `"a"`, ContentLength: 10}, `"a"`, "", 10, true},
		{PDFProbe{ETag: `"b"`, ContentLength: 10}, `"a"`, "", 10, false},
		{PDFProbe{LastModified: "x", ContentLength: 10}, "", "x", 10, true},
		{PDFProbe{LastModified: "y", ContentLength: 10}, "", "x", 10, false},
		{PDFProbe{LastModified: "x", ContentLength: 11}, "", "x", 10, false},
		{PDFProbe{ContentLength: 10}, "", "", 10, true},
		{PDFProbe{ContentLength: 11}, "", "", 10, false},
		{PDFProbe{ContentLength: -1}, "", "", 10, false},
	}
	for i, c := range cases {
		if got := c.p.Unchanged(c.etag, c.lm, c.size); got != c.want {
			t.Errorf("caso %d: %v, quería %v", i, got, c.want)
		}
	}
}

func TestFetchPDFTope(t *testing.T) {
	pdf := append([]byte("%PDF-1.4\n"), make([]byte, 2<<20)...)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("ETag", `"v1"`)
		_, _ = w.Write(pdf)
	}))
	defer srv.Close()
	res, err := FetchPDF(context.Background(), srv.URL+"/a.pdf", 1<<20)
	if err == nil || !strings.Contains(err.Error(), "demasiado grande") || res.Body != nil {
		t.Fatalf("con tope de 1 MB debía rechazar: err=%v", err)
	}
	res, err = FetchPDF(context.Background(), srv.URL+"/a.pdf", 4<<20)
	if err != nil || !res.IsPDF || res.ETag != `"v1"` || len(res.Body) != len(pdf) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}
