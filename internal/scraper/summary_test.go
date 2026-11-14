package scraper

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateWords(t *testing.T) {
	tests := []struct {
		name string
		in   string
		max  int
		want string
	}{
		{"corto no cambia", "Año de la niñez", 50, "Año de la niñez"},
		{"exacto no cambia", "canción", 7, "canción"},
		{"corta por palabra", "El gobernador anunció fondos", 20, "El gobernador…"},
		{"no parte la ñ", "mañana mañana mañana", 9, "mañana…"},
		{"corte justo en espacio", "uno dos tres", 5, "uno…"},
		{"palabra única larga corta por runas", "Superextraordinariamente", 6, "Super…"},
		{"quita coma final", "Ponce, Mayagüez, Caguas", 10, "Ponce…"},
		{"espacios alrededor", "  hola  ", 10, "hola"},
		{"max cero", "hola", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncateWords(tt.in, tt.max)
			if got != tt.want {
				t.Fatalf("TruncateWords(%q, %d) = %q, want %q", tt.in, tt.max, got, tt.want)
			}
			if !utf8.ValidString(got) {
				t.Fatalf("resultado no es UTF-8 válido: %q", got)
			}
			if n := utf8.RuneCountInString(got); tt.max > 0 && n > tt.max {
				t.Fatalf("largo %d runas > max %d", n, tt.max)
			}
		})
	}
}

func TestRuleSummary(t *testing.T) {
	long := strings.Repeat("La Asociación de Pequeñas Empresas recibió fondos. ", 20)
	tests := []struct {
		name       string
		candidates []string
		want       string
	}{
		{"primer candidato no vacío", []string{"", "  ", "Descripción del medio."}, "Descripción del medio."},
		{"quita HTML y entidades", []string{"<p>Niños &amp; jóvenes</p><img src='x.jpg'>"}, "Niños & jóvenes"},
		{"colapsa espacios", []string{"uno\n\n  dos\ttres"}, "uno dos tres"},
		{"nada", []string{"", "<br/>"}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := RuleSummary(tt.candidates...); got != tt.want {
				t.Fatalf("RuleSummary = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("recorta a 300 runas por palabra", func(t *testing.T) {
		got := RuleSummary(long)
		if n := utf8.RuneCountInString(got); n > SummaryMaxRunes {
			t.Fatalf("largo %d > %d", n, SummaryMaxRunes)
		}
		if !strings.HasSuffix(got, "…") {
			t.Fatalf("falta el «…»: %q", got)
		}
		body := strings.TrimSuffix(got, "…")
		if !strings.HasPrefix(long, body) {
			t.Fatalf("no es prefijo del original: %q", body)
		}
		if next := long[len(body)]; next != ' ' && next != '.' {
			t.Fatalf("cortó a mitad de palabra: …%q|%q", body[len(body)-10:], long[len(body):len(body)+5])
		}
	})
}

func TestFirstParagraph(t *testing.T) {
	text := "Por Redacción\n13 de noviembre\n\nEl Departamento de Educación abrió una convocatoria de fondos para organizaciones sin fines de lucro.\nSegundo párrafo."
	want := "El Departamento de Educación abrió una convocatoria de fondos para organizaciones sin fines de lucro."
	if got := FirstParagraph(text); got != want {
		t.Fatalf("FirstParagraph = %q, want %q", got, want)
	}
	if got := FirstParagraph("corto\nmás cortito"); got != "más cortito" {
		t.Fatalf("sin párrafo largo debe devolver el más largo, got %q", got)
	}
}

func TestMetaDescription(t *testing.T) {
	html := `<head><meta name="description" content="Resumen del sitio"></head>`
	if got := MetaDescription(html); got != "Resumen del sitio" {
		t.Fatalf("got %q", got)
	}
	og := `<meta property="og:description" content="OG gana">` + html
	if got := MetaDescription(og); got != "OG gana" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeTags(t *testing.T) {
	got := NormalizeTags([]string{" Política ", "política", "", "<b>Economía</b>", "Salud"})
	want := []string{"Política", "Economía", "Salud"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v, want %v", got, want)
	}
	if NormalizeTags(nil) == nil {
		t.Fatal("NormalizeTags(nil) no debe ser nil")
	}
}

func TestIsPRRelevant(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"FEMA approves aid for Puerto Rico after hurricane", true},
		{"Puerto Rican nurses recruited in Florida", true},
		{"Nuevo hospital en Mayagüez", true},
		{"Fiscal board (junta de control fiscal) meets", true},
		{"El alcalde de Bayamon anuncia", true},
		{"Virginia, a Commonwealth, passes budget", false},
		{"Northern Territory elections", false},
		{"Statehood debate in DC", false},
		{"Federal Reserve raises rates", false},
		{"", false},
	}
	for _, tt := range tests {
		if got := isPRRelevant(tt.text); got != tt.want {
			t.Errorf("isPRRelevant(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestParseRSSCategories(t *testing.T) {
	data := []byte(`<?xml version="1.0"?><rss><channel><item><title>T</title><link>https://x.pr/a</link>
	<description>&lt;p&gt;Hola&lt;/p&gt;</description><category>Política</category><category>Gobierno</category></item></channel></rss>`)
	items, err := parseRSS(data)
	if err != nil || len(items) != 1 {
		t.Fatalf("parseRSS: %v %v", items, err)
	}
	if strings.Join(items[0].Categories, ",") != "Política,Gobierno" {
		t.Fatalf("categorías: %v", items[0].Categories)
	}
}

func TestCleanTextKeepsCase(t *testing.T) {
	got := CleanText("<P>Gobierno de Puerto Rico</P><BR/>San Juan")
	if !strings.Contains(got, "Gobierno de Puerto Rico") || !strings.Contains(got, "San Juan") {
		t.Fatalf("CleanText cambió mayúsculas o texto: %q", got)
	}
	if !strings.Contains(got, "\n") {
		t.Fatalf("CleanText no separó bloques: %q", got)
	}
}
