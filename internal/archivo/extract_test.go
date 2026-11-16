package archivo

import (
	"os"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Páginas reales (recortadas) de tres medios que salieron en la corrida.
func TestParsePageMedios(t *testing.T) {
	tests := []struct {
		file, url string
		title     string
		site      string
		authors   []string
		date      string // YYYY-MM-DD en UTC
		lang      string
		canon     string
		textHas   string
		minText   int
	}{
		{
			file: "elnuevodia.html", url: "https://www.elnuevodia.com/negocios/x/",
			title: "Celebrarán feria de empleo este viernes en San Juan", site: "El Nuevo Día",
			authors: []string{"Itzel Rivera"}, date: "2026-11-04", lang: "es",
			canon:   "https://www.elnuevodia.com/negocios/empresas-comercios/notas/celebraran-feria-de-empleo-este-viernes-en-san-juan/",
			textHas: "segunda feria de empleo de 2026", minText: 800,
		},
		{
			file: "presencia.html", url: "https://presenciapr.com/vimenti-celebrara-segunda-feria-de-empleo-en-san-juan/",
			title: "Vimenti celebrará segunda feria de empleo en San Juan", site: "Periódico Presencia",
			authors: []string{"Redacción Presencia"}, date: "2026-11-03", lang: "es",
			canon:   "https://presenciapr.com/vimenti-celebrara-segunda-feria-de-empleo-en-san-juan/",
			textHas: "Residencial Ernesto Ramos Antonini", minText: 800,
		},
		{
			file: "projectmakers.html", url: "https://www.projectmakerspr.org/post/negocio-del-mes-libros-787",
			title: "Negocio del mes: Libros 787", site: "Project Makers",
			authors: []string{"Jorge R. Herrera"}, date: "2026-02-09", lang: "es",
			canon:   "https://www.projectmakerspr.org/post/negocio-del-mes-libros-787",
			textHas: "Entrevistamos a Libros 787", minText: 800,
		},
	}
	for _, tc := range tests {
		t.Run(tc.file, func(t *testing.T) {
			p := ParsePage(fixture(t, tc.file), tc.url)
			if p.Title != tc.title || p.Site != tc.site || p.Lang != tc.lang || p.Canonical != tc.canon {
				t.Errorf("metadatos: title=%q site=%q lang=%q canon=%q", p.Title, p.Site, p.Lang, p.Canonical)
			}
			if strings.Join(p.Authors, "|") != strings.Join(tc.authors, "|") {
				t.Errorf("autores = %q, want %q", p.Authors, tc.authors)
			}
			if p.Published == nil || p.Published.Format("2006-01-02") != tc.date {
				t.Errorf("fecha = %v, want %s", p.Published, tc.date)
			}
			if !strings.HasPrefix(p.Image, "https://") {
				t.Errorf("imagen = %q", p.Image)
			}
			if !strings.Contains(p.Text, tc.textHas) || len(p.Text) < tc.minText {
				t.Errorf("texto (%d): falta %q", len(p.Text), tc.textHas)
			}
			for _, ruido := range []string{"Todos los derechos reservados", "Inicio Negocios"} {
				if strings.Contains(p.Text, ruido) {
					t.Errorf("el texto trae menú o pie: %q", ruido)
				}
			}
		})
	}
}

func TestResumenPorReglas(t *testing.T) {
	end := ParsePage(fixture(t, "elnuevodia.html"), "https://www.elnuevodia.com/x")
	// Descripción corta, sin sustancia aparte del título: salen los párrafos.
	if got := Resumen("Corta.", end.Text, end.Title); !strings.Contains(got, "segunda feria de empleo") || len([]rune(got)) > ResumenMaxRunes {
		t.Errorf("resumen de párrafos: %q", got)
	}
	// Con sustancia, manda la descripción.
	pp := ParsePage(fixture(t, "presencia.html"), "https://presenciapr.com/x")
	if got := Resumen(pp.Description, pp.Text, pp.Title); got != pp.Description {
		t.Errorf("debía usar la descripción: %q", got)
	}
	// Wix corta su descripción a media frase: se prefiere el texto.
	pm := ParsePage(fixture(t, "projectmakers.html"), "https://www.projectmakerspr.org/post/x")
	got := Resumen(pm.Description, pm.Text, pm.Title)
	if strings.Contains(got, "qué te") || !strings.HasSuffix(got, ".") || len([]rune(got)) > ResumenMaxRunes {
		t.Errorf("resumen Wix: %q", got)
	}
}

func TestCortarPorOracion(t *testing.T) {
	s := "El Dr. Pérez dijo que J. Rivera llegó a las 5 p.m. del jueves. " + strings.Repeat("Otra oración con varias palabras para llenar el espacio disponible. ", 20)
	got := CortarPorOracion(s, 200)
	if len([]rune(got)) > 200 || !strings.HasPrefix(got, "El Dr. Pérez dijo que J. Rivera llegó a las 5 p.m. del jueves.") || !strings.HasSuffix(got, ".") {
		t.Errorf("corte por oración: %q", got)
	}
	// Una sola oración larguísima: corta por palabra, con «…» y sin partir runas.
	long := strings.Repeat("ñandú ", 300)
	got = CortarPorOracion(long, 50)
	if len([]rune(got)) > 50 || !strings.HasSuffix(got, "…") {
		t.Errorf("corte por palabra: %q", got)
	}
}

func TestParseDate(t *testing.T) {
	for in, want := range map[string]string{
		"2026-02-09T17:14:14.064Z":  "2026-02-09",
		"2026-11-03T18:18:56+00:00": "2026-11-03",
		"2026-11-03":                "2026-11-03",
	} {
		if p := ParseDate(in); p == nil || p.Format("2006-01-02") != want {
			t.Errorf("ParseDate(%q) = %v", in, p)
		}
	}
	if ParseDate("mañana") != nil || ParseDate("1970-01-01") != nil {
		t.Error("fechas inválidas deben dar nil")
	}
}
