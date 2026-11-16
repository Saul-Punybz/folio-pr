package extract

import (
	"regexp"
	"sort"
	"strconv"
	"time"
)

// Zona de Puerto Rico (AST, UTC-4, sin horario de verano). Una fecha sin hora
// se guarda como 23:59 de ese día, para que una fecha límite "de hoy" no se
// vea vencida antes de tiempo.
var ast = time.FixedZone("AST", -4*3600)

var monthNum = map[string]int{
	"enero": 1, "ene": 1, "january": 1, "jan": 1,
	"febrero": 2, "feb": 2, "february": 2,
	"marzo": 3, "mar": 3, "march": 3,
	"abril": 4, "abr": 4, "april": 4, "apr": 4,
	"mayo": 5, "may": 5,
	"junio": 6, "jun": 6, "june": 6,
	"julio": 7, "jul": 7, "july": 7,
	"agosto": 8, "ago": 8, "august": 8, "aug": 8,
	"septiembre": 9, "setiembre": 9, "sept": 9, "sep": 9, "september": 9,
	"octubre": 10, "oct": 10, "october": 10,
	"noviembre": 11, "nov": 11, "november": 11,
	"diciembre": 12, "dic": 12, "december": 12, "dec": 12,
}

const monthRe = `(?:enero|ene|febrero|feb|marzo|mar|abril|abr|mayo|junio|jun|julio|jul|agosto|ago|septiembre|setiembre|sept|sep|octubre|oct|noviembre|nov|diciembre|dic|january|jan|february|march|april|apr|may|june|july|august|aug|september|october|november|december|dec)`

var (
	reDMY = regexp.MustCompile(`\b(\d{1,2})(?:st|nd|rd|th|ro|ero|er|o)?\.?\s+(?:de\s+)?(` + monthRe + `)\b\.?,?\s+(?:de(?:l)?\s+)?(\d{4})\b`)
	reMDY = regexp.MustCompile(`\b(` + monthRe + `)\b\.?\s+(\d{1,2})(?:st|nd|rd|th)?\b,?\s+(?:de\s+)?(\d{4})\b`)
	// sin año ("30 de noviembre", "1ro de diciembre", "November 30"): solo
	// valen con contexto de entrega; el año se deduce (ver allDates).
	reDM        = regexp.MustCompile(`\b(\d{1,2})(?:st|nd|rd|th|ro|ero|er|o)?\.?\s+de\s+(` + monthRe + `)\b`)
	reMD        = regexp.MustCompile(`\b(enero|febrero|marzo|abril|mayo|junio|julio|agosto|septiembre|setiembre|octubre|noviembre|diciembre|january|february|march|april|june|july|august|september|october|november|december)\s+(\d{1,2})(?:st|nd|rd|th)?\b`)
	reYearAfter = regexp.MustCompile(`^\.?,?\s+(?:de(?:l)?\s+)?\d{4}\b`)
	reSlash     = regexp.MustCompile(`\b(\d{1,2})/(\d{1,2})/(\d{4}|\d{2})\b`)
	reISO       = regexp.MustCompile(`\b(\d{4})-(\d{2})-(\d{2})\b`)

	wk = `(?:(?:lunes|martes|miercoles|jueves|viernes|sabado|domingo|monday|tuesday|wednesday|thursday|friday|saturday|sunday),?\s*)?`
	// hora después de la fecha: "2:00 p.m.", "a las 14:00", "1:00p.m.", "12:00:00 AM"
	reTimeAfterAP = regexp.MustCompile(`^[\s,;@\-]*(?:(?:a\s+las?|at|by|before|antes\s+de\s+las?|hasta\s+las?|no\s+later\s+than)\s+)?(\d{1,2})(?::(\d{2}))?(?::\d{2})?\s?([ap])\.?\s?m\b`)
	reTime24      = regexp.MustCompile(`^[\s,;@\-]*(?:(?:a\s+las?|at|by|before|antes\s+de\s+las?|hasta\s+las?)\s+)?(\d{1,2}):(\d{2})\b`)
	// hora antes de la fecha: "hasta las 2:00 p.m. del 30 de noviembre"
	reTimeBefore = regexp.MustCompile(`(\d{1,2})(?::(\d{2}))?\s?([ap])\.?\s?m\.?\s*(?:del?|el|on|,|-)?\s*` + wk + `$`)
)

type dateHit struct {
	s, e    int // runas en flat
	t       time.Time
	hasTime bool
	noYear  bool // el texto no traía año: se dedujo del "ahora"
}

func (d *doc) allDates(now time.Time) []dateHit {
	var hits []dateHit
	lo, hi := now.Year()-2, now.Year()+5
	add := func(bs, be, y, m, dd int) {
		if y < lo || y > hi || m < 1 || m > 12 || dd < 1 || dd > 31 {
			return
		}
		t := time.Date(y, time.Month(m), dd, 23, 59, 0, 0, ast)
		if t.Day() != dd { // 30 de febrero, etc.
			return
		}
		s, e := d.span(bs, be)
		h := dateHit{s: s, e: e, t: t}
		d.attachTime(&h, bs, be)
		hits = append(hits, h)
	}
	atoi := func(s string) int { n, _ := strconv.Atoi(s); return n }
	for _, m := range reDMY.FindAllStringSubmatchIndex(d.fold, -1) {
		add(m[0], m[1], atoi(d.fold[m[6]:m[7]]), monthNum[d.fold[m[4]:m[5]]], atoi(d.fold[m[2]:m[3]]))
	}
	for _, m := range reMDY.FindAllStringSubmatchIndex(d.fold, -1) {
		add(m[0], m[1], atoi(d.fold[m[6]:m[7]]), monthNum[d.fold[m[2]:m[3]]], atoi(d.fold[m[4]:m[5]]))
	}
	for _, m := range reSlash.FindAllStringSubmatchIndex(d.fold, -1) {
		a, b, y := atoi(d.fold[m[2]:m[3]]), atoi(d.fold[m[4]:m[5]]), atoi(d.fold[m[6]:m[7]])
		if y < 100 {
			y += 2000
		}
		mo, dd := a, b // formato de EE. UU./PR: mes/día
		if a > 12 && b <= 12 {
			mo, dd = b, a
		}
		add(m[0], m[1], y, mo, dd)
	}
	for _, m := range reISO.FindAllStringSubmatchIndex(d.fold, -1) {
		add(m[0], m[1], atoi(d.fold[m[2]:m[3]]), atoi(d.fold[m[4]:m[5]]), atoi(d.fold[m[6]:m[7]]))
	}
	// Sin año: el del "ahora", o el siguiente si esa fecha ya pasó hace más
	// de 60 días. Si hay una fecha completa en el mismo sitio, esa gana (la
	// deduplicación de abajo deja la más larga).
	addNY := func(bs, be, m, dd int) {
		if reYearAfter.MatchString(d.fold[be:min(len(d.fold), be+24)]) {
			return // trae año (aunque sea absurdo): no se adivina
		}
		if m < 1 || m > 12 || dd < 1 || dd > 31 {
			return
		}
		y := now.Year()
		t := time.Date(y, time.Month(m), dd, 23, 59, 0, 0, ast)
		if t.Day() != dd {
			return
		}
		if t.Before(now.AddDate(0, 0, -60)) {
			y++
			t = time.Date(y, time.Month(m), dd, 23, 59, 0, 0, ast)
			if t.Day() != dd {
				return
			}
		}
		s, e := d.span(bs, be)
		h := dateHit{s: s, e: e, t: t, noYear: true}
		d.attachTime(&h, bs, be)
		hits = append(hits, h)
	}
	for _, m := range reDM.FindAllStringSubmatchIndex(d.fold, -1) {
		addNY(m[0], m[1], monthNum[d.fold[m[4]:m[5]]], atoi(d.fold[m[2]:m[3]]))
	}
	for _, m := range reMD.FindAllStringSubmatchIndex(d.fold, -1) {
		addNY(m[0], m[1], monthNum[d.fold[m[2]:m[3]]], atoi(d.fold[m[4]:m[5]]))
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].s < hits[j].s || (hits[i].s == hits[j].s && hits[i].e > hits[j].e) })
	out := hits[:0]
	last := -1
	for _, h := range hits {
		if h.s >= last {
			out = append(out, h)
			last = h.e
		}
	}
	return out
}

// attachTime busca una hora justo después (o justo antes) de la fecha.
func (d *doc) attachTime(h *dateHit, bs, be int) {
	rest := d.fold[be:]
	if len(rest) > 48 {
		rest = rest[:48]
	}
	set := func(hh, mm int, pm bool, ampm bool) bool {
		if ampm {
			if hh < 1 || hh > 12 {
				return false
			}
			hh %= 12
			if pm {
				hh += 12
			}
		}
		if hh > 23 || mm > 59 {
			return false
		}
		h.t = time.Date(h.t.Year(), h.t.Month(), h.t.Day(), hh, mm, 0, 0, ast)
		h.hasTime = true
		return true
	}
	if m := reTimeAfterAP.FindStringSubmatch(rest); m != nil {
		mm := 0
		if m[2] != "" {
			mm, _ = strconv.Atoi(m[2])
		}
		hh, _ := strconv.Atoi(m[1])
		if set(hh, mm, m[3] == "p", true) {
			return
		}
	}
	if m := reTime24.FindStringSubmatch(rest); m != nil {
		hh, _ := strconv.Atoi(m[1])
		mm, _ := strconv.Atoi(m[2])
		if set(hh, mm, false, false) {
			return
		}
	}
	pre := d.fold[:bs]
	if len(pre) > 40 {
		pre = pre[len(pre)-40:]
	}
	if m := reTimeBefore.FindStringSubmatch(pre); m != nil {
		mm := 0
		if m[2] != "" {
			mm, _ = strconv.Atoi(m[2])
		}
		hh, _ := strconv.Atoi(m[1])
		set(hh, mm, m[3] == "p", true)
	}
}

type cue struct {
	re       *regexp.Regexp
	strength int // 3 = "fecha límite"/deadline, 2 = hasta/cierre/entrega/apertura
}

var deadlineCues = []cue{
	{regexp.MustCompile(`fecha\s+(?:y\s+hora\s+)?(?:limite|de\s+(?:cierre|vencimiento|entrega|radicacion|sometimiento|recibo))|en\s+o\s+antes\s+d|\bdeadline|closing\s+date|close\s+date|due\s+date|\bdue\b|\blimite\b|plazo\s+(?:para|hasta)`), 3},
	{regexp.MustCompile(`\bhasta\s+(?:el|las?|este)\b|\bcierre\b|\bcierra\b|\bvence\b|vencimiento|\bcloses?\b|closing|submit(?:ted)?\s+(?:by|before|on)|(?:received|postmarked|delivered)\s+(?:by|before|on\s+or\s+before)|applications?\s+(?:are\s+)?(?:due|accepted\s+until)|(?:entrega|recibo|recepcion|presentacion|radicacion|sometimiento)\s+de\s+(?:las?\s+)?(?:propuestas?|licitaciones|ofertas|solicitudes|cotizaciones|pliegos?)|apertura\s+de\s+(?:la\s+)?(?:subasta|licitacion|propuestas|ofertas|pliegos)|acto\s+de\s+apertura|fecha\s+(?:de\s+)?apertura|\bbids?\s+(?:are\s+)?(?:due|open)|proposals?\s+(?:are\s+)?due|recibiran?\s+(?:propuestas|ofertas|licitaciones)|antes\s+de\s+las?\s+\d|\bon\s+or\s+before\b|\bbefore\b|no\s+later\s+than|a\s+mas\s+tardar|no\s+mas\s+tarde`), 2},
}

var notDeadlineCue = regexp.MustCompile(`pre[\s-]?subasta|pre[\s-]?licitacion|pre[\s-]?bid|pre[\s-]?propuesta|pre[\s-]?conferencia|reunion|conferencia|visita\s+(?:al?\s+)?(?:el\s+)?(?:sitio|lugar|proyecto|terreno|facilidad)|site\s+visit|webinar|taller|orientacion|informativa|emitid[ao]|publicad[ao]|publicacion|fecha\s+pliegos?|\bposted\b|\bissued\b|\bpublished\b|\bposting\s+date|preguntas|consultas|questions|inquiries|adjudicacion|\baward\s+date|notificacion|inicio|comienzo|start\s+date|begin|vigencia|archive|\bfecha\s+de\s+este\b|actualizad|modificad|updated|revised|desde\s+el`)

// Contexto de ENTREGA de propuestas/ofertas/solicitudes (someter, radicar,
// recibir...). Una fecha con este contexto es la fecha límite aunque la frase
// también hable de ejecución.
const objRe = `(?:propuestas?|ofertas?|licitaciones|solicitudes|cotizaciones|pliegos?|proposals?|applications?|bids?|quotes?|quotations?)`
const verbRe = `(?:someter|sometid|sometimiento|entreg|radic|recib|recepcion|present(?:ar|ad[oa]s?|acion)|enviar|enviad|submit|submission|deliver|receiv|received|filed?\b)`

var deliveryCtx = regexp.MustCompile(verbRe + `\w*\s+(?:[\w"'()/-]+\s+){0,6}?` + objRe + `|` +
	objRe + `\s+(?:[\w"'()/-]+\s+){0,6}?(?:deberan?\s+ser\s+|seran\s+|ser\s+|must\s+be\s+|are\s+(?:due|to\s+be)\s+)?` + verbRe + `|` +
	`(?:fecha|plazo|cierre|deadline|due\s+date|closing(?:\s+date)?|due)\s+(?:limite\s+)?(?:de\s+|para\s+|for\s+|of\s+)?(?:la\s+)?(?:` + verbRe + `\w*|` + objRe + `)|` +
	`\b` + objRe + `\s+(?:are\s+)?due\b|\bdue\s+(?:date\s+)?(?:for|of)\s+` + objRe)

// Contexto de EJECUCIÓN del proyecto o contrato: no es fecha de entrega.
var execCtx = regexp.MustCompile(`\bcomplet(?:ar|ado|ada|ara|e|ion|ando)|\btermin(?:ar|ado|ada|ara|e|acion)|\bfinaliz(?:ar|ado|ada|ara|e|acion)|\bculmin|vigencia|periodo\s+de\s+(?:desempeno|ejecucion|vigencia|contrato|contratacion|servicio)|desempeno|entregables?|deliverables?|\bhitos?\b|milestones?|\bejecut(?:ar|ado|ada|ara|e|ion)|\breclut|\bimplement|` +
	`(?:vigencia|duracion|termino|periodo|plazo|fin|vencimiento|expiracion)\s+(?:del?\s+)?(?:contrato|acuerdo)|contrato\s+(?:comenzara|vencera|terminara|expira|finaliza|sera\s+efectivo)|firma\s+del\s+contrato|contract\s+(?:term|period|end|expir|start)|period\s+of\s+performance|performance\s+period|project\s+(?:start|end|period)|informes?\s+(?:final|parcial|trimestral|mensual|semestral|anual)|(?:final|progress|interim)\s+reports?|\bcompleted?\b|\bcomplete\s+the`)

// Palabras de una enmienda que mueve la fecha.
var extensionCtx = regexp.MustCompile(`nueva\s+fecha|nuevo\s+plazo|nuevo\s+termino|se\s+extiende|extiend[ae]|extension|extendid|extended|extending|prorrog|pospon|pospuest|aplaz|posterg|cambia\s+a|se\s+modifica|new\s+(?:deadline|due\s+date|closing\s+date)|postponed|rescheduled|reprogram|debe\s+decir|leera|should\s+read|now\s+read`)

type dateClass struct {
	strength int  // ver classifyDate
	deliv    bool // contexto explícito de entrega de propuestas/ofertas
	ext      bool // contexto de "se extiende / nueva fecha"
}

// classifyDate devuelve la fuerza (0 = no es fecha límite, -1 = es otra fecha
// reconocida como tal, p. ej. pre-subasta; >0 = fecha límite).
func (d *doc) classifyBase(h dateHit) int {
	cs := d.clauseStart(h.s, 140)
	pre := d.sub(cs, h.s)
	best, bestEnd, strongEnd := 0, -1, -1
	preLen := len([]rune(pre))
	for _, c := range deadlineCues {
		for _, m := range c.re.FindAllStringIndex(pre, -1) {
			e := len([]rune(pre[:m[1]]))
			if e > bestEnd {
				bestEnd = e
			}
			if c.strength == 3 && e > strongEnd {
				strongEnd = e
			}
			if preLen-e <= 60 && c.strength > best {
				best = c.strength // la señal más fuerte cercana ("fecha límite ... hasta el")
			}
		}
	}
	nbest := -1
	for _, m := range notDeadlineCue.FindAllStringIndex(pre, -1) {
		if e := len([]rune(pre[:m[1]])); e > nbest {
			nbest = e
		}
	}
	if bestEnd >= 0 && preLen-bestEnd <= 60 && bestEnd >= nbest {
		return best
	}
	// "La fecha límite para someter las propuestas a través del enlace … es el 5 de noviembre"
	if strongEnd >= 0 && preLen-strongEnd <= 130 && strongEnd >= nbest {
		return 3
	}
	if nbest >= 0 && preLen-nbest <= 60 {
		return -1
	}
	// Etiqueta después de la fecha: "9 de noviembre de 2026 1:00p.m. Acceder Acto de Apertura"
	ce := d.clauseEnd(h.e, 60)
	post := d.sub(h.e, ce)
	if len([]rune(post)) > 48 {
		post = string([]rune(post)[:48])
	}
	pBest, pStart := 0, 1<<30
	for _, c := range deadlineCues {
		if m := c.re.FindStringIndex(post); m != nil && m[0] < pStart {
			pBest, pStart = c.strength, m[0]
		}
	}
	if m := notDeadlineCue.FindStringIndex(post); m != nil && m[0] < pStart {
		return -1
	}
	if pBest > 0 {
		return pBest
	}
	return 0
}

// classifyDate clasifica una fecha: strength como en classifyBase, más el
// contexto. Una fecha con contexto de ejecución (completar, vigencia,
// entregables, período de desempeño...) y sin contexto de entrega no es fecha
// límite de la convocatoria sino un hito del proyecto: "No más tarde del 30 de
// septiembre de 2027, completar y documentar el reclutamiento".
func (d *doc) classifyDate(h dateHit) dateClass {
	s := d.classifyBase(h)
	if s <= 0 {
		return dateClass{strength: s}
	}
	pre := d.sub(d.clauseStart(h.s, 140), h.s)
	post := d.sub(h.e, d.clauseEnd(h.e, 60))
	if r := []rune(post); len(r) > 60 {
		post = string(r[:60])
	}
	tail := func(x string, n int) string {
		if r := []rune(x); len(r) > n {
			return string(r[len(r)-n:])
		}
		return x
	}
	deliv := deliveryCtx.MatchString(pre) || deliveryCtx.MatchString(post)
	if !deliv && execCtx.MatchString(tail(pre, 110)+" | "+post) {
		return dateClass{strength: -1}
	}
	c := dateClass{strength: s, deliv: deliv, ext: extensionCtx.MatchString(tail(pre, 110) + " | " + post)}
	if deliv && c.strength < 3 {
		c.strength = 3
	}
	return c
}

type deadlineResult struct {
	t        time.Time
	rs, re   int
	distinct int // fechas límite distintas encontradas
}

// findDeadline elige la fecha de ENTREGA de propuestas. En una enmienda, si
// alguna fecha viene con contexto de "se extiende / nueva fecha", solo cuentan
// esas.
func (d *doc) findDeadline(now time.Time, amendment bool) (deadlineResult, bool) {
	type cand struct {
		h dateHit
		c dateClass
	}
	var cands []cand
	seen := map[string]bool{}
	for _, h := range d.allDates(now) {
		c := d.classifyDate(h)
		if c.strength <= 0 {
			continue
		}
		if h.noYear && !(c.deliv || c.strength == 3) {
			continue // sin año solo con contexto claro de entrega
		}
		cands = append(cands, cand{h, c})
		seen[h.t.Format("2006-01-02")] = true
	}
	if len(cands) == 0 {
		return deadlineResult{}, false
	}
	if amendment {
		var ext []cand
		for _, c := range cands {
			if c.c.ext {
				ext = append(ext, c)
			}
		}
		if len(ext) > 0 {
			cands = ext
		}
	}
	// Con fecha de entrega explícita, las demás (sin ese contexto) no cuentan.
	var dv []cand
	for _, c := range cands {
		if c.c.deliv {
			dv = append(dv, c)
		}
	}
	if len(dv) > 0 {
		cands = dv
	}
	top := 0
	for _, c := range cands {
		if c.c.strength > top {
			top = c.c.strength
		}
	}
	// Entre las de mayor fuerza: la más próxima que aún no venció; si todas
	// vencieron, la más reciente.
	var pick *dateHit
	for i := range cands {
		c := &cands[i]
		if c.c.strength != top {
			continue
		}
		switch {
		case pick == nil:
			pick = &c.h
		case c.h.hasTime && !pick.hasTime && c.h.t.Format("2006-01-02") == pick.t.Format("2006-01-02"):
			pick = &c.h // mismo día, pero esta mención trae la hora
		case !c.h.t.Before(now) && (pick.t.Before(now) || c.h.t.Before(pick.t)):
			pick = &c.h
		case c.h.t.Before(now) && pick.t.Before(now) && c.h.t.After(pick.t):
			pick = &c.h
		}
	}
	return deadlineResult{t: pick.t, rs: pick.s, re: pick.e, distinct: len(seen)}, true
}
