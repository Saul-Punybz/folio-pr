package watch

import (
	"strings"
	"unicode"

	"github.com/Saul-Punybz/folio/internal/models"
)

// fold pasa a minúsculas y quita acentos (á→a, ñ→n) para comparar.
func fold(s string) string {
	var b strings.Builder
	b.Grow(len(s))
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
		case '’', '‘':
			r = '\''
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// containsWord dice si term aparece en text (ya doblados) como palabra entera:
// «vimenti» no entra en «vimentin».
func containsWord(text, term string) bool {
	if term == "" {
		return false
	}
	for i := 0; ; {
		j := strings.Index(text[i:], term)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(term)
		before, after := ' ', ' '
		if start > 0 {
			before = lastRune(text[:start])
		}
		if end < len(text) {
			after = firstRune(text[end:])
		}
		if !isWordRune(before) && !isWordRune(after) {
			return true
		}
		i = start + 1
		if i >= len(text) {
			return false
		}
	}
}

func firstRune(s string) rune {
	for _, r := range s {
		return r
	}
	return ' '
}

func lastRune(s string) rune {
	r := []rune(s)
	if len(r) == 0 {
		return ' '
	}
	return r[len(r)-1]
}

// matchTerms son los términos que identifican a un org. En una marca con
// alias, el nombre NO cuenta por sí solo (así «Oasis» a secas no trae la banda):
// cuentan los alias. En temas y personas, el nombre y las palabras clave.
func matchTerms(org models.WatchlistOrg) []string {
	var terms []string
	if org.IsBrand() && len(org.Aliases)+len(org.ContextAliases) > 0 {
		terms = append(terms, org.Aliases...)
		terms = append(terms, org.ContextAliases...)
	} else {
		terms = append(terms, org.Name)
		terms = append(terms, org.Aliases...)
	}
	terms = append(terms, org.Keywords...)
	out := terms[:0]
	for _, t := range terms {
		t = strings.TrimSpace(t)
		if len([]rune(t)) > 1 {
			out = append(out, t)
		}
	}
	return out
}

// weakTerm: un alias corto («PSI») es demasiado genérico y solo cuenta junto
// a contexto (Puerto Rico u otro nombre de la organización).
func weakTerm(t string) bool { return len([]rune(strings.TrimSpace(t))) <= 4 }

// Excluded dice si el texto trae alguna exclusión del org.
func Excluded(text string, org models.WatchlistOrg) bool {
	f := fold(text)
	for _, ex := range org.ExcludeTerms {
		ex = fold(strings.TrimSpace(ex))
		if ex != "" && containsWord(f, ex) {
			return true
		}
	}
	return false
}

// Matcher decide si un texto habla de un org.
type Matcher struct {
	// Context: otros nombres de la organización (alias de todas las marcas)
	// que, junto a Puerto Rico, le dan sentido a un alias corto.
	Context []string
	// NeedPR: en fuentes globales (Bluesky), un alias de varias palabras
	// («Project Makers») también exige contexto.
	NeedPR bool
	// Brands: todos los nombres y alias de marca (contexto de personas clave).
	Brands []string
}

// Match aplica alias + exclusiones. Devuelve el término que coincidió.
func (m Matcher) Match(text string, org models.WatchlistOrg) (string, bool) {
	f := fold(text)
	if Excluded(text, org) {
		return "", false
	}
	ctx := -1 // -1 sin calcular, 0 no, 1 sí
	hasContext := func(skip string) bool {
		if ctx >= 0 {
			return ctx == 1
		}
		ctx = 0
		if mentionsPR(f) {
			ctx = 1
			return true
		}
		for _, c := range m.Context {
			c = fold(strings.TrimSpace(c))
			if c != "" && c != skip && !weakTerm(c) && containsWord(f, c) {
				ctx = 1
				return true
			}
		}
		return false
	}
	for _, t := range matchTerms(org) {
		ft := fold(t)
		if !containsWord(f, ft) {
			continue
		}
		// «Impact PR» también es el nombre de agencias de relaciones públicas:
		// solo cuenta con contexto propio de la organización.
		if ft == "impact pr" {
			if impactPRContext(f) {
				return t, true
			}
			continue
		}
		weak := weakTerm(t) || isContextAlias(t, org)
		multi := strings.ContainsAny(strings.TrimSpace(t), " ")
		if org.IsBrand() && (weak || (m.NeedPR && multi)) && !hasContext(ft) {
			continue
		}
		return t, true
	}
	return "", false
}

// impactPRContext: «Impact PR» solo cuenta junto a «Platform for Social
// Impact», «PSI», «Oasis», «impactpr.co», o «Puerto Rico» con «impacto social»
// / «social impact». Texto ya doblado (fold).
func impactPRContext(f string) bool {
	for _, c := range []string{"platform for social impact", "psi", "oasis"} {
		if containsWord(f, c) {
			return true
		}
	}
	if strings.Contains(f, "impactpr.co") {
		return true
	}
	return strings.Contains(f, "puerto rico") && (strings.Contains(f, "impacto social") || strings.Contains(f, "social impact"))
}

// MatchesOrg dice si el texto menciona el org (alias o nombre + palabras
// clave, como palabras enteras, sin distinguir mayúsculas ni acentos) y no
// trae ninguna exclusión. Es la regla única de vigilancia en noticias.
func MatchesOrg(text string, org models.WatchlistOrg) bool {
	_, ok := Matcher{}.Match(text, org)
	return ok
}

// containsAnyKeyword se mantiene como alias interno.
func containsAnyKeyword(text string, org models.WatchlistOrg) bool { return MatchesOrg(text, org) }

// normTag quita el # y espacios: «#Vimenti » → «Vimenti».
func normTag(t string) string {
	return strings.TrimSpace(strings.TrimLeft(strings.TrimSpace(t), "#"))
}

// hasHashtag dice si el texto trae #tag (sin distinguir mayúsculas ni acentos).
func hasHashtag(text, tag string) bool {
	tag = fold(normTag(tag))
	if tag == "" {
		return false
	}
	f := fold(text)
	needle := "#" + tag
	for i := 0; ; {
		j := strings.Index(f[i:], needle)
		if j < 0 {
			return false
		}
		end := i + j + len(needle)
		if end >= len(f) || !isWordRune(firstRune(f[end:])) && firstRune(f[end:]) != '_' {
			return true
		}
		i = end
	}
}

// distinctiveTag: el hashtag contiene un alias de la marca (#Vimenti,
// #VimentiLaFamiliaEsElCentro) y basta por sí solo; uno genérico
// (#historiadeexito) necesita además que el texto nombre a la marca.
func distinctiveTag(tag string, org models.WatchlistOrg) bool {
	ft := strings.ReplaceAll(fold(normTag(tag)), "_", "")
	for _, a := range matchTerms(org) {
		fa := strings.NewReplacer(" ", "", ".", "", "-", "").Replace(fold(a))
		if len(fa) >= 5 && strings.Contains(ft, fa) {
			return true
		}
	}
	return false
}

// isContextAlias: el alias está en la lista de «solo con contexto».
func isContextAlias(t string, org models.WatchlistOrg) bool {
	ft := fold(strings.TrimSpace(t))
	for _, c := range org.ContextAliases {
		if fold(strings.TrimSpace(c)) == ft {
			return true
		}
	}
	return false
}
