package grants

import (
	"regexp"
	"strings"
)

// Level es la relevancia de una oportunidad para una nonprofit de Puerto Rico.
type Level int

const (
	Discarded Level = iota // no se inserta
	Low                    // se inserta con confianza baja, solo si sobra lugar
	High
)

func (l Level) String() string {
	switch l {
	case High:
		return "alta"
	case Low:
		return "baja"
	}
	return "descartada"
}

// Relevance es el resultado de Assess (reglas, sin IA).
type Relevance struct {
	Level      Level
	Confidence int    // 0..100
	Reason     string // por qué, en español
	Evidence   string // fragmento literal de elegibilidad (o de la descripción)
}

var (
	reNonprofit = regexp.MustCompile(`(?i)non-?profit|not-for-profit|501\s*\(c\)|nongovernmental|non-governmental|community[- ]based|faith[- ]based|charitable|civil society|any (public or private )?(entity|organization)|all (entities|organizations)|private organization`)
	rePR        = regexp.MustCompile(`(?i)puerto rico|commonwealth of puerto`)
	reTerr      = regexp.MustCompile(`(?i)\bu\.?s\.? territor|territories|territorial|insular area`)
	// Quién se menciona como único elegible.
	reOnlyOthers = regexp.MustCompile(`(?i)tribe|tribal|native american|alaska native|native hawaiian|indian|universit|college|institutions? of higher|state government|state agenc|units? of local government|local government|small business|for-profit|commercial|individual|sole source|single source|only entity|only eligible|is the only|amtrak|school district|electric cooperative|federal agenc`)
	// Restricción geográfica a estados concretos.
	reStateRestrict = regexp.MustCompile(`(?i)(located|residing|resident|reside|operating|based|headquartered|serving|service area|within|only)\W+(in|of|to)?\W*(the )?(state|states|counties|county|city) of|(Alabama|Alaska|Arizona|Arkansas|California|Colorado|Connecticut|Delaware|Florida|Georgia|Hawaii|Idaho|Illinois|Indiana|Iowa|Kansas|Kentucky|Louisiana|Maine|Maryland|Massachusetts|Michigan|Minnesota|Mississippi|Missouri|Montana|Nebraska|Nevada|New Hampshire|New Jersey|New Mexico|New York|North Carolina|North Dakota|Ohio|Oklahoma|Oregon|Pennsylvania|Rhode Island|South Carolina|South Dakota|Tennessee|Texas|Utah|Vermont|Virginia|Washington|West Virginia|Wisconsin|Wyoming)\b[^.]{0,60}(only|residents|located|counties|state)`)
	reAbroad        = regexp.MustCompile(`(?i)embassy|\bU\.?S\.? mission\b|public diplomacy|consulate|PEPFAR|overseas|Department of State`)
	reRegion        = regexp.MustCompile(`(?i)great lakes|chesapeake|columbia river|mississippi river|appalachia|gulf of (mexico|america)|puget sound|everglades`)
	reIndiv         = regexp.MustCompile(`(?i)fellowship|scholarship|internship|\bFellows?\b`)
	reResearch      = regexp.MustCompile(`(?i)\bNIH\b|national institutes? of health|national science foundation|\bNSF\b|office of naval research|\bDARPA\b|\bSBIR\b|\bSTTR\b|clinical trial|\bR01\b|\bR21\b|\bresearch\b|\bDoW\b|CDMRP|\bstudy\b|\bstudies\b`)
)

// tipos de applicant de Grants.gov
func hasType(d *Detail, ids ...string) bool {
	for _, t := range d.ApplicantTypes {
		for _, id := range ids {
			if t == id {
				return true
			}
		}
	}
	return false
}

// Assess calcula la relevancia SIN IA. Reglas:
//   - Sin tipo 12/13/99/25 (nonprofit, «otros», sin restricción): descartada.
//   - Tipo 12 o 13 o 99 = elegible; solo 25 («otros») exige que el texto de
//     elegibilidad nombre a nonprofits u organizaciones; si no, descartada.
//   - El texto libre manda sobre los tipos cuando nombra a un único grupo
//     ajeno (tribus, universidades, gobiernos, empresas...) sin nombrar
//     nonprofits: descartada.
//   - Restringida a estados que no incluyen a Puerto Rico: descartada.
//   - Alta: elegible y no restringe por estado (90 si el texto nombra a Puerto
//     Rico, 80 si nombra territorios, 70 si no restringe). Baja (35-45):
//     investigación (NIH/NSF), o elegibilidad ambigua.
func Assess(d *Detail) Relevance {
	el := d.EligibilityText
	all := el + " " + d.Description
	npText := reNonprofit.MatchString(el)
	ev := func(re *regexp.Regexp, s string) string {
		loc := re.FindStringIndex(s)
		if loc == nil {
			return ""
		}
		a, b := loc[0]-60, loc[1]+80
		if a < 0 {
			a = 0
		}
		if b > len(s) {
			b = len(s)
		}
		for a > 0 && a < len(s) && s[a]&0xC0 == 0x80 {
			a--
		}
		for b < len(s) && s[b]&0xC0 == 0x80 {
			b++
		}
		return strings.TrimSpace(s[a:b])
	}
	// Evidencia por defecto: el texto de elegibilidad, o las etiquetas de tipo.
	evidence := Truncate(el, 300)
	if evidence == "" {
		evidence = "Applicant types: " + Truncate(strings.Join(d.ApplicantLabels, "; "), 300)
	}

	if !hasType(d, "12", "13", "99", "25") {
		return Relevance{Discarded, 0, "la elegibilidad no incluye organizaciones sin fines de lucro", evidence}
	}
	npType := hasType(d, "12", "13", "99")
	if !npType && !npText {
		return Relevance{Discarded, 0, "solo «otros» y el texto no nombra organizaciones sin fines de lucro", evidence}
	}
	if !npText && el != "" && reOnlyOthers.MatchString(el) && len(d.ApplicantTypes) <= 6 {
		return Relevance{Discarded, 0, "el texto de elegibilidad nombra otro grupo (tribus, gobiernos, universidades, empresas...)", evidence}
	}
	if !rePR.MatchString(all) && reAbroad.MatchString(d.Agency+" "+d.Title+" "+Truncate(d.Description, 300)) {
		return Relevance{Discarded, 0, "programa para el extranjero (embajadas, Departamento de Estado)", evidence}
	}
	if !rePR.MatchString(all) && reRegion.MatchString(d.Title+" "+Truncate(d.Description, 400)) {
		return Relevance{Discarded, 0, "restringida a una región de EE. UU. que no incluye a Puerto Rico", evidence}
	}
	if reIndiv.MatchString(d.Title) {
		return Relevance{Discarded, 0, "beca o fellowship para individuos", evidence}
	}
	prInEl := rePR.MatchString(el)
	prAny := prInEl || rePR.MatchString(all)
	terr := reTerr.MatchString(el) || reTerr.MatchString(d.Description)
	if !prAny && !terr && reStateRestrict.MatchString(el) && !strings.Contains(strings.ToLower(el), "any state") {
		return Relevance{Discarded, 0, "restringida a estados que no incluyen a Puerto Rico", ev(reStateRestrict, el)}
	}
	conf, why := 70, "elegible para nonprofits y no restringe por estado"
	switch {
	case prInEl:
		conf, why, evidence = 90, "la elegibilidad nombra a Puerto Rico", firstNonEmpty(ev(rePR, el), evidence)
	case prAny:
		conf, why = 85, "la descripción nombra a Puerto Rico"
	case terr:
		conf, why = 80, "nombra a los territorios de EE. UU."
	}
	if !npType {
		conf -= 10
		why += " (elegibilidad «otros», texto menciona nonprofits)"
	}
	if npText && evidence == Truncate(el, 300) {
		evidence = firstNonEmpty(ev(reNonprofit, el), evidence)
	}
	if reResearch.MatchString(d.Agency+" "+d.Number+" "+d.Title) && !prAny {
		return Relevance{Low, 40, "programa de investigación (NIH/NSF/similar), poco probable para una nonprofit comunitaria", evidence}
	}
	if d.Forecast {
		conf -= 10
		why += " (pronóstico, aún sin convocatoria)"
	}
	lvl := High
	if conf < 60 {
		lvl = Low
	}
	return Relevance{lvl, conf, why, evidence}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}
