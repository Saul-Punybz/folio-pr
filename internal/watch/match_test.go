package watch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/Saul-Punybz/folio/internal/models"
)

func brand(name string) models.WatchlistOrg {
	for _, b := range BrandTemplates() {
		if b.Name == name {
			b.ID = uuid.New()
			return b
		}
	}
	panic("sin plantilla " + name)
}

func allBrands() []models.WatchlistOrg {
	var out []models.WatchlistOrg
	for _, b := range BrandTemplates() {
		b.ID = uuid.New()
		out = append(out, b)
	}
	return out
}

func TestMatchAliasesYExclusiones(t *testing.T) {
	m := Matcher{Context: brandContext(allBrands())}
	tests := []struct {
		org  string
		text string
		want bool
	}{
		// Oasis: el nombre suelto y la banda no entran.
		{"Oasis 360", "Oasis anuncia gira de reunión con Liam y Noel Gallagher", false},
		{"Oasis 360", "Oasis Hub reunion tour tickets", false},
		{"Oasis 360", "Wonderwall: Oasis Hub de fans", false},
		{"Oasis 360", "Oasis vuelve a los escenarios en 2026", false},
		{"Oasis 360", "Low-cost food pantry Oasis Hub Bath awarded grant", false},
		{"Oasis 360", "Oasis of the Seas llega a San Juan", false},
		{"Oasis 360", "Avanza la construcción del Oasis Hub en Villa Prades", true},
		{"Oasis 360", "PSI y el proyecto Oasis 360 en San Juan", true},
		{"Oasis 360", "Visita @oasis360_pr", true},
		// Vimenti: palabra entera, sin la proteína ni acentos raros.
		{"Vimenti", "Vimenti y Starbucks lanzan recaudación", true},
		{"Vimenti", "VIMENTI gradúa su primera clase", true},
		{"Vimenti", "vimentin expression in cancer cells", false},
		{"Vimenti", "Thank you Princess-vimenti for the follow", false},
		// PSI: «PSI» solo, únicamente con contexto.
		{"Platform for Social Impact (PSI)", "Tire pressure 32 PSI recommended", false},
		{"Platform for Social Impact (PSI)", "PSI anuncia inversión en Puerto Rico", true},
		{"Platform for Social Impact (PSI)", "PSI y Vimenti firman acuerdo", true},
		{"Platform for Social Impact (PSI)", "Population Services International (PSI) in Puerto Rico", false},
		{"Platform for Social Impact (PSI)", "Platform for Social Impact secures $43 million for a community center in Puerto Rico", true},
		{"Platform for Social Impact (PSI)", "Campground Raises $2.2 Million To Scale AI-Native Platform For Social Impact Programs", false},
		{"Platform for Social Impact (PSI)", "How Yara Shahidi Is Using Her Platform For Social Change", false},
		{"Platform for Social Impact (PSI)", "Platform for Social Change en San Juan", true},
		{"Platform for Social Impact (PSI)", "Síguelos en socialimpact.pr", true},
		{"Platform for Social Impact (PSI)", "Olimpiadas de la Fraternidad Phi Chi Psi en Guayanilla, Puerto Rico", false},
		{"Oasis 360", "OASIS HUB LLC in Mayaguez, PR - Bizapedia", false},
		{"Oasis 360", "About Us | Oasis360 Solutions", false},
		{"Project Makers", "Project Makers announces 2nd edition of Actívate 2025 event", true}, // en noticias basta; en redes globales pide contexto
		{"Platform for Social Impact (PSI)", "Hire our impact PR firm today", false},
		// Project Makers: alias de varias palabras.
		{"Project Makers", "Project Makers PR abre convocatoria StartUp", true},
		{"Project Makers", "Nada que ver con nadie", false},
	}
	for _, tt := range tests {
		_, got := m.Match(tt.text, brand(tt.org))
		if got != tt.want {
			t.Errorf("%s: Match(%q) = %v, want %v", tt.org, tt.text, got, tt.want)
		}
	}
}

// Error de la corrida real: un post de una agencia de relaciones públicas
// llamada «Impact PR» entró en PSI.
func TestImpactPRAgenciaNoEsPSI(t *testing.T) {
	psi := brand("Platform for Social Impact (PSI)")
	for _, m := range []Matcher{{Context: brandContext(allBrands())}, {Context: brandContext(allBrands()), NeedPR: true}} {
		for _, text := range []string{
			"Impact PR launches a new campaign for local brands in Puerto Rico",
			"Impact PR celebra su aniversario en San Juan con clientes y aliados",
			"Gracias a Impact PR por la cobertura del lanzamiento en Puerto Rico",
			"Impact PR is hiring a content creator",
		} {
			if _, ok := m.Match(text, psi); ok {
				t.Errorf("la agencia no debe entrar en PSI: %q", text)
			}
		}
		for _, text := range []string{
			"Impact PR y Platform for Social Impact firman acuerdo",
			"Impact PR, el sistema de Oasis, abre convocatoria",
			"Impact PR impulsa el impacto social en Puerto Rico",
			"Impact PR works on social impact in Puerto Rico",
			"PSI e Impact PR anuncian programa",
		} {
			if _, ok := m.Match(text, psi); !ok {
				t.Errorf("debía entrar en PSI: %q", text)
			}
		}
	}
}

func TestNeedPRParaRedesGlobales(t *testing.T) {
	m := Matcher{Context: brandContext(allBrands()), NeedPR: true}
	pm := brand("Project Makers")
	if _, ok := m.Match("Fan project makers deploy!", pm); ok {
		t.Error("«project makers» sin contexto no debe entrar en Bluesky")
	}
	if _, ok := m.Match("Project Makers gradúa emprendedores en San Juan", pm); !ok {
		t.Error("con San Juan debe entrar")
	}
	if _, ok := m.Match("Comisionado entrega a Vimenti una subvención", brand("Vimenti")); !ok {
		t.Error("un alias de una palabra no necesita contexto")
	}
}

func TestMatchesOrgTemas(t *testing.T) {
	org := models.WatchlistOrg{Name: "Fundación Ángel Ramos", Kind: models.KindTema, Keywords: []string{"FAR", "x", " "}}
	tests := []struct {
		text string
		want bool
	}{
		{"La FUNDACIÓN ÁNGEL RAMOS dona", true},
		{"La fundacion angel ramos dona", true}, // sin acentos también
		{"Premio de la far en Ponce", true},
		{"x marca el lugar", false}, // palabras clave de 1 letra se ignoran
		{"Un faro en Ponce", false}, // palabra entera
		{"Nada que ver", false},
	}
	for _, tt := range tests {
		if got := MatchesOrg(tt.text, org); got != tt.want {
			t.Errorf("MatchesOrg(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}

func TestHashtags(t *testing.T) {
	v := brand("Vimenti")
	if !hasHashtag("Graduación #VimentiLaFamiliaEsElCentro hoy", "VimentiLaFamiliaEsElCentro") {
		t.Error("debió ver el hashtag")
	}
	if hasHashtag("#VimentiLaFamilia", "Vimenti") {
		t.Error("#Vimenti no es #VimentiLaFamilia")
	}
	if !distinctiveTag("VimentiLaFamiliaEsElCentro", v) || distinctiveTag("ModeloDeDosGeneraciones", v) {
		t.Error("distintivo = contiene un alias de la marca")
	}
}

func readTestdata(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// Respuesta real de api.bsky.app searchPosts (q=vimenti y q="oasis hub",
// 16 nov 2026), recortada.
func TestParseBlueskyYAceptar(t *testing.T) {
	posts, err := parseBlueskyPosts(readTestdata(t, "bluesky_search.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 8 {
		t.Fatalf("posts = %d, want 8", len(posts))
	}
	p := posts[0]
	if p.URL != "https://bsky.app/profile/noticiasenlineapr.bsky.social/post/3mxcz3nqepj2j" {
		t.Errorf("url = %q", p.URL)
	}
	if p.Published.IsZero() || p.Published.Year() != 2026 {
		t.Errorf("fecha = %v", p.Published)
	}
	if !strings.Contains(p.Text, "Vimenti") {
		t.Errorf("texto = %q", p.Text)
	}
	// El post de News is my Business no trae texto: el título del enlace sí.
	if !strings.Contains(posts[2].Text, "Vimenti, Starbucks") {
		t.Errorf("texto del enlace incrustado = %q", posts[2].Text)
	}

	m := Matcher{Context: brandContext(allBrands())}
	vim, oasis := brand("Vimenti"), brand("Oasis 360")
	accepted := map[string]bool{}
	for _, p := range posts {
		if _, ok := acceptPost(p, vim, m, false); ok {
			accepted["vimenti "+p.URL] = true
		}
		if _, ok := acceptPost(p, oasis, m, false); ok {
			accepted["oasis "+p.URL] = true
		}
	}
	want := []string{
		"vimenti https://bsky.app/profile/noticiasenlineapr.bsky.social/post/3mxcz3nqepj2j",
		"vimenti https://bsky.app/profile/newsismybusiness.com/post/3mwpya2iu3j2k",
		"oasis https://bsky.app/profile/nff-news.bsky.social/post/3mxcymh7pnk23",
	}
	for _, w := range want {
		if !accepted[w] {
			t.Errorf("faltó %s", w)
		}
	}
	// Princess-vimenti, Oasis Hub Bath, Oasis Hub Waterloo y los enlaces sueltos: fuera.
	for k := range accepted {
		if strings.Contains(k, "thewelshtaffy24") || strings.Contains(k, "a51fr3d") || strings.Contains(k, "karenada59") || strings.Contains(k, "razzcus") {
			t.Errorf("ruido aceptado: %s", k)
		}
	}
}

// Respuesta real de mastodon.social /api/v1/timelines/tag/puertorico (16 nov 2026), recortada.
func TestParseMastodon(t *testing.T) {
	posts, err := parseMastodonStatuses(readTestdata(t, "mastodon_tag.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(posts) != 3 {
		t.Fatalf("posts = %d", len(posts))
	}
	for _, p := range posts {
		if !strings.HasPrefix(p.URL, "https://") || p.Published.IsZero() {
			t.Errorf("post incompleto: %+v", p)
		}
		if strings.Contains(p.Text, "<") {
			t.Errorf("quedó HTML: %q", p.Text)
		}
		found := false
		for _, tg := range p.Tags {
			if tg == "puertorico" {
				found = true
			}
		}
		if !found {
			t.Errorf("sin la etiqueta puertorico: %v", p.Tags)
		}
	}
	// Ninguno nombra una marca: ninguno entra.
	m := Matcher{Context: brandContext(allBrands())}
	for _, b := range allBrands() {
		for _, p := range posts {
			if _, ok := acceptPost(p, b, m, false); ok {
				t.Errorf("%s aceptó %s", b.Name, p.URL)
			}
		}
	}
}

func TestSiteOwnerDominioCompartido(t *testing.T) {
	all := allBrands()
	var oasis, psi string
	for _, b := range all {
		switch b.Name {
		case "Oasis 360":
			oasis = b.ID.String()
		case "Platform for Social Impact (PSI)":
			psi = b.ID.String()
		}
	}
	cases := map[string]string{
		"https://impactpr.co/oasis360.html":       oasis,
		"https://impactpr.co/oasis360-story.html": oasis,
		"https://impactpr.co/who-we-are.html":     psi,
		"https://impactpr.co/":                    psi,
		"https://www.impactpr.co/investors.html":  psi,
		"https://otro.org/oasis360.html":          "",
	}
	for u, want := range cases {
		if got := siteOwner(u, all); got != want {
			t.Errorf("siteOwner(%s) = %s, want %s", u, got, want)
		}
	}
}

func TestDescubrirFeed(t *testing.T) {
	page := []byte(`<link rel="alternate" type="application/rss+xml" title="Vimenti &raquo; Feed" href="https://vimenti.org/feed/" />
<link rel="alternate" type="application/rss+xml" title="Comentarios" href="https://vimenti.org/comments/feed/" />
<link rel="alternate" href="/blog-feed.xml" type="application/rss+xml" title="Project Makers - RSS"/>
<link rel="stylesheet" href="x.css">`)
	base := mustURL(t, "https://www.projectmakerspr.org/")
	got := discoverFeedLinks(page, base)
	if len(got) != 2 || got[0] != "https://vimenti.org/feed/" || got[1] != "https://www.projectmakerspr.org/blog-feed.xml" {
		t.Errorf("feeds = %v", got)
	}
}

func TestBuildSearchQueriesMarca(t *testing.T) {
	q := buildSearchQueries(brand("Platform for Social Impact (PSI)"))
	for _, s := range q {
		if strings.Contains(s, ".") {
			t.Errorf("un dominio no es búsqueda: %q", s)
		}
		if strings.HasPrefix(s, `"PSI"`) && !strings.Contains(s, "Puerto Rico") {
			t.Errorf("PSI sin Puerto Rico: %q", s)
		}
	}
	if len(q) == 0 || len(q) > 7 {
		t.Errorf("queries = %v", q)
	}
}

func TestYouTubeChannelsDePerfiles(t *testing.T) {
	got := youtubeChannels(brand("Vimenti"))
	if len(got) != 1 || got[0] != "UCtIP11PWtx-QX_odLsQVLbw" {
		t.Errorf("canales = %v", got)
	}
}

func TestDeduplicadoEntreFuentes(t *testing.T) {
	gn := titleKey("Vimenti y Starbucks lanzan recaudación de $5 para familias en Puerto Rico - News is My Business")
	bing := titleKey("Vimenti y Starbucks lanzan recaudación de $5 para familias en Puerto Rico")
	if gn == "" || gn != bing {
		t.Errorf("la misma noticia debe dar la misma clave: %q vs %q", gn, bing)
	}
	if titleKey("Vimenti") != "" {
		t.Error("un título corto no deduplica")
	}
	u := unwrapRedirect("http://www.bing.com/news/apiclick.aspx?ref=FexRss&aid=&tid=x&url=https%3a%2f%2fpresenciapr.com%2fvimenti-celebrara-segunda-feria-de-empleo-en-san-juan%2f&c=1&mkt=es-xl")
	if u != "https://presenciapr.com/vimenti-celebrara-segunda-feria-de-empleo-en-san-juan/" {
		t.Errorf("bing = %q", u)
	}
	g := unwrapRedirect("https://www.google.com/url?rct=j&sa=t&url=https://www.lisc.org/our-resources/resource/oasis-hub/%3Futm_source%3Dx&ct=ga")
	if g != "https://www.lisc.org/our-resources/resource/oasis-hub/" {
		t.Errorf("google alerts = %q", g)
	}
}

func TestMatchItemPorHashtag(t *testing.T) {
	m := Matcher{Context: brandContext(allBrands())}
	psi := brand("Platform for Social Impact (PSI)")
	if mt, ok := matchItem("Mira lo nuevo en #SocialImpactPR", psi, m); !ok || !strings.EqualFold(mt.Tag, "socialimpactpr") {
		t.Errorf("hashtag distintivo: tag=%q ok=%v", mt.Tag, ok)
	}
	v := brand("Vimenti")
	if _, ok := matchItem("#ModeloDeDosGeneraciones en otra escuela", v, m); ok {
		t.Error("un hashtag genérico solo no basta")
	}
	if mt, ok := matchItem("Vimenti celebra #ModeloDeDosGeneraciones", v, m); !ok || mt.Tag != "ModeloDeDosGeneraciones" {
		t.Errorf("con el nombre sí, y guarda el hashtag: %q %v", mt.Tag, ok)
	}
	if mt, _ := matchItem("Vimenti abre matrícula", v, m); mt.Tag != "" {
		t.Errorf("la palabra «Vimenti» sin # no es el hashtag #Vimenti: %q", mt.Tag)
	}
}

func TestPersonaClave(t *testing.T) {
	all := allBrands()
	m := Matcher{Context: brandContext(all), Brands: brandTerms(all)}
	psi := brand("Platform for Social Impact (PSI)")
	tests := []struct {
		text string
		want bool
	}{
		{"Eduardo Carrera, fundador de PSI, habló en el foro", true},
		{"Eduardo Carrera visitó el Oasis Hub", true},
		{"Eduardo Carrera y Vimenti firman alianza", true},
		{"Eduardo Carrera, líder sin fines de lucro en Puerto Rico", true},
		{"Eduardo Carrera habla de social impact investing in Puerto Rico", true},
		{"Eduardo Carrera ganó el torneo de golf en Puerto Rico", false}, // PR solo no basta
		{"El actor Eduardo Carrera estrena película", false},
		{"Eduardo Carrera, nonprofit leader in Texas", false},
		{"Carrera de Eduardo en PSI", false}, // no es el nombre completo
	}
	for _, tt := range tests {
		mt, ok := matchItem(tt.text, psi, m)
		got := ok && mt.Person == "Eduardo Carrera"
		if got != tt.want {
			t.Errorf("persona(%q) = %v (ok=%v person=%q), want %v", tt.text, got, ok, mt.Person, tt.want)
		}
	}
}

// Feed de Google Alerts con la estructura documentada (Atom, enlaces
// google.com/url?url=…). No se pudo generar uno real sin cuenta de Google.
func TestAlertasAtom(t *testing.T) {
	body := readTestdata(t, "google_alerts.xml")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/atom+xml")
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	items, err := parseFeed(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("entradas = %d", len(items))
	}
	if got := stripHTML(items[0].Title); got != "Vimenti celebrará segunda feria de empleo en San Juan" {
		t.Errorf("título = %q", got)
	}
	if got := unwrapRedirect(items[0].Link); got != "https://presenciapr.com/vimenti-celebrara-segunda-feria-de-empleo-en-san-juan/" {
		t.Errorf("enlace = %q", got)
	}
	if items[0].Published.IsZero() {
		t.Error("sin fecha")
	}
}
