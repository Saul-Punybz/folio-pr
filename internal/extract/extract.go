// Package extract encuentra, por patrones y sin IA, los datos de una
// convocatoria (subasta, RFP, grant, beca...) en texto de páginas y PDF.
//
// Principio: es preferible dejar un campo vacío que inventarlo. La persona
// que revisa ve los datos resaltados y confirma o descarta.
package extract

import (
	"sort"
	"strings"
	"time"
)

type Found struct {
	IsCandidate  bool       // ¿parece una convocatoria/fondo?
	Score        int        // 0-100, qué tan seguro
	Category     string     // rfp, grant, funding, proposal, contract, fellowship, scholarship, other
	Title        string     // mejor línea de título, ≤160 runas
	Reference    string     // número de subasta/RFP/NOFO, p. ej. "Subasta Formal 24-015"
	Deadline     *time.Time // fecha límite para someter (zona AST; sin hora = 23:59)
	Amount       string     // como aparece, p. ej. "$250,000"
	AmountCents  int64      // en centavos
	ContactEmail string
	ContactPhone string
	Evidence     map[string]string // "deadline","amount","reference","title": fragmento literal (≤200 runas)
}

// Fórmula del score (0-100), cap 100:
//
//	vocabulario: con alguna frase fuerte ("solicitud de propuestas", "subasta
//	  formal", RFP, NOFO...) 24 + 4 por cada frase fuerte distinta extra (máx 3);
//	  si solo hay palabras medias (subasta, fondos, beca...) 10 por cada tipo
//	  distinto (máx 2). Tope 36.
//	+25 fecha límite válida · +15 referencia · +10 monto · +5..8 verbos de
//	  acción (someter, requisitos, elegibilidad) · +3 contacto
//	−25 marcadores de pasado/noticia ("se celebró", "adjudicó", "noticias"),
//	  −10 si aun así hay fecha límite futura
//	−20 adenda/enmienda/suplemento (no es una convocatoria nueva): nunca es
//	  candidata; si trae referencia y fecha límite se devuelven, con
//	  Evidence["amendment"]="1", para que quien llama actualice la original
//	−20 listado (≥3 fechas límite o ≥3 referencias distintas): no es un aviso;
//	  además se dejan vacíos fecha, referencia, monto y título
//	IsCandidate = Score ≥ 45 y la fecha límite, si existe, no ha vencido
//	(un aviso vencido queda con Score ≤ 30).
func Opportunity(text string, now time.Time) Found {
	if now.IsZero() {
		now = time.Now()
	}
	f := Found{Category: "other", Evidence: map[string]string{}}
	d := newDoc(text)
	if len(d.flat) == 0 {
		return f
	}

	strong := countMatches(strongVocab, d.fold)
	medium := countMatches(mediumVocab, d.fold)
	score := 0
	switch {
	case strong > 0:
		extra := strong - 1
		if extra > 3 {
			extra = 3
		}
		score = 24 + 4*extra
	case medium > 0:
		if medium > 2 {
			medium = 2
		}
		score = 10 * medium
	}
	if score > 36 {
		score = 36
	}
	hasVocab := strong+medium > 0
	if hasVocab {
		f.Category = d.category()
	}

	// Fecha límite.
	// Es enmienda si lo dice al comienzo (título/encabezado); un RFP completo
	// que menciona "adenda" en el cuerpo sigue siendo una convocatoria.
	amendment := amendmentMarkers.MatchString(d.fold[:min(len(d.fold), 300)])
	dl, hasDL := d.findDeadline(now, amendment)
	refs := d.findRefs()
	sort.Slice(refs, func(i, j int) bool { return refs[i].s < refs[j].s })
	// Un listado (varias fechas límite o varias referencias distintas) no es
	// un aviso: no se adivina cuál de sus filas es "la" convocatoria.
	listing := (hasDL && dl.distinct >= 3) || len(refs) >= 3

	if !listing {
		if hasDL {
			t := dl.t
			f.Deadline = &t
			f.Evidence["deadline"] = d.evidence(dl.rs, dl.re)
			if hasVocab {
				score += 25
			}
		}
		if len(refs) > 0 {
			f.Reference = refs[0].text
			f.Evidence["reference"] = d.evidence(refs[0].s, refs[0].e)
			if hasVocab {
				score += 15
			}
		}
		if a, ok := d.findAmount(); ok {
			f.Amount, f.AmountCents = a.text, a.cents
			f.Evidence["amount"] = d.evidence(a.s, a.e)
			if hasVocab {
				score += 10
			}
		}
	}

	f.ContactEmail = d.findEmail()
	f.ContactPhone = d.findPhone()

	if hasVocab {
		if n := countMatches(actionCues, d.fold); n > 0 {
			score += 5
			if n >= 3 {
				score += 3
			}
		}
		if f.ContactEmail != "" || f.ContactPhone != "" {
			score += 3
		}
	}

	if pastMarkers.MatchString(d.fold) {
		if f.Deadline != nil && !f.Deadline.Before(now) {
			score -= 10
		} else {
			score -= 25
		}
	}
	if amendmentMarkers.MatchString(d.fold) {
		score -= 20
	}
	if listing {
		score -= 20
	}

	if !listing {
		f.Title = d.findTitle(f.Reference)
	}
	if f.Title != "" {
		f.Evidence["title"] = f.Title
	}

	if score < 0 {
		score = 0
	}
	if score > 100 {
		score = 100
	}
	expired := f.Deadline != nil && f.Deadline.Before(now)
	if expired && score > 30 {
		score = 30
	}
	f.Score = score
	f.IsCandidate = score >= 45 && !expired
	if amendment {
		// Una enmienda no es una convocatoria nueva. Si trae referencia y
		// fecha, se entrega la nueva fecha para que quien llama actualice la
		// convocatoria original.
		f.IsCandidate = false
		if f.Reference != "" && f.Deadline != nil {
			f.Evidence["amendment"] = "1"
		}
	}
	if f.Category == "" {
		f.Category = "other"
	}
	for k, v := range f.Evidence {
		if strings.TrimSpace(v) == "" {
			delete(f.Evidence, k)
		}
	}
	return f
}
