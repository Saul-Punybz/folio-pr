package crawler

import (
	"os"
	"strings"
	"testing"
	"time"
)

var ast = time.FixedZone("AST", -4*3600)

// HTML real (recortado) del Registro Único de Subastas: seis filas, tres
// abiertas con apertura en 2027, una adjudicada, una cancelada y una
// "Abierta" de 2024 que ya venció.
func TestSplitRUSListado(t *testing.T) {
	raw, err := os.ReadFile("testdata/rus_listado.html")
	if err != nil {
		t.Fatal(err)
	}
	const pageURL = "https://subastas.pr.gov/Pages/subastas.aspx"
	rows := SplitHTMLRows(string(raw), pageURL)
	if len(rows) != 6 {
		for _, r := range rows {
			t.Logf("fila: %.80q", r.Text)
		}
		t.Fatalf("filas = %d, se esperaban 6 (el menú <nav> no cuenta)", len(rows))
	}

	now := time.Date(2026, 11, 14, 9, 0, 0, 0, ast)
	got := RowCandidates(rows, "Registro Único de Subastas", now)

	want := []struct {
		ref, deadline, link, titleHas string
	}{
		{"27-SP-019", "2027-04-06", "https://subastas.pr.gov/Pages/aviso.aspx?itemID=13437", "SAN SEBASTIAN REPLACEMENT"},
		{"27-SP-018", "2027-03-11", "https://subastas.pr.gov/Pages/aviso.aspx?itemID=13436", "EL YUNQUE"},
		{"27-SP-017", "2027-03-09", "https://subastas.pr.gov/Pages/aviso.aspx?itemID=13435", "INDIERA ALTA"},
	}
	if len(got) != len(want) {
		for _, c := range got {
			t.Logf("candidata: ref=%q dl=%v", c.Found.Reference, c.Found.Deadline)
		}
		t.Fatalf("candidatas = %d, se esperaban %d (adjudicada, cancelada y vencida fuera)", len(got), len(want))
	}
	for i, w := range want {
		c := got[i]
		if c.Found.Reference != w.ref {
			t.Errorf("fila %d: referencia %q, quería %q", i, c.Found.Reference, w.ref)
		}
		if c.Found.Deadline == nil || c.Found.Deadline.Format("2006-01-02") != w.deadline {
			t.Errorf("fila %d: fecha límite %v, quería %s", i, c.Found.Deadline, w.deadline)
		}
		if c.Row.Link != w.link {
			t.Errorf("fila %d: enlace %q, quería %q", i, c.Row.Link, w.link)
		}
		if a := agencyFromText(c.Row.Text); a != "Autoridad de Acueductos y Alcantarillados (AAA)" {
			t.Errorf("fila %d: agencia %q", i, a)
		}
		title := c.Found.Title
		if title == "" {
			title = stripRefSuffix(c.Row.LinkText, c.Found.Reference)
		}
		if !strings.Contains(title, w.titleHas) || strings.Contains(title, "("+w.ref+")") {
			t.Errorf("fila %d: título %q (debe traer %q y no la referencia)", i, title, w.titleHas)
		}
		if strings.Contains(title, "Registro Único") {
			t.Errorf("fila %d: el contexto se volvió título: %q", i, title)
		}
	}
}

func TestSplitHTMLRows(t *testing.T) {
	long := strings.Repeat("x", maxRowRunes+1)
	tests := []struct {
		name  string
		html  string
		texts []string // prefijos esperados, en orden
		links []string
	}{
		{
			name:  "tabla: cada tr es una fila; el enlace se resuelve",
			html:  `<table><tr><td><a href="/a/1.pdf">Subasta 2027-001 compra de equipo</a></td><td>30 de noviembre de 2026</td></tr><tr><td><a href="b.html#x">RFP 2027-002 servicios profesionales</a></td><td>1 de diciembre de 2026</td></tr></table>`,
			texts: []string{"Subasta 2027-001 compra de equipo\n30 de noviembre de 2026", "RFP 2027-002 servicios profesionales\n1 de diciembre de 2026"},
			links: []string{"https://m.gov.pr/a/1.pdf", "https://m.gov.pr/avisos/b.html"},
		},
		{
			name:  "li anidado: solo cuenta la hoja",
			html:  `<ul><li>Grupo de avisos públicos del municipio<ul><li>Aviso de subasta 2027-010 asfalto</li><li>Aviso de subasta 2027-011 luminarias</li></ul></li></ul>`,
			texts: []string{"Aviso de subasta 2027-010 asfalto", "Aviso de subasta 2027-011 luminarias"},
			links: []string{"", ""},
		},
		{
			name:  "menú y pie fuera; filas cortas, largas y repetidas fuera",
			html:  `<nav><ul><li>Subastas y licitaciones del municipio de prueba</li></ul></nav><ul><li>corta</li><li>` + long + `</li><li>Convocatoria abierta para becas de verano</li><li>Convocatoria abierta para becas de verano</li></ul><footer><ul><li>Todos los derechos reservados al municipio</li></ul></footer>`,
			texts: []string{"Convocatoria abierta para becas de verano"},
			links: []string{""},
		},
		{
			name:  "article sin li ni tr; javascript: y mailto: no son enlace",
			html:  `<article><h2><a href="javascript:void(0)">Solicitud de propuestas 2026-7</a></h2><p>Fecha límite 3 de diciembre de 2026</p></article><article><a href="mailto:a@b.pr">Escriba a compras</a> sobre la subasta 2026-8</article>`,
			texts: []string{"Solicitud de propuestas 2026-7\nFecha límite 3 de diciembre de 2026", "Escriba a compras sobre la subasta 2026-8"},
			links: []string{"", ""},
		},
		{
			name: "sin filas",
			html: `<div><p>Página sin listados</p></div>`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := SplitHTMLRows("<html><body>"+tt.html+"</body></html>", "https://m.gov.pr/avisos/")
			if len(rows) != len(tt.texts) {
				for _, r := range rows {
					t.Logf("fila: %q", r.Text)
				}
				t.Fatalf("filas = %d, quería %d", len(rows), len(tt.texts))
			}
			for i, r := range rows {
				if r.Text != tt.texts[i] {
					t.Errorf("fila %d: %q, quería %q", i, r.Text, tt.texts[i])
				}
				if r.Link != tt.links[i] {
					t.Errorf("fila %d: enlace %q, quería %q", i, r.Link, tt.links[i])
				}
			}
		})
	}
}

func TestSplitTextBlocks(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"un solo aviso: no se parte", "AVISO DE SUBASTA 2027-016\nFecha límite 30 de noviembre de 2026", nil},
		{"sin arranques", "Texto suelto\nsin convocatorias", nil},
		{
			"PDF con tres subastas y encabezado",
			"MUNICIPIO DE PRUEBA\nAvisos del mes\nSubasta Núm. 2027-001 Asfalto\nApertura 1 de diciembre de 2026\nSubasta Núm. 2027-002 Luminarias\nApertura 2 de diciembre de 2026\nRFP 2027-003 Auditoría\nFecha límite 3 de diciembre de 2026",
			[]string{
				"Subasta Núm. 2027-001 Asfalto\nApertura 1 de diciembre de 2026",
				"Subasta Núm. 2027-002 Luminarias\nApertura 2 de diciembre de 2026",
				"RFP 2027-003 Auditoría\nFecha límite 3 de diciembre de 2026",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitTextBlocks(tt.in)
			if len(got) != len(tt.want) {
				t.Fatalf("bloques = %d (%q), quería %d", len(got), got, len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("bloque %d: %q, quería %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestScannedHintAndCleanTitle(t *testing.T) {
	tests := []struct {
		anchor string
		ok     bool
	}{
		{"Solicitud de Propuestas de Servicios Profesionales", true},
		{"RFP  Feasibility Study - New Port Cataño and Toa Baja", true},
		{"Descargar Addendum 7 de DV RFP 2026-11", false},
		{"Descargar Notificación de Adjudicación de DV RFP 2026-11", false},
		{"Ver Reglamento", false},
		{"View Documents", false},
	}
	for _, tt := range tests {
		h := fold(tt.anchor)
		if got := convocatoriaHint.MatchString(h) && !notNewCall.MatchString(h); got != tt.ok {
			t.Errorf("%q: %v, quería %v", tt.anchor, got, tt.ok)
		}
	}
	if got := cleanTitle("Solicitud  de\tPropuestas "); got != "Solicitud de Propuestas" {
		t.Errorf("cleanTitle = %q", got)
	}
}
