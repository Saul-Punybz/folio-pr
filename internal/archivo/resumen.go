package archivo

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// ResumenMaxRunes: largo máximo del resumen por reglas (en runas).
const ResumenMaxRunes = 600

// minSustancia: una descripción más corta que esto no resume nada.
const minSustancia = 80

// Resumen arma el resumen SIN IA: la descripción de la página si tiene
// sustancia; si no, los primeros 2 o 3 párrafos del texto limpio, cortados
// por oración, hasta ~600 caracteres. title evita aceptar una descripción
// que solo repite el título.
func Resumen(description, text, title string) string {
	d := collapse(description)
	if sustancia(d, title) {
		return CortarPorOracion(d, ResumenMaxRunes)
	}
	var paras []string
	for _, p := range strings.Split(text, "\n") {
		p = collapse(p)
		// Un párrafo de menos de 60 runas suele ser firma, fecha o intertítulo.
		if utf8.RuneCountInString(p) >= 60 {
			paras = append(paras, p)
		}
		if len(paras) == 3 {
			break
		}
	}
	if len(paras) == 0 {
		if utf8.RuneCountInString(d) > 0 && fold(d) != fold(title) {
			return CortarPorOracion(d, ResumenMaxRunes)
		}
		return ""
	}
	return CortarPorOracion(strings.Join(paras, " "), ResumenMaxRunes)
}

var abreviaturas = map[string]bool{"sr": true, "sra": true, "srta": true, "dr": true, "dra": true, "lcdo": true, "lcda": true,
	"ing": true, "prof": true, "gob": true, "núm": true, "num": true, "pág": true, "etc": true, "vs": true, "mr": true, "mrs": true,
	"ms": true, "st": true, "inc": true, "corp": true, "ee": true, "uu": true, "a.m": true, "p.m": true, "u.s": true, "jr": true}

// Oraciones parte el texto en oraciones (punto, ? o ! seguidos de espacio y
// mayúscula, comilla o número), sin cortar en abreviaturas ni iniciales.
func Oraciones(s string) []string {
	r := []rune(collapse(s))
	var out []string
	start := 0
	for i := 0; i < len(r); i++ {
		if r[i] != '.' && r[i] != '?' && r[i] != '!' && r[i] != '…' {
			continue
		}
		j := i + 1
		for j < len(r) && (r[j] == '"' || r[j] == '”' || r[j] == '»' || r[j] == '\'' || r[j] == ')') {
			j++
		}
		if j < len(r) && r[j] != ' ' {
			continue
		}
		if j >= len(r) {
			break
		}
		k := j
		for k < len(r) && r[k] == ' ' {
			k++
		}
		if k >= len(r) || !(unicode.IsUpper(r[k]) || unicode.IsDigit(r[k]) || strings.ContainsRune("\"“«¿¡'(", r[k])) {
			continue
		}
		if r[i] == '.' {
			w := i
			for w > start && (unicode.IsLetter(r[w-1]) || r[w-1] == '.') {
				w--
			}
			word := strings.ToLower(strings.TrimRight(string(r[w:i]), "."))
			if abreviaturas[word] || utf8.RuneCountInString(word) == 1 {
				continue
			}
		}
		out = append(out, strings.TrimSpace(string(r[start:j])))
		start = k
		i = k - 1
	}
	if start < len(r) {
		out = append(out, strings.TrimSpace(string(r[start:])))
	}
	return out
}

// CortarPorOracion deja las oraciones completas que quepan en max runas. Si
// ni la primera cabe, corta por palabra y termina en «…».
func CortarPorOracion(s string, max int) string {
	s = collapse(s)
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	var b []string
	n := 0
	for _, o := range Oraciones(s) {
		l := utf8.RuneCountInString(o)
		add := l
		if len(b) > 0 {
			add++
		}
		if n+add > max {
			break
		}
		b = append(b, o)
		n += add
	}
	if len(b) > 0 {
		return strings.Join(b, " ")
	}
	r := []rune(s)[:max-1]
	if i := lastSpace(r); i > len(r)/2 {
		r = r[:i]
	}
	return strings.TrimRight(string(r), " ,;:-–—") + "…"
}

func lastSpace(r []rune) int {
	for i := len(r) - 1; i >= 0; i-- {
		if unicode.IsSpace(r[i]) {
			return i
		}
	}
	return -1
}

// sustancia: la descripción sirve de resumen si dice algo más que el título
// y no viene cortada a media frase (Wix corta la suya a ~500 caracteres).
func sustancia(d, title string) bool {
	n := utf8.RuneCountInString(d)
	if n < minSustancia || fold(d) == fold(title) || strings.HasPrefix(fold(d), fold(title)+" ") {
		return false
	}
	if n > 400 && !strings.ContainsAny(string([]rune(d)[n-1:]), ".?!…\"”»") {
		return false
	}
	return true
}
