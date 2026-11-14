package extract

import (
	"regexp"
	"strconv"
	"strings"
)

const refLabel = `(?:aviso\s+de\s+)?(?:subasta(?:\s+(?:formal|informal|publica|general|electronica|abierta))?|licitacion(?:\s+(?:publica|formal|informal))?|solicitud\s+de\s+(?:propuestas?|cotizaciones?|ofertas|informacion)|invitacion\s+a\s+licitar|convocatoria|request\s+for\s+(?:proposals?|quotes?|quotations?|applications?|information)|notice\s+of\s+funding\s+opportunity|funding\s+opportunity|opportunity|rfp|rfq|rfi|ifb|nofo|nofa|rfa)`
const refID = `[a-z0-9]{1,10}(?:[-/][a-z0-9]{1,10}){1,4}`

var (
	reRefLabeled = regexp.MustCompile(`\b(` + refLabel + `)(?:\s+(numero|number|id|num|no|nro))?\.?\s*[:#]?[\s\-]*(` + refID + `)\b`)
	reRefASG     = regexp.MustCompile(`\b(\d{2}j-\d{4,5}(?:-r\d+)?)\b`)
	reRefParen   = regexp.MustCompile(`\(\s*(\d{2}-(?:sp|rfp|rfq|rfi)-\d{2,4})\s*\)`)
)

type refHit struct {
	s, e int
	text string
	id   string
}

func validRefID(id string) bool {
	segs := strings.FieldsFunc(id, func(r rune) bool { return r == '-' || r == '/' })
	if len(segs) < 2 {
		return false
	}
	hasDigit, allNum := false, true
	for _, sg := range segs {
		if monthNum[sg] != 0 {
			return false
		}
		if _, err := strconv.Atoi(sg); err != nil {
			allNum = false
		}
		for _, r := range sg {
			if r >= '0' && r <= '9' {
				hasDigit = true
			}
		}
	}
	if !hasDigit {
		return false
	}
	if allNum {
		n := make([]int, len(segs))
		for i, sg := range segs {
			n[i], _ = strconv.Atoi(sg)
		}
		l := func(i int) int { return len(segs[i]) }
		// rangos de años ("2026-2027") y fechas
		if len(segs) == 2 && l(0) == 4 && l(1) == 4 && n[1]-n[0] == 1 {
			return false
		}
		if len(segs) == 3 && ((l(0) == 4 && l(1) <= 2 && l(2) <= 2 && n[1] >= 1 && n[1] <= 12 && n[2] >= 1 && n[2] <= 31) ||
			(l(2) == 4 && l(0) <= 2 && l(1) <= 2 && n[0] >= 1 && n[1] >= 1 && (n[0] <= 12 || n[1] <= 12))) {
			return false
		}
		// teléfonos 787-722-0590
		if len(segs) == 3 && l(0) == 3 && l(1) == 3 && l(2) == 4 {
			return false
		}
	}
	return true
}

func (d *doc) findRefs() []refHit {
	var out []refHit
	seen := map[string]bool{}
	push := func(h refHit) {
		k := strings.ToLower(h.id)
		if !seen[k] {
			seen[k] = true
			out = append(out, h)
		}
	}
	for _, m := range reRefLabeled.FindAllStringSubmatchIndex(d.fold, -1) {
		id := d.fold[m[6]:m[7]]
		if !validRefID(id) {
			continue
		}
		ls, le := d.span(m[2], m[3])
		is, ie := d.span(m[6], m[7])
		label := d.flatStr(ls, le)
		if m[4] >= 0 { // "Opportunity Number": se conserva la palabra completa
			if w := d.fold[m[4]:m[5]]; w == "number" || w == "numero" || w == "id" {
				_, we := d.span(m[4], m[5])
				label = d.flatStr(ls, we)
			}
		}
		between := strings.TrimSpace(d.sub(le, is))
		sep := " "
		if between == "-" {
			sep = "-"
		}
		out2 := refHit{s: ls, e: ie, id: id, text: label + sep + d.flatStr(is, ie)}
		push(out2)
	}
	for _, re := range []*regexp.Regexp{reRefASG, reRefParen} {
		for _, m := range re.FindAllStringSubmatchIndex(d.fold, -1) {
			is, ie := d.span(m[2], m[3])
			push(refHit{s: is, e: ie, id: d.fold[m[2]:m[3]], text: strings.ToUpper(d.flatStr(is, ie))})
		}
	}
	return out
}
