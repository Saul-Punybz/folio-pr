package extract

import "regexp"

// Vocabulario "fuerte": frases que casi solo aparecen en una convocatoria.
var strongVocab = []*regexp.Regexp{
	regexp.MustCompile(`subasta\s+(?:formal|informal|publica|general|electronica|abierta)|aviso\s+de\s+(?:subasta|licitacion)|licitacion\s+publica|invitacion\s+a\s+licitar`),
	regexp.MustCompile(`solicitud\s+de\s+(?:propuestas?|cotizaciones?|ofertas|licitaciones)`),
	regexp.MustCompile(`\b(?:rfp|rfq|ifb|nofo|nofa|rfa)\b`),
	regexp.MustCompile(`request\s+for\s+(?:proposals?|quotes?|quotations?|applications?)|invitation\s+for\s+bids?|notice\s+of\s+funding|funding\s+opportunit|call\s+for\s+(?:proposals?|applications?|abstracts?)`),
	regexp.MustCompile(`convocatoria\s+(?:de|para|abierta)|fondos\s+competitivos|oportunidad(?:es)?\s+de\s+(?:financiamiento|fondos|subvencion)|programa\s+de\s+subvenciones`),
	regexp.MustCompile(`se\s+(?:aceptan|reciben|recibiran)\s+(?:propuestas|solicitudes|ofertas)|accepting\s+(?:proposals|applications)|invitamos\s+a\s+(?:someter|presentar)|applications?\s+(?:are\s+)?(?:now\s+)?(?:open|being\s+accepted)`),
}

// Vocabulario "medio": palabras que también salen en menús y noticias.
var mediumVocab = []*regexp.Regexp{
	regexp.MustCompile(`subasta`),
	regexp.MustCompile(`licitacion`),
	regexp.MustCompile(`\bgrants?\b|subvencion|donativo`),
	regexp.MustCompile(`\bbecas?\b|scholarship|fellowship`),
	regexp.MustCompile(`propuestas?\b|proposals?\b`),
	regexp.MustCompile(`\bfondos\b|funding|financiamiento`),
	regexp.MustCompile(`convocatoria`),
	regexp.MustCompile(`applications?\b|\bsolicitud`),
	regexp.MustCompile(`\bbids?\b|ofertas?\b|contrato`),
}

var actionCues = []*regexp.Regexp{
	regexp.MustCompile(`como\s+(?:solicitar|aplicar|participar)|how\s+to\s+apply|para\s+(?:solicitar|participar|someter)`),
	regexp.MustCompile(`deberan?\s+(?:someter|presentar|radicar)|must\s+(?:submit|be\s+submitted)|\bsubmit|\bsometer|\bradicar|\bapply\b`),
	regexp.MustCompile(`requisitos|elegibilidad|eligib|applicants?|aplicantes|proponentes?|licitadores?|bidders?`),
	regexp.MustCompile(`rolling|de\s+manera\s+continua|hasta\s+agotar|open\s+until\s+filled`),
}

// Marcadores de algo que ya pasó o de una noticia que solo lo cuenta.
var pastMarkers = regexp.MustCompile(`\bcelebro\b|\bcelebrada\b|se\s+celebro|\badjudico\b|fue\s+adjudicad|ha\s+sido\s+adjudicad|awarded\s+to|was\s+awarded|notice\s+of\s+award|aviso\s+de\s+adjudicacion|\bcancelad[ao]\b|\bcancell?ed\b|declarad[ao]\s+desiert|\binformo\b|\bdijo\b|segun\s+(?:informo|el\s+comunicado)|se\s+llevo\s+a\s+cabo|took\s+place|was\s+held|ya\s+cerro|ha\s+cerrado|\bclosed\s+on\b|\bnoticias?\b|\bnews\b|according\s+to`)

// Categoría: se cuentan coincidencias (x3 si están en el arranque del texto).
type catRule struct {
	name   string
	weight int
	re     *regexp.Regexp
}

var catRules = []catRule{
	{"scholarship", 3, regexp.MustCompile(`\bbecas?\b|scholarship`)},
	{"fellowship", 3, regexp.MustCompile(`fellowship|\bfellows\b`)},
	{"grant", 3, regexp.MustCompile(`\bgrants?\b|subvencion|fondos\s+competitivos|\bnofo\b|\bnofa\b|notice\s+of\s+funding|donativo|assistance\s+listing|\bcfda\b`)},
	{"rfp", 3, regexp.MustCompile(`subasta|licitacion|\brfp\b|\brfq\b|\brfi\b|\bifb\b|solicitud\s+de\s+(?:propuestas?|cotizaciones?|ofertas)|request\s+for\s+(?:proposals?|quotes?|quotations?)|invitation\s+for\s+bid`)},
	{"proposal", 1, regexp.MustCompile(`call\s+for\s+(?:proposals?|applications?)|convocatoria`)},
	{"contract", 1, regexp.MustCompile(`contrato|contract\s+opportunit|\bsolicitation\b`)},
	{"funding", 1, regexp.MustCompile(`\bfondos\b|funding|financiamiento|capital\s+semilla|incentivo`)},
}

func (d *doc) category() string {
	head := d.fold
	if r := []rune(head); len(r) > 400 {
		head = string(r[:400])
	}
	bestName, best := "other", 0
	for _, c := range catRules {
		n := c.weight * (len(c.re.FindAllStringIndex(d.fold, -1)) + 2*len(c.re.FindAllStringIndex(head, -1)))
		if n > best {
			bestName, best = c.name, n
		}
	}
	return bestName
}

func countMatches(res []*regexp.Regexp, s string) int {
	n := 0
	for _, re := range res {
		if re.MatchString(s) {
			n++
		}
	}
	return n
}

// Avisos que no son una convocatoria nueva: adendas, enmiendas y suplementos.
var amendmentMarkers = regexp.MustCompile(`\badenda\b|\baddend(?:um|a)\b|\bamendment\b|\benmienda\b|fe\s+de\s+erratas|\bsupplement(?:al)?\b|\bsuplemento\b`)
