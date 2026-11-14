package extract

import (
	"regexp"
	"strings"
)

var (
	reEmail = regexp.MustCompile(`[A-Za-z0-9][A-Za-z0-9._%+\-]*@[A-Za-z0-9][A-Za-z0-9.\-]*\.[A-Za-z]{2,}`)
	rePhone = regexp.MustCompile(`(?:\+?1[\s.\-]?)?\(?([2-9]\d{2})\)?[\s.\-]{1,2}([2-9]\d{2})[\s.\-](\d{4})`)

	contactCue = regexp.MustCompile(`contact|correo|e-?mail|preguntas|questions|informacion|inquir|consultas|tel[eé]fono|phone|llam|call`)
	badEmail   = regexp.MustCompile(`(?i)^(?:no-?reply|donotreply|webmaster|privacy|privacidad|abuse)@|@(?:example|sentry|wixpress)\.`)
)

func (d *doc) cueNear(s int) bool {
	a := d.clauseStart(s, 70)
	return contactCue.MatchString(d.sub(a, s))
}

func (d *doc) findEmail() string {
	idx := reEmail.FindAllStringIndex(string(d.flat), -1)
	flat := string(d.flat)
	first := ""
	for _, m := range idx {
		e := strings.TrimRight(flat[m[0]:m[1]], ".-_")
		if badEmail.MatchString(e) {
			continue
		}
		rs := len([]rune(flat[:m[0]]))
		if d.cueNear(rs) {
			return e
		}
		if first == "" {
			first = e
		}
	}
	return first
}

func (d *doc) findPhone() string {
	flat := string(d.flat)
	first := ""
	for _, m := range rePhone.FindAllStringSubmatchIndex(flat, -1) {
		if m[0] > 0 && flat[m[0]-1] >= '0' && flat[m[0]-1] <= '9' {
			continue
		}
		if m[1] < len(flat) && flat[m[1]] >= '0' && flat[m[1]] <= '9' {
			continue
		}
		if m[0] > 0 && (flat[m[0]-1] == '-' || flat[m[0]-1] == '/' || flat[m[0]-1] == '$' || flat[m[0]-1] == ',') {
			continue // parte de un código, una fecha o una cifra
		}
		p := flat[m[2]:m[3]] + "-" + flat[m[4]:m[5]] + "-" + flat[m[6]:m[7]]
		rs := len([]rune(flat[:m[0]]))
		if d.cueNear(rs) {
			return p
		}
		if first == "" {
			first = p
		}
	}
	return first
}
