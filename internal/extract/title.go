package extract

import (
	"regexp"
	"strings"
	"unicode"
)

var (
	titleVocab = regexp.MustCompile(`subasta|licitacion|\brfp\b|\brfq\b|\bnofo\b|solicitud\s+de|propuestas?|\bgrants?\b|subvencion|fondos|\bbecas?\b|convocatoria|oportunidad|funding|aviso|request\s+for|scholarship|fellowship|programa`)
	titleBad   = regexp.MustCompile(`^(?:fecha|date|deadline|emitid|issued|tel|e-?mail|correo|contact|pagina|page|po\s+box|www\.|http|menu|inicio|home|buscar|search|skip|©|copyright|todos\s+los\s+derechos)`)
	longSplit  = regexp.MustCompile(`(?:[.!?]\s+|\s+\|\s+|\s+[-–—]\s+|;\s+)`)
)

func foldString(s string) string {
	r := []rune(s)
	for i := range r {
		r[i] = foldRune(r[i])
	}
	return string(r)
}

func truncRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	cut := n - 1
	for i := cut; i > n/2; i-- {
		if r[i] == ' ' {
			cut = i
			break
		}
	}
	return strings.TrimRight(string(r[:cut]), " ,;:-–") + "…"
}

// findTitle elige la mejor línea del texto original. Devuelve "" si ninguna
// llega al mínimo de confianza.
func (d *doc) findTitle(ref string) string {
	var lines []string
	for _, ln := range strings.Split(string(d.orig), "\n") {
		ln = strings.Join(strings.Fields(ln), " ")
		if len([]rune(ln)) > 200 {
			start := 0
			for _, m := range longSplit.FindAllStringIndex(ln, -1) {
				if m[0] > start {
					lines = append(lines, strings.TrimSpace(ln[start:m[0]]))
				}
				start = m[1]
			}
			lines = append(lines, strings.TrimSpace(ln[start:]))
			continue
		}
		lines = append(lines, ln)
	}
	best, bestScore := "", 0
	seen := 0
	for _, ln := range lines {
		n := len([]rune(ln))
		if n < 8 {
			continue
		}
		seen++
		f := foldString(ln)
		score := 0
		if titleVocab.MatchString(f) {
			score += 3
		}
		if seen <= 15 {
			score++
		}
		if n >= 15 && n <= 160 {
			score++
		}
		if ref != "" && strings.Contains(f, strings.ToLower(ref)) {
			score += 2
		}
		if titleBad.MatchString(f) {
			score -= 4
		}
		if first := []rune(ln)[0]; unicode.IsLower(first) { // continuación de frase
			score -= 3
		}
		if strings.Contains(ln, "@") {
			score -= 2
		}
		if len(strings.Fields(ln)) < 3 {
			score -= 2
		}
		letters := 0
		for _, r := range ln {
			if unicode.IsLetter(r) {
				letters++
			}
		}
		if letters*2 < n { // más números/signos que letras
			score -= 3
		}
		if score > bestScore {
			best, bestScore = ln, score
		}
	}
	if bestScore < 4 {
		return ""
	}
	return truncRunes(best, 160)
}
