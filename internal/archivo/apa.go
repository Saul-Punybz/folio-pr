// Package archivo reúne lo que Folio necesita para dejar una mención en
// récord: extracción de metadatos y texto de una página, resumen por reglas,
// cita APA 7 en español y exportación. Todo por reglas, sin IA.
package archivo

import (
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// Tipos de documento para la cita.
const (
	KindArticulo = "articulo"
	KindPost     = "post"
	KindVideo    = "video"
)

// KindFor da el tipo de cita de una fuente de Folio.
func KindFor(sourceType string) string {
	switch sourceType {
	case "youtube":
		return KindVideo
	case "bluesky", "mastodon":
		return KindPost
	default:
		return KindArticulo
	}
}

// PlatformFor da el nombre de la plataforma de una fuente de redes o video.
func PlatformFor(sourceType string) string {
	switch sourceType {
	case "youtube":
		return "YouTube"
	case "bluesky":
		return "Bluesky"
	case "mastodon":
		return "Mastodon"
	case "reddit":
		return "Reddit"
	}
	return ""
}

// Meta son los metadatos de los que sale la cita. La usuaria puede
// corregirlos y la cita se vuelve a generar.
type Meta struct {
	Kind      string // KindArticulo, KindPost o KindVideo
	Title     string
	Site      string // medio o plataforma
	Authors   []string
	Published *time.Time
	URL       string
	Text      string // texto del post (para KindPost)
	Platform  string // «Bluesky», «Mastodon», «YouTube»
}

// Cita es la referencia APA en dos formas: texto plano (sin cursiva) y
// Markdown (cursiva con *…*).
type Cita struct {
	Plain    string `json:"plain"`
	Markdown string `json:"markdown"`
}

// puertoRico: sin horario de verano, UTC-4 todo el año.
var puertoRico = time.FixedZone("AST", -4*3600)

var meses = [...]string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio",
	"agosto", "septiembre", "octubre", "noviembre", "diciembre"}

// FechaAPA da «2025, 19 de octubre», o «s. f.» si no hay fecha.
func FechaAPA(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "s. f."
	}
	l := t.In(puertoRico)
	return strconv.Itoa(l.Year()) + ", " + strconv.Itoa(l.Day()) + " de " + meses[l.Month()-1]
}

// FechaLarga: «19 de octubre de 2025» (para la pantalla).
func FechaLarga(t *time.Time) string {
	if t == nil || t.IsZero() {
		return ""
	}
	l := t.In(puertoRico)
	return strconv.Itoa(l.Day()) + " de " + meses[l.Month()-1] + " de " + strconv.Itoa(l.Year())
}

type part struct {
	s      string
	italic bool
}

type builder struct{ parts []part }

func (b *builder) add(s string) { b.parts = append(b.parts, part{s: s}) }
func (b *builder) italic(s string) {
	b.parts = append(b.parts, part{s: strings.ReplaceAll(s, "*", ""), italic: true})
}
func (b *builder) cita() Cita {
	var plain, md strings.Builder
	for _, p := range b.parts {
		plain.WriteString(p.s)
		if p.italic && p.s != "" {
			md.WriteString("*" + p.s + "*")
		} else {
			md.WriteString(p.s)
		}
	}
	return Cita{Plain: plain.String(), Markdown: md.String()}
}

// endDot agrega el punto final salvo que ya termine en puntuación.
func endDot(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}
	r, _ := utf8.DecodeLastRuneInString(s)
	switch r {
	case '.', '?', '!', '…', ']':
		if r == ']' {
			return s + "."
		}
		return s
	}
	return s + "."
}

// APA arma la referencia APA 7 en español.
//
//	artículo con autor: Apellido, I. (2025, 19 de octubre). Título. *Medio*. URL
//	artículo sin autor: Título. (2025, 19 de octubre). *Medio*. URL
//	sin fecha:          (s. f.)
//	post:               Autor [@usuario]. (fecha). *Primeras 20 palabras…* [Publicación]. Plataforma. URL
//	video:              Canal. (fecha). *Título* [Video]. YouTube. URL
func APA(m Meta) Cita {
	title := collapse(m.Title)
	site := collapse(m.Site)
	date := "(" + FechaAPA(m.Published) + ")"
	link := strings.TrimSpace(m.URL)

	var b builder
	switch m.Kind {
	case KindPost:
		who := postAuthor(m)
		text := collapse(m.Text)
		if text == "" {
			text = title
		}
		if text == "" {
			text = "(sin texto)"
		}
		if who != "" {
			b.add(endDot(who) + " ")
		}
		b.add(date + ". ")
		b.italic(firstWords(text, 20))
		b.add(" [Publicación]. ")
		plat := firstNonEmpty(m.Platform, site)
		if plat != "" {
			b.add(endDot(plat) + " ")
		}
	case KindVideo:
		who := ""
		if len(m.Authors) > 0 {
			who = collapse(m.Authors[0])
		}
		if who != "" {
			b.add(endDot(who) + " ")
		}
		b.add(date + ". ")
		if title == "" {
			title = "(sin título)"
		}
		b.italic(strings.TrimRight(title, ". "))
		b.add(" [Video]. ")
		b.add(endDot(firstNonEmpty(m.Platform, site, "YouTube")) + " ")
	default:
		if title == "" {
			title = "(sin título)"
		}
		authors := FormatAuthors(m.Authors)
		if authors != "" {
			b.add(endDot(authors) + " " + date + ". " + endDot(title) + " ")
		} else {
			b.add(endDot(title) + " " + date + ". ")
		}
		// Si el autor es el mismo medio, APA no repite el medio.
		if site != "" && !(len(m.Authors) == 1 && fold(collapse(m.Authors[0])) == fold(site)) {
			b.italic(strings.TrimRight(site, ". "))
			b.add(". ")
		}
	}
	b.add(link)
	c := b.cita()
	c.Plain = strings.TrimSpace(c.Plain)
	c.Markdown = strings.TrimSpace(c.Markdown)
	return c
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if s = collapse(s); s != "" {
			return s
		}
	}
	return ""
}

func collapse(s string) string { return strings.Join(strings.Fields(s), " ") }

// firstWords devuelve las primeras n palabras; si cortó, agrega «…».
func firstWords(s string, n int) string {
	w := strings.Fields(s)
	if len(w) <= n {
		return strings.Join(w, " ")
	}
	return strings.TrimRight(strings.Join(w[:n], " "), ",;:") + "…"
}

// postAuthor: «Nombre [@usuario]» (o solo uno de los dos).
func postAuthor(m Meta) string {
	name := ""
	if len(m.Authors) > 0 {
		name = collapse(m.Authors[0])
	}
	handle := HandleFromURL(m.URL)
	switch {
	case name == "" && handle == "":
		return ""
	case handle == "":
		return name
	case name == "" || fold(strings.TrimPrefix(name, "@")) == fold(strings.TrimPrefix(handle, "@")):
		return handle
	}
	return name + " [" + handle + "]"
}

// HandleFromURL saca @usuario de una URL de Bluesky o Mastodon.
func HandleFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	switch {
	case strings.HasSuffix(u.Hostname(), "bsky.app") && len(parts) >= 2 && parts[0] == "profile":
		if strings.HasPrefix(parts[1], "did:") {
			return ""
		}
		return "@" + parts[1]
	case len(parts) >= 1 && strings.HasPrefix(parts[0], "@"):
		return parts[0]
	}
	return ""
}

// ── Autores ──────────────────────────────────────────────────────

// FormatAuthors da el bloque de autores APA: «Rivera Batista, B., y Pérez, J.».
// En español se usa «y» sin coma antes (A, B y C); hasta 20 autores.
func FormatAuthors(names []string) string {
	var out []string
	for _, n := range names {
		if f := FormatAuthor(n); f != "" {
			out = append(out, f)
		}
	}
	switch len(out) {
	case 0:
		return ""
	case 1:
		return out[0]
	case 2:
		return out[0] + " y " + out[1]
	}
	if len(out) > 20 {
		return strings.Join(out[:19], ", ") + ", … " + out[len(out)-1]
	}
	return strings.Join(out[:len(out)-1], ", ") + " y " + out[len(out)-1]
}

var orgWords = []string{"redaccion", "staff", "news", "noticias", "media", "inc", "llc", "press",
	"agencia", "equipo", "team", "foundation", "fundacion", "universidad", "university",
	"departamento", "gobierno", "oficina", "periodico", "newsroom", "editorial", "project makers",
	"vimenti", "oasis", "platform for", "associated press", "efe", "ap", "el nuevo dia", "primera hora"}

// partículas de apellido que se pegan al apellido que sigue.
var particulas = map[string]bool{"de": true, "del": true, "la": true, "las": true, "los": true,
	"y": true, "van": true, "von": true, "da": true, "di": true, "do": true, "dos": true,
	"san": true, "santa": true, "st.": true}

// nombres de pila hispanos frecuentes: permiten distinguir «Beatriz Rivera
// Batista» (un nombre, dos apellidos) de «John Ronald Tolkien».
var nombresPila = setOf("alberto", "alejandro", "alexandra", "alfredo", "alicia", "amanda", "ana", "andrea",
	"andres", "angel", "angela", "antonio", "armando", "beatriz", "benjamin", "blanca", "camila", "carla",
	"carlos", "carmen", "carolina", "celia", "cesar", "claudia", "cristina", "daniel", "david", "diana",
	"diego", "dolores", "eduardo", "edwin", "elena", "eliezer", "elizabeth", "emilio", "enrique", "ernesto",
	"esteban", "eva", "fernando", "francisco", "gabriel", "gabriela", "gerardo", "gloria", "guillermo",
	"hector", "ines", "irma", "isabel", "israel", "ivan", "javier", "jesus", "joel", "jorge", "jose",
	"josefina", "juan", "judith", "julio", "karla", "laura", "leonardo", "lillian", "lilliam", "lisa",
	"lorena", "lourdes", "luis", "luz", "manuel", "marcos", "margarita", "maria", "mariana", "marta",
	"mario", "marisol", "miguel", "mildred", "mónica", "monica", "natalia", "nelson", "nestor", "nilda",
	"noel", "olga", "omar", "oscar", "pablo", "pedro", "rafael", "ramon", "raul", "ricardo", "roberto",
	"rosa", "ruben", "samuel", "sandra", "santiago", "sara", "sergio", "silvia", "sofia", "teresa",
	"tomas", "valeria", "vanessa", "vicente", "victor", "victoria", "wanda", "xavier", "yolanda",
	"itzel", "ivelisse", "zulma", "yaritza", "wilfredo", "eliud", "idalia", "migdalia", "marangelie")

func setOf(ss ...string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[fold(s)] = true
	}
	return m
}

func isOrgName(name string) bool {
	f := fold(name)
	if strings.ContainsAny(f, "@0123456789") || strings.Contains(f, ".com") || strings.Contains(f, ".org") {
		return true
	}
	words := strings.Fields(f)
	for _, ow := range orgWords {
		if strings.Contains(ow, " ") {
			if strings.Contains(f, ow) {
				return true
			}
			continue
		}
		for _, w := range words {
			if w == ow {
				return true
			}
		}
	}
	return false
}

// FormatAuthor pasa un nombre a «Apellido(s), I.». Acepta «Nombre Apellido»,
// «Apellido, Nombre» y nombres con dos apellidos o compuestos. Un nombre de
// organización o de usuario se deja tal cual.
func FormatAuthor(name string) string {
	name = collapse(strings.Trim(name, " ,;"))
	if name == "" {
		return ""
	}
	if i := strings.Index(name, ","); i > 0 {
		sur, given := collapse(name[:i]), collapse(name[i+1:])
		if given == "" {
			return sur
		}
		return sur + ", " + initials(strings.Fields(given))
	}
	if isOrgName(name) {
		return name
	}
	toks := mergeParticles(strings.Fields(name))
	if len(toks) < 2 {
		return name
	}
	given, sur := splitGiven(toks)
	if len(given) == 0 || len(sur) == 0 {
		return name
	}
	return strings.Join(sur, " ") + ", " + initials(given)
}

// mergeParticles pega «de», «del», «de la»… al apellido que sigue.
func mergeParticles(toks []string) []string {
	var out []string
	var pend []string
	for _, t := range toks {
		if particulas[fold(t)] && len(out) > 0 {
			pend = append(pend, t)
			continue
		}
		if len(pend) > 0 {
			t = strings.Join(append(pend, t), " ")
			pend = nil
		}
		out = append(out, t)
	}
	if len(pend) > 0 && len(out) > 0 {
		out[len(out)-1] += " " + strings.Join(pend, " ")
	}
	return out
}

// splitGiven separa nombres de pila y apellidos.
func splitGiven(toks []string) (given, sur []string) {
	n := len(toks)
	if n == 2 {
		return toks[:1], toks[1:]
	}
	// «John R. Smith»: inicial en medio.
	if isInitial(toks[1]) {
		k := 1
		for k < n-1 && isInitial(toks[k]) {
			k++
		}
		return toks[:k], toks[k:]
	}
	// Nombres compuestos hispanos: «José Luis Rodríguez Santos».
	k := 0
	for k < n-1 && nombresPila[fold(toks[k])] {
		k++
	}
	switch {
	case k >= 2:
		return toks[:k], toks[k:]
	case k == 1:
		if n == 3 {
			return toks[:1], toks[1:] // un nombre, dos apellidos
		}
		return toks[:1], toks[1:]
	}
	// Primer nombre desconocido: un solo apellido, el último.
	return toks[:n-1], toks[n-1:]
}

func isInitial(t string) bool {
	t = strings.TrimSuffix(t, ".")
	return utf8.RuneCountInString(t) == 1 && unicode.IsUpper([]rune(t)[0])
}

// initials: «José Luis» → «J. L.»; «Jean-Pierre» → «J.-P.».
func initials(given []string) string {
	var out []string
	for _, g := range given {
		var parts []string
		for _, h := range strings.Split(g, "-") {
			h = strings.Trim(h, ".")
			if h == "" {
				continue
			}
			r, _ := utf8.DecodeRuneInString(h)
			parts = append(parts, strings.ToUpper(string(r))+".")
		}
		if len(parts) > 0 {
			out = append(out, strings.Join(parts, "-"))
		}
	}
	return strings.Join(out, " ")
}

// ── Bibliografía ─────────────────────────────────────────────────

// fold: minúsculas sin acentos.
func fold(s string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(s) {
		switch r {
		case 'á', 'à', 'ä', 'â':
			r = 'a'
		case 'é', 'è', 'ë', 'ê':
			r = 'e'
		case 'í', 'ì', 'ï', 'î':
			r = 'i'
		case 'ó', 'ò', 'ö', 'ô':
			r = 'o'
		case 'ú', 'ù', 'ü', 'û':
			r = 'u'
		case 'ñ':
			r = 'n'
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Entrada de bibliografía: la cita y la fecha, para ordenar.
type Entrada struct {
	Cita      Cita
	Published *time.Time
}

// Ordenar pone las citas en orden alfabético APA (por autor o, si no hay,
// por título; a igual autor, por fecha).
func Ordenar(es []Entrada) {
	key := func(e Entrada) string {
		return strings.TrimLeft(fold(e.Cita.Plain), " \"'«¿¡([")
	}
	sort.SliceStable(es, func(i, j int) bool {
		ki, kj := key(es[i]), key(es[j])
		if ki != kj {
			return ki < kj
		}
		a, b := es[i].Published, es[j].Published
		switch {
		case a == nil:
			return false
		case b == nil:
			return true
		}
		return a.Before(*b)
	})
}
