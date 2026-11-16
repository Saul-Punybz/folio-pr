package archivo

import (
	"strings"
	"testing"
	"time"
)

func fecha(s string) *time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		panic(err)
	}
	return &t
}

func TestAPA(t *testing.T) {
	d := fecha("2025-10-19T15:00:00Z")
	tests := []struct {
		name      string
		m         Meta
		plain, md string
	}{
		{
			name:  "un autor, medio en español",
			m:     Meta{Kind: KindArticulo, Title: "Celebrarán feria de empleo este viernes en San Juan", Site: "El Nuevo Día", Authors: []string{"Itzel Rivera Rivera"}, Published: d, URL: "https://www.elnuevodia.com/x/"},
			plain: "Rivera Rivera, I. (2025, 19 de octubre). Celebrarán feria de empleo este viernes en San Juan. El Nuevo Día. https://www.elnuevodia.com/x/",
			md:    "Rivera Rivera, I. (2025, 19 de octubre). Celebrarán feria de empleo este viernes en San Juan. *El Nuevo Día*. https://www.elnuevodia.com/x/",
		},
		{
			name:  "dos apellidos hispanos",
			m:     Meta{Kind: KindArticulo, Title: "Título", Site: "Medio", Authors: []string{"Beatriz Rivera Batista"}, Published: d, URL: "https://a.pr/1"},
			plain: "Rivera Batista, B. (2025, 19 de octubre). Título. Medio. https://a.pr/1",
			md:    "Rivera Batista, B. (2025, 19 de octubre). Título. *Medio*. https://a.pr/1",
		},
		{
			name:  "dos autores",
			m:     Meta{Kind: KindArticulo, Title: "Título", Site: "Medio", Authors: []string{"Beatriz Rivera Batista", "José Luis Pérez"}, Published: d, URL: "https://a.pr/1"},
			plain: "Rivera Batista, B. y Pérez, J. L. (2025, 19 de octubre). Título. Medio. https://a.pr/1",
		},
		{
			name:  "tres autores, partícula y nombre compuesto",
			m:     Meta{Kind: KindArticulo, Title: "Título", Site: "Medio", Authors: []string{"Beatriz Rivera Batista", "José Luis Pérez", "Carlos de la Cruz"}, Published: d, URL: "https://a.pr/1"},
			plain: "Rivera Batista, B., Pérez, J. L. y de la Cruz, C. (2025, 19 de octubre). Título. Medio. https://a.pr/1",
		},
		{
			name:  "nombre y dos apellidos con un solo nombre",
			m:     Meta{Kind: KindArticulo, Title: "T", Site: "M", Authors: []string{"Gabriel García Márquez"}, Published: d, URL: "https://a.pr/1"},
			plain: "García Márquez, G. (2025, 19 de octubre). T. M. https://a.pr/1",
		},
		{
			name:  "nombre compuesto con guion e inicial",
			m:     Meta{Kind: KindArticulo, Title: "T", Site: "M", Authors: []string{"Jean-Pierre Dupont", "John R. Smith"}, Published: d, URL: "https://a.pr/1"},
			plain: "Dupont, J.-P. y Smith, J. R. (2025, 19 de octubre). T. M. https://a.pr/1",
		},
		{
			name:  "autor escrito «Apellido, Nombre»",
			m:     Meta{Kind: KindArticulo, Title: "T", Site: "M", Authors: []string{"Rivera Batista, Beatriz"}, Published: d, URL: "https://a.pr/1"},
			plain: "Rivera Batista, B. (2025, 19 de octubre). T. M. https://a.pr/1",
		},
		{
			name:  "sin autor: el título pasa al frente",
			m:     Meta{Kind: KindArticulo, Title: "Project Makers promueve negocios comunitarios", Site: "News is My Business", Published: d, URL: "https://nimb.com/a"},
			plain: "Project Makers promueve negocios comunitarios. (2025, 19 de octubre). News is My Business. https://nimb.com/a",
			md:    "Project Makers promueve negocios comunitarios. (2025, 19 de octubre). *News is My Business*. https://nimb.com/a",
		},
		{
			name:  "sin fecha",
			m:     Meta{Kind: KindArticulo, Title: "Título", Site: "Medio", Authors: []string{"Ana Pérez"}, URL: "https://a.pr/1"},
			plain: "Pérez, A. (s. f.). Título. Medio. https://a.pr/1",
		},
		{
			name:  "sin autor y sin fecha",
			m:     Meta{Kind: KindArticulo, Title: "Título", Site: "Medio", URL: "https://a.pr/1"},
			plain: "Título. (s. f.). Medio. https://a.pr/1",
		},
		{
			name:  "título con signo de pregunta no lleva punto extra",
			m:     Meta{Kind: KindArticulo, Title: "¿Cómo nace un negocio?", Site: "Medio", Authors: []string{"Ana Pérez"}, Published: d, URL: "https://a.pr/1"},
			plain: "Pérez, A. (2025, 19 de octubre). ¿Cómo nace un negocio? Medio. https://a.pr/1",
		},
		{
			name:  "autor que es el mismo medio no repite el medio",
			m:     Meta{Kind: KindArticulo, Title: "Título", Site: "Periódico Presencia", Authors: []string{"Periódico Presencia"}, Published: d, URL: "https://a.pr/1"},
			plain: "Periódico Presencia. (2025, 19 de octubre). Título. https://a.pr/1",
		},
		{
			name:  "autor institucional queda tal cual",
			m:     Meta{Kind: KindArticulo, Title: "Título", Site: "Periódico Presencia", Authors: []string{"Redacción Presencia"}, Published: d, URL: "https://a.pr/1"},
			plain: "Redacción Presencia. (2025, 19 de octubre). Título. Periódico Presencia. https://a.pr/1",
		},
		{
			name:  "medio en inglés",
			m:     Meta{Kind: KindArticulo, Title: "Puerto Rico's debt plan moves forward", Site: "The New York Times", Authors: []string{"Mary Smith", "John Doe"}, Published: d, URL: "https://nytimes.com/a"},
			plain: "Smith, M. y Doe, J. (2025, 19 de octubre). Puerto Rico's debt plan moves forward. The New York Times. https://nytimes.com/a",
			md:    "Smith, M. y Doe, J. (2025, 19 de octubre). Puerto Rico's debt plan moves forward. *The New York Times*. https://nytimes.com/a",
		},
		{
			name:  "la fecha se cuenta en hora de Puerto Rico",
			m:     Meta{Kind: KindArticulo, Title: "T", Site: "M", Published: fecha("2025-10-20T02:30:00Z"), URL: "https://a.pr/1"},
			plain: "T. (2025, 19 de octubre). M. https://a.pr/1",
		},
		{
			name: "post de Bluesky: primeras 20 palabras",
			m: Meta{Kind: KindPost, Platform: "Bluesky", Authors: []string{"Project Makers"}, Published: d,
				Text: "Hoy celebramos a los negocios de nuestra comunidad que cada día abren sus puertas con esfuerzo y mucho amor por Puerto Rico y su gente trabajadora, gracias a todos",
				URL:  "https://bsky.app/profile/projectmakerspr.bsky.social/post/3abc"},
			plain: "Project Makers [@projectmakerspr.bsky.social]. (2025, 19 de octubre). Hoy celebramos a los negocios de nuestra comunidad que cada día abren sus puertas con esfuerzo y mucho amor por… [Publicación]. Bluesky. https://bsky.app/profile/projectmakerspr.bsky.social/post/3abc",
			md:    "Project Makers [@projectmakerspr.bsky.social]. (2025, 19 de octubre). *Hoy celebramos a los negocios de nuestra comunidad que cada día abren sus puertas con esfuerzo y mucho amor por…* [Publicación]. Bluesky. https://bsky.app/profile/projectmakerspr.bsky.social/post/3abc",
		},
		{
			name:  "post de Mastodon corto",
			m:     Meta{Kind: KindPost, Platform: "Mastodon", Authors: []string{"Vimenti"}, Published: d, Text: "Feria de empleo este viernes", URL: "https://mastodon.social/@vimenti/1234"},
			plain: "@vimenti. (2025, 19 de octubre). Feria de empleo este viernes [Publicación]. Mastodon. https://mastodon.social/@vimenti/1234",
		},
		{
			name:  "episodio de podcast sin anfitrión",
			m:     Meta{Kind: KindPodcast, Platform: "Apple Podcasts", Title: "Emprender en la isla", Site: "Voces de PR", Published: d, URL: "https://podcasts.apple.com/us/podcast/x/id1?i=2"},
			plain: "Emprender en la isla [Episodio de pódcast]. (2025, 19 de octubre). En Voces de PR. Apple Podcasts. https://podcasts.apple.com/us/podcast/x/id1?i=2",
			md:    "Emprender en la isla [Episodio de pódcast]. (2025, 19 de octubre). En *Voces de PR*. Apple Podcasts. https://podcasts.apple.com/us/podcast/x/id1?i=2",
		},
		{
			name:  "episodio de podcast con anfitrión",
			m:     Meta{Kind: KindPodcast, Platform: "Apple Podcasts", Title: "Emprender en la isla", Site: "Voces de PR", Authors: []string{"Beatriz Rivera Batista"}, Published: d, URL: "https://podcasts.apple.com/x"},
			plain: "Rivera Batista, B. (2025, 19 de octubre). Emprender en la isla [Episodio de pódcast]. En Voces de PR. Apple Podcasts. https://podcasts.apple.com/x",
		},
		{
			name:  "video de YouTube",
			m:     Meta{Kind: KindVideo, Platform: "YouTube", Title: "Historia de éxito: Libros 787", Authors: []string{"Project Makers PR"}, Published: d, URL: "https://www.youtube.com/watch?v=abc"},
			plain: "Project Makers PR. (2025, 19 de octubre). Historia de éxito: Libros 787 [Video]. YouTube. https://www.youtube.com/watch?v=abc",
			md:    "Project Makers PR. (2025, 19 de octubre). *Historia de éxito: Libros 787* [Video]. YouTube. https://www.youtube.com/watch?v=abc",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := APA(tc.m)
			if got.Plain != tc.plain {
				t.Errorf("plain:\n got %s\nwant %s", got.Plain, tc.plain)
			}
			if tc.md != "" && got.Markdown != tc.md {
				t.Errorf("markdown:\n got %s\nwant %s", got.Markdown, tc.md)
			}
			if strings.Contains(got.Plain, "*") {
				t.Errorf("el texto plano no lleva cursiva: %s", got.Plain)
			}
		})
	}
}

func TestOrdenarAlfabetico(t *testing.T) {
	es := []Entrada{
		{Cita: Cita{Plain: "Zeta, A. (2025, 1 de enero). X. M. u"}},
		{Cita: Cita{Plain: "Álvarez, B. (2025, 1 de enero). X. M. u"}},
		{Cita: Cita{Plain: "Arroyo, C. (2025, 1 de enero). X. M. u"}},
		{Cita: Cita{Plain: "¿Cómo nace? (s. f.). M. u"}},
	}
	Ordenar(es)
	var got []string
	for _, e := range es {
		got = append(got, string([]rune(e.Cita.Plain)[:3]))
	}
	want := []string{"Álv", "Arr", "¿Có", "Zet"}
	// «¿Cómo» ordena como «como»: entre Arroyo y Zeta.
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("orden = %v, want %v", got, want)
		}
	}
}

func TestExportarCSVYMarkdown(t *testing.T) {
	f := []Fila{{Titulo: "=SUMA(1)", Medio: "Medio, S.A.", Autores: "Ana Pérez", Fecha: "2025-10-19", URL: "https://a.pr/1", Resumen: "Dijo \"hola\".\nOtra línea.", Cita: Cita{Plain: "p", Markdown: "*m*"}}}
	b, err := CSV(f)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	if !strings.HasPrefix(s, "\xef\xbb\xbf") || !strings.Contains(s, "'=SUMA(1)") || !strings.Contains(s, `"Medio, S.A."`) || !strings.Contains(s, `"Dijo ""hola"".`) {
		t.Fatalf("CSV mal: %q", s)
	}
	md := Markdown("Archivo de Vimenti", f)
	if !strings.Contains(md, "# Archivo de Vimenti") || !strings.Contains(md, "- Cita APA: *m*") {
		t.Fatalf("Markdown mal: %s", md)
	}
}
