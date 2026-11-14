package extract

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// doc es el texto preparado una sola vez: se trabaja siempre por runas.
//
//	orig  runas del texto original
//	flat  runas con los espacios/saltos de línea colapsados a un espacio
//	fold  flat en minúsculas y sin acentos (misma cantidad de runas que flat)
//	fo    índice en orig de cada runa de flat (para citar evidencia literal)
//	b2r   desplazamiento en bytes de fold -> índice de runa
type doc struct {
	orig  []rune
	flat  []rune
	foldR []rune
	fold  string
	fo    []int
	b2r   []int
}

func newDoc(text string) *doc {
	src := []rune(text)
	d := &doc{orig: src}
	flat := make([]rune, 0, len(src))
	fo := make([]int, 0, len(src))
	pending := false
	spaceIdx := 0
	emit := func(r rune, i int) {
		if pending && len(flat) > 0 {
			flat = append(flat, ' ')
			fo = append(fo, spaceIdx)
		}
		pending = false
		flat = append(flat, r)
		fo = append(fo, i)
	}
	isBreak := func(r rune) bool { return r == '\n' || r == '\r' || r == '\f' || r == '\v' }
	for i := 0; i < len(src); i++ {
		r := src[i]
		switch {
		case r == 0xAD || r == 0x200B || r == 0x200C || r == 0x200D || r == 0xFEFF || unicode.Is(unicode.Mn, r):
			continue // invisibles y tildes sueltas (NFD)
		case r == '-' || r == 0x2010:
			// Guion al final de renglón de un PDF ("propues-\nta", "24-\n015").
			j := i + 1
			for j < len(src) && (src[j] == ' ' || src[j] == '\t') {
				j++
			}
			if j < len(src) && isBreak(src[j]) && i > 0 && isAlnum(src[i-1]) {
				k := j
				for k < len(src) && unicode.IsSpace(src[k]) {
					k++
				}
				if k < len(src) {
					if unicode.IsLetter(src[i-1]) && unicode.IsLower(src[k]) {
						i = k - 1 // palabra partida: se une sin guion
						continue
					}
					emit('-', i)
					i = k - 1 // se conserva el guion, se quita el salto
					continue
				}
			}
			emit(r, i)
		case unicode.IsSpace(r) || r < 32 || r == 0x7f:
			if !pending {
				pending = true
				spaceIdx = i
			}
		default:
			emit(r, i)
		}
	}
	d.flat, d.fo = flat, fo
	d.foldR = make([]rune, len(flat))
	var sb strings.Builder
	b2r := make([]int, 0, len(flat)+1)
	for i, r := range flat {
		f := foldRune(r)
		d.foldR[i] = f
		n := utf8.RuneLen(f)
		if n < 0 {
			f, n = ' ', 1
			d.foldR[i] = f
		}
		for k := 0; k < n; k++ {
			b2r = append(b2r, i)
		}
		sb.WriteRune(f)
	}
	b2r = append(b2r, len(flat))
	d.fold, d.b2r = sb.String(), b2r
	return d
}

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

func foldRune(r rune) rune {
	r = unicode.ToLower(r)
	switch r {
	case 'á', 'à', 'â', 'ä', 'ã', 'å':
		return 'a'
	case 'é', 'è', 'ê', 'ë':
		return 'e'
	case 'í', 'ì', 'î', 'ï':
		return 'i'
	case 'ó', 'ò', 'ô', 'ö', 'õ', 'º', '°':
		return 'o'
	case 'ú', 'ù', 'û', 'ü':
		return 'u'
	case 'ñ':
		return 'n'
	case 'ç':
		return 'c'
	case '–', '—', '‑', '−':
		return '-'
	case '’', '‘', '´':
		return '\''
	case '“', '”', '«', '»':
		return '"'
	}
	return r
}

// span convierte un rango en bytes de d.fold a un rango en runas.
func (d *doc) span(bs, be int) (int, int) { return d.b2r[bs], d.b2r[be] }

func (d *doc) sub(a, b int) string {
	a, b = d.clamp(a), d.clamp(b)
	if b < a {
		return ""
	}
	return string(d.foldR[a:b])
}

func (d *doc) clamp(i int) int {
	if i < 0 {
		return 0
	}
	if i > len(d.flat) {
		return len(d.flat)
	}
	return i
}

var abbrevs = map[string]bool{"num": true, "no": true, "p.m": true, "a.m": true, "sr": true, "sra": true,
	"dr": true, "dra": true, "lic": true, "ing": true, "inc": true, "art": true, "sec": true, "st": true,
	"corp": true, "dept": true, "u.s": true, "depto": true, "tel": true, "ext": true, "vs": true, "e.g": true, "i.e": true}

func (d *doc) abbrevBefore(i int) bool {
	j := i
	for j > 0 && i-j < 6 && (unicode.IsLetter(d.flat[j-1]) || d.flat[j-1] == '.') {
		j--
	}
	return abbrevs[strings.ToLower(string(d.flat[j:i]))]
}

// clauseStart: inicio de la oración/cláusula que contiene pos, mirando a lo
// sumo max runas atrás. Corta en '|', ';' y en ". " seguido de mayúscula.
func (d *doc) clauseStart(pos, max int) int {
	lo := pos - max
	if lo < 0 {
		lo = 0
	}
	for i := pos - 1; i >= lo; i-- {
		c := d.flat[i]
		if c == '|' || c == ';' {
			return i + 1
		}
		if (c == '.' || c == '!' || c == '?') && i+2 < len(d.flat) && d.flat[i+1] == ' ' &&
			unicode.IsUpper(d.flat[i+2]) && !d.abbrevBefore(i) {
			return i + 2
		}
	}
	return lo
}

func (d *doc) clauseEnd(pos, max int) int {
	hi := pos + max
	if hi > len(d.flat) {
		hi = len(d.flat)
	}
	for i := pos; i < hi; i++ {
		c := d.flat[i]
		if c == '|' || c == ';' {
			return i
		}
		if (c == '.' || c == '!' || c == '?') && i+2 < len(d.flat) && d.flat[i+1] == ' ' &&
			unicode.IsUpper(d.flat[i+2]) && !d.abbrevBefore(i) {
			return i + 1
		}
	}
	return hi
}

// evidence devuelve un fragmento literal del texto original (≤200 runas) que
// contiene el rango [rs,re) de flat, con algo de contexto.
func (d *doc) evidence(rs, re int) string {
	if len(d.flat) == 0 || re <= rs {
		return ""
	}
	rs, re = d.clamp(rs), d.clamp(re)
	if re <= rs {
		return ""
	}
	a, b := d.clamp(rs-60), d.clamp(re+60)
	oa, ob := d.fo[a], d.fo[b-1]+1
	ms, me := d.fo[rs], d.fo[re-1]+1
	if me-ms >= 200 {
		oa, ob = ms, ms+200
	} else {
		extra := 200 - (me - ms)
		if oa < ms-extra/2 {
			oa = ms - extra/2
		}
		if ob > oa+200 {
			ob = oa + 200
		}
		for oa > 0 && oa < ms && !unicode.IsSpace(d.orig[oa-1]) && !unicode.IsSpace(d.orig[oa]) {
			oa++
		}
		for ob < len(d.orig) && ob > me && !unicode.IsSpace(d.orig[ob-1]) && !unicode.IsSpace(d.orig[ob]) {
			ob--
		}
	}
	return strings.TrimSpace(string(d.orig[oa:ob]))
}

func (d *doc) flatStr(a, b int) string { return string(d.flat[d.clamp(a):d.clamp(b)]) }
