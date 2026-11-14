package crawler

import (
	"testing"
	"time"
)

func TestDedupeKey(t *testing.T) {
	d := func(s string) *time.Time { x, _ := time.ParseInLocation("2006-01-02", s, ast); return &x }
	type in struct {
		ref, agency, title string
		deadline           *time.Time
		url                string
	}
	tests := []struct {
		name string
		a, b in
		same bool
		want string // si no está vacío, la huella exacta de a
	}{
		{
			name: "misma referencia con ceros y prefijos distintos, misma agencia",
			a:    in{ref: "RFP 2026-001", agency: "Consejo Estatal sobre Deficiencias en el Desarrollo", url: "https://cedd.pr.gov/rfps"},
			b:    in{ref: "RFP 2026-1", agency: "Consejo Estatal sobre Deficiencias en el Desarrollo", url: "https://cedd.pr.gov/media/rfp-2026-001.pdf"},
			same: true,
			want: "ref:rfp-2026-1|consejo estatal sobre deficiencias en el desarrollo",
		},
		{
			name: "listado y ficha del RUS: la sigla entre paréntesis manda",
			a:    in{ref: "27-SP-019", agency: "Autoridad de Acueductos y Alcantarillados (AAA)", url: "https://subastas.pr.gov/Pages/aviso.aspx?itemID=13437"},
			b:    in{ref: "(27-SP-019)", agency: "Autoridad de Acueductos y Alcantarillados (AAA) ", url: "https://subastas.pr.gov/Pages/subastas.aspx"},
			same: true,
			want: "ref:27-sp-19|aaa",
		},
		{
			name: "palabras de relleno fuera: Subasta Formal Núm.",
			a:    in{ref: "Subasta Formal Núm. 2026-27-53", agency: "AEP"},
			b:    in{ref: "2026-27-53", agency: "aep"},
			same: true,
		},
		{
			name: "misma referencia, otra agencia: distintas",
			a:    in{ref: "RFP 2026-001", agency: "CEDD"},
			b:    in{ref: "RFP 2026-001", agency: "Vivienda"},
		},
		{
			name: "sin referencia: título y fecha (acentos y mayúsculas no importan)",
			a:    in{title: "Becas de Verano 2027", deadline: d("2026-12-01"), url: "https://a.pr.gov/1"},
			b:    in{title: "BECAS DE VERANO 2027", deadline: d("2026-12-01"), url: "https://a.pr.gov/2"},
			same: true,
			want: "td:becas de verano 2027|2026-12-01",
		},
		{
			name: "sin referencia: misma fecha, otro título",
			a:    in{title: "Becas de verano", deadline: d("2026-12-01")},
			b:    in{title: "Becas de invierno", deadline: d("2026-12-01")},
		},
		{
			name: "solo URL: la canónica (sin fragmento ni barra final)",
			a:    in{url: "https://Caguas.gov.pr/wp-content/uploads/Aviso.pdf#page=2"},
			b:    in{url: "https://caguas.gov.pr/wp-content/uploads/Aviso.pdf"},
			same: true,
			want: "url:https://caguas.gov.pr/wp-content/uploads/Aviso.pdf",
		},
		{
			name: "la misma página con ?tab= distintos y sin fecha: una sola",
			a:    in{title: "Programa para PyMEs del Casco Urbano", agency: "DDEC", url: "https://www.desarrollo.pr.gov/ayudas-e-incentivos?tab=pymes"},
			b:    in{title: "Programa para PyMEs del Casco Urbano", agency: "DDEC", url: "https://www.desarrollo.pr.gov/ayudas-e-incentivos?tab=jovenes"},
			same: true,
			want: "ta:programa para pymes del casco urbano|ddec",
		},
		{
			name: "misma página sin enlace por fila: el título las separa",
			a:    in{title: "Subasta de asfalto", url: "https://m.gov.pr/avisos"},
			b:    in{title: "Subasta de luminarias", url: "https://m.gov.pr/avisos"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ka := DedupeKey(tt.a.ref, tt.a.agency, tt.a.title, tt.a.deadline, tt.a.url)
			kb := DedupeKey(tt.b.ref, tt.b.agency, tt.b.title, tt.b.deadline, tt.b.url)
			if (ka == kb) != tt.same {
				t.Errorf("a=%q b=%q, iguales=%v quería %v", ka, kb, ka == kb, tt.same)
			}
			if tt.want != "" && ka != tt.want {
				t.Errorf("huella %q, quería %q", ka, tt.want)
			}
		})
	}
}
