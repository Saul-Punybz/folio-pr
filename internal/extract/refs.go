package extract

import (
	"regexp"
	"strconv"
	"strings"
)

const refLabel = `(?:aviso\s+de\s+)?(?:subasta(?:\s+(?:formal|informal|publica|general|electronica|abierta))?|licitacion(?:\s+(?:publica|formal|informal))?|solicitud\s+de\s+(?:propuestas?|cotizaciones?|ofertas|informacion)|invitacion\s+a\s+licitar|convocatoria|request\s+for\s+(?:proposals?|quotes?|quotations?|applications?|information)|notice\s+of\s+funding\s+opportunity|funding\s+opportunity|opportunity|rfp|rfq|rfi|ifb|nofo|nofa|rfa)`

// Segmentos alfanuméricos unidos por guion; tolera espacios sobrantes de PDF
// ("4671 -2026-001"). El guion con espacios solo une segmentos con algún
// dígito (ver trimRefTail): "RFP 2026-001 - Academia" no se come el título.
const refID = `[a-z0-9]{1,10}(?:(?:\s?-\s?|/)[a-z0-9]{1,10}){1,8}`

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
		idEnd := trimRefTail(d.fold, m[6], m[7])
		// "OCE-RFP-24-03-Enmienda1" es la enmienda de "OCE-RFP-24-03".
		if loc := reAmendSuffix.FindStringIndex(d.fold[m[6]:idEnd]); loc != nil {
			idEnd = m[6] + loc[0]
		}
		id := strings.Join(strings.Fields(d.fold[m[6]:idEnd]), "")
		if !validRefID(id) {
			continue
		}
		ls, le := d.span(m[2], m[3])
		is, ie := d.span(m[6], idEnd)
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
		out2 := refHit{s: ls, e: ie, id: id, text: label + sep + reSpacedDash.ReplaceAllString(d.flatStr(is, ie), "$1")}
		push(out2)
	}
	for _, re := range []*regexp.Regexp{reRefASG, reRefParen} {
		for _, m := range re.FindAllStringSubmatchIndex(d.fold, -1) {
			is, ie := d.span(m[2], m[3])
			push(refHit{s: is, e: ie, id: d.fold[m[2]:m[3]], text: strings.ToUpper(d.flatStr(is, ie))})
		}
	}
	return dropPrefixRefs(out)
}

var reAmendSuffix = regexp.MustCompile(`-(?:enmienda|adenda|amendment|addendum|mod)\d*$`)

var reSpacedDash = regexp.MustCompile(`\s*([-/])\s*`)

// trimRefTail recorta del final del id los segmentos unidos con espacios
// alrededor del guion que no traen ningún dígito (casi seguro es texto).
func trimRefTail(fold string, start, end int) int {
	for end > start {
		i := strings.LastIndexAny(fold[start:end], "-/")
		if i < 0 {
			return end
		}
		i += start
		seg := fold[i+1 : end]
		before := i > start && fold[i-1] == ' '
		after := strings.HasPrefix(seg, " ")
		// " - " con espacio a ambos lados es un guion de puntuación, no del id.
		if !(before && after) && (!(before || after) || strings.ContainsAny(seg, "0123456789")) {
			return end
		}
		end = i
		for end > start && fold[end-1] == ' ' {
			end--
		}
	}
	return end
}

// dropPrefixRefs quita la referencia que es solo el comienzo de otra más
// larga ("…-4671" frente a "…-4671-2026-001"): es la misma convocatoria.
func dropPrefixRefs(in []refHit) []refHit {
	out := make([]refHit, 0, len(in))
	for i, h := range in {
		cut := false
		for j, o := range in {
			if i != j && len(o.id) > len(h.id) && strings.HasPrefix(o.id, h.id) && (o.id[len(h.id)] == '-' || o.id[len(h.id)] == '/') {
				cut = true
				break
			}
		}
		if !cut {
			out = append(out, h)
		}
	}
	return out
}

var (
	reNormNum    = regexp.MustCompile(`\b(?:NUMERO|NUMBER|NUM|NRO|NO)(?:\.\s*|:\s*|\s+)|#\s*`)
	reNormDash   = regexp.MustCompile(`\s*([-/])\s*`)
	reNormSpaces = regexp.MustCompile(`\s+`)
	reNormPrefix = regexp.MustCompile(`^(?:AVISO DE )?(SUBASTA(?: FORMAL| INFORMAL| PUBLICA| GENERAL| ELECTRONICA| ABIERTA)?|LICITACION(?: PUBLICA| FORMAL| INFORMAL)?|SOLICITUD DE PROPUESTAS?|REQUEST FOR PROPOSALS?|RFP|SOLICITUD DE COTIZACIONES?|REQUEST FOR (?:QUOTES?|QUOTATIONS?)|RFQ|SOLICITUD DE OFERTAS|INVITACION A LICITAR|IFB|SOLICITUD DE INFORMACION|REQUEST FOR INFORMATION|RFI|REQUEST FOR APPLICATIONS?|RFA|NOTICE OF FUNDING OPPORTUNITY|FUNDING OPPORTUNITY|OPPORTUNITY|NOFO|NOFA|CONVOCATORIA)(?:[\s-]+(.*))?$`)
)

var refPrefixCanon = map[string]string{
	"SOLICITUD DE PROPUESTA": "RFP", "SOLICITUD DE PROPUESTAS": "RFP", "REQUEST FOR PROPOSAL": "RFP", "REQUEST FOR PROPOSALS": "RFP",
	"SOLICITUD DE COTIZACION": "RFQ", "SOLICITUD DE COTIZACIONES": "RFQ", "REQUEST FOR QUOTE": "RFQ", "REQUEST FOR QUOTES": "RFQ",
	"REQUEST FOR QUOTATION": "RFQ", "REQUEST FOR QUOTATIONS": "RFQ", "SOLICITUD DE OFERTAS": "RFQ",
	"INVITACION A LICITAR": "IFB", "SOLICITUD DE INFORMACION": "RFI", "REQUEST FOR INFORMATION": "RFI",
	"REQUEST FOR APPLICATION": "RFA", "REQUEST FOR APPLICATIONS": "RFA",
	"NOTICE OF FUNDING OPPORTUNITY": "NOFO", "FUNDING OPPORTUNITY": "NOFO", "OPPORTUNITY": "NOFO", "NOFA": "NOFO",
}

// NormalizeReference deja una referencia en una forma comparable: mayúsculas,
// sin acentos, sin espacios alrededor de guiones, sin "Núm."/"No."/"#" y con
// el prefijo unificado ("Solicitud de Propuestas" y "RFP" son "RFP"; todas las
// subastas, "SUBASTA"; "Aviso de" se quita). Sirve para que quien deduplique
// convocatorias compare "RFP PRPA-FEMA-PA-DR-4671 -2026-001" con
// "Solicitud de propuestas Núm. PRPA-FEMA-PA-DR-4671-2026-001".
func NormalizeReference(ref string) string {
	var sb strings.Builder
	for _, r := range strings.TrimSpace(ref) {
		sb.WriteRune(foldRune(r))
	}
	s := strings.ToUpper(sb.String())
	s = reNormSpaces.ReplaceAllString(s, " ")
	s = reNormNum.ReplaceAllString(s, "")
	s = reNormDash.ReplaceAllString(s, "$1")
	s = strings.Trim(s, " .,;:")
	if m := reNormPrefix.FindStringSubmatch(s); m != nil {
		pre := m[1]
		switch {
		case strings.HasPrefix(pre, "SUBASTA"):
			pre = "SUBASTA"
		case strings.HasPrefix(pre, "LICITACION"):
			pre = "LICITACION"
		default:
			if c, ok := refPrefixCanon[pre]; ok {
				pre = c
			}
		}
		rest := strings.Trim(m[2], " -.,;:")
		if rest == "" {
			return pre
		}
		return pre + " " + rest
	}
	return s
}
