package extract

import (
	"regexp"
	"strconv"
	"strings"
)

// Cifra con símbolo: "$1,500,000", "US$250,000.00", "$1.5 millones", "$2M".
// El multiplicador solo cuenta si va pegado o a un solo espacio de la cifra
// y es una palabra completa: "$500 mensuales" son 500 dólares.
var reAmount = regexp.MustCompile(
	`(?:\bus\s?\$|\busd\s?\$?\s?|\$)\s?(\d{1,3}(?:,\d{3})+|\d+)(?:\.(\d+))?` +
		`(?:\s?(millones|millon|million|billones|billon|billion|mil|thousand)\b|(mm|m|k|b)\b)?`)

var reAmountWords = regexp.MustCompile(`\b(\d{1,3}(?:,\d{3})+(?:\.\d{1,2})?)\s+(?:dolares|dollars|usd)\b`)

var multWords = map[string]int64{
	"millones": 1e6, "millon": 1e6, "million": 1e6, "mm": 1e6, "m": 1e6,
	"billones": 1e9, "billon": 1e9, "billion": 1e9, "b": 1e9,
	"mil": 1e3, "thousand": 1e3, "k": 1e3,
}

var (
	amountPos4       = regexp.MustCompile(`cuantia|presupuesto|asignacion|funding\s+available|available\s+funding|fondos\s+disponibles|total\s+(?:program\s+)?funding|estimated\s+(?:total\s+)?(?:program\s+)?funding|valor\s+total|funds?\s+available`)
	amountPos3       = regexp.MustCompile(`asignado|\bawards?\b|\bmonto|ceiling|valor\s+estimado|subvencion|grant\s+(?:amount|of)|financiamiento|disponible|ayuda\s+de|premio|stipend|\bbeca\s+de`)
	amountPos2       = regexp.MustCompile(`cantidad|\btotal\b|estimado|estimated|\bfondos\b|funding|importe|\bvalor\b|\bmaximo|maximum|por\s+un\s+total`)
	amountUpTo       = regexp.MustCompile(`(?:\bhasta|\bup\s+to|\bde\s+hasta)\s*(?:un\s+)?(?:maximo\s+de\s+)?$`)
	amountNeg        = regexp.MustCompile(`fianza|\bbond\b|garantia|costo\s+d|cost\s+of|cuesta|pliego|registro|inscripcion|matricula|\bcargo|\bfees?\b|tarifa|arancel|\bcuota|deposito|\bdeposit|multa|penalidad|penalty|salario|salary|sueldo|reembols|ejemplo|example|e\.g|no\s+representa|does\s+not\s+represent|not\s+the\s+(?:available|total)|costo\s+unitario|\bseguro|insurance|por\s+ciento|\bpareo|cost\s+share|match`)
	amountExample    = regexp.MustCompile(`ejemplo|example|for\s+instance|supongamos|suppose|illustrat`)
	amountDisclaimer = regexp.MustCompile(`^[\s.]*(?:esta|dicha|esa)\s+(?:cantidad|suma|cifra)[^.]{0,220}?no\s+(?:representa|es\s+el)|^[\s.]*(?:this|that)\s+(?:amount|figure|total)[^.]{0,220}?(?:does\s+not|is\s+not)`)
	amountNegAfter   = regexp.MustCompile(`^[\s/\-]{0,3}(?:mensual|semanal|diari|anual\s+por|al\s+mes|por\s+(?:hora|dia|mes|ano|semana|milla|empleado|persona|participante|pagina|copia|unidad|cada)|per\s+(?:hour|day|month|year|week|mile|person|participant|unit|diem|each)|monthly|weekly|hourly|(?:mo|month|mes|hr|hour|ano|year|dia|day)\b|de\s+fianza|de\s+matricula|de\s+cargo|de\s+deposito|fee|cada\b|each\b)`)
)

type amountHit struct {
	s, e     int
	text     string
	cents    int64
	tier     int // 3 fuerte, 2 normal, 1 "hasta", 0 sin contexto
	excluded bool
}

func parseAmount(intPart, frac, mult string) (int64, bool) {
	ip := strings.ReplaceAll(intPart, ",", "")
	if len(ip) > 13 {
		return 0, false
	}
	n, err := strconv.ParseInt(ip, 10, 64)
	if err != nil {
		return 0, false
	}
	m := int64(1)
	if mult != "" {
		m = multWords[mult]
	}
	dollars := n * m
	var cents int64
	if frac != "" {
		if len(frac) > 9 {
			frac = frac[:9]
		}
		f, _ := strconv.ParseInt(frac, 10, 64)
		pow := int64(1)
		for range frac {
			pow *= 10
		}
		if m == 1 {
			c := frac
			if len(c) < 2 {
				c += "0"
			}
			cc, _ := strconv.ParseInt(c[:2], 10, 64)
			cents = cc
		} else {
			dollars += f * m / pow
		}
	}
	if dollars > 1e12 {
		return 0, false
	}
	return dollars*100 + cents, true
}

func (d *doc) findAmount() (amountHit, bool) {
	var hits []amountHit
	for _, m := range reAmount.FindAllStringSubmatchIndex(d.fold, -1) {
		get := func(i int) string {
			if m[2*i] < 0 {
				return ""
			}
			return d.fold[m[2*i]:m[2*i+1]]
		}
		// "1.500.000" o "1,5": formato ambiguo, mejor vacío que inventado.
		if m[1] < len(d.fold)-1 && (d.fold[m[1]] == '.' || d.fold[m[1]] == ',') && d.fold[m[1]+1] >= '0' && d.fold[m[1]+1] <= '9' {
			continue
		}
		mult := get(3)
		if mult == "" {
			mult = get(4)
		}
		cents, ok := parseAmount(get(1), get(2), mult)
		if !ok {
			continue
		}
		rs, re := d.span(m[0], m[1])
		hits = append(hits, amountHit{s: rs, e: re, cents: cents})
	}
	for _, m := range reAmountWords.FindAllStringSubmatchIndex(d.fold, -1) {
		num := d.fold[m[2]:m[3]]
		ip, fr, _ := strings.Cut(num, ".")
		cents, ok := parseAmount(ip, fr, "")
		if !ok {
			continue
		}
		rs, re := d.span(m[0], m[1])
		hits = append(hits, amountHit{s: rs, e: re, cents: cents})
	}
	if len(hits) == 0 {
		return amountHit{}, false
	}
	distinct := map[int64]bool{}
	for i := range hits {
		h := &hits[i]
		h.text = strings.TrimRight(d.flatStr(h.s, h.e), " .,;:")
		if h.cents < 100*100 { // < $100
			h.excluded = true
			continue
		}
		cs := d.clauseStart(h.s, 70)
		ce := d.clauseEnd(h.e, 40)
		before := d.sub(cs, h.s)
		after := d.sub(h.e, ce)
		if amountExample.MatchString(d.sub(d.clauseStart(h.s, 220), h.s)) {
			h.excluded = true
			continue
		}
		nb := before
		if r := []rune(nb); len(r) > 45 {
			nb = string(r[len(r)-45:])
		}
		if amountNeg.MatchString(nb) || amountNegAfter.MatchString(after) {
			h.excluded = true
			continue
		}
		if r := []rune(after); len(r) > 25 {
			after = string(r[:25])
		}
		if amountNeg.MatchString(after) {
			h.excluded = true
			continue
		}
		if amountDisclaimer.MatchString(d.sub(h.e, h.e+300)) {
			h.excluded = true // "…$2,506,930. Esta cantidad … no representa el presupuesto"
			continue
		}
		distinct[h.cents] = true
		short := before
		if r := []rune(short); len(r) > 60 {
			short = string(r[len(r)-60:])
		}
		switch {
		case amountPos4.MatchString(short) || amountPos4.MatchString(after):
			h.tier = 4
		case amountPos3.MatchString(short) || amountPos3.MatchString(after):
			h.tier = 3
		case amountUpTo.MatchString(before):
			h.tier = 3
		case amountPos2.MatchString(short):
			h.tier = 2
		}
	}
	var best *amountHit
	for i := range hits {
		h := &hits[i]
		if h.excluded {
			continue
		}
		if best == nil || h.tier > best.tier {
			best = h
		}
	}
	if best == nil {
		return amountHit{}, false
	}
	if best.tier == 0 && (len(distinct) != 1 || best.cents < 1000*100) {
		return amountHit{}, false // sin contexto y ambiguo: se deja vacío
	}
	return *best, true
}

// AmountCents convierte un monto escrito a mano ("$50,000", "$1.5 millones",
// "250,000 dólares") a centavos. Es el único parser de montos de Folio: usa
// las mismas reglas que Opportunity y, si el texto es solo la cifra (sin
// contexto), toma la primera. 0 si no hay cifra reconocible.
func AmountCents(s string) int64 {
	d := newDoc(s)
	if len(d.flat) == 0 {
		return 0
	}
	if a, ok := d.findAmount(); ok {
		return a.cents
	}
	if m := reAmount.FindStringSubmatch(d.fold); m != nil {
		mult := m[3]
		if mult == "" {
			mult = m[4]
		}
		if c, ok := parseAmount(m[1], m[2], mult); ok {
			return c
		}
	}
	if m := reAmountWords.FindStringSubmatch(d.fold); m != nil {
		ip, fr, _ := strings.Cut(m[1], ".")
		if c, ok := parseAmount(ip, fr, ""); ok {
			return c
		}
	}
	return 0
}
