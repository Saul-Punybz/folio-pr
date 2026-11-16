package crawler

import (
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Topes de descarga de PDF.
const (
	// maxPDFPerDomainRun: PDF que se bajan por dominio en una corrida.
	maxPDFPerDomainRun = 25
	// maxPDFBytes: tope normal; maxCallPDFBytes si el enlace parece convocatoria.
	maxPDFBytes     int64 = 15 << 20
	maxCallPDFBytes int64 = 40 << 20
	// maxPDFAgeMonths: un archivo subido (wp-content/uploads/AAAA/MM) hace
	// más que esto ya no es una convocatoria abierta.
	maxPDFAgeMonths = 18
)

// notCallRe: lo que claramente no es una convocatoria, aunque el enlace
// mencione una subasta ("Adjudicación de Subasta 5", "Reglamento de Subastas").
// Se compara sin acentos y en minúsculas, sobre el texto del enlace y el
// nombre del archivo.
var notCallRe = regexp.MustCompile(`reglamento|regulation|informe\s+(?:anual|trimestral|mensual|de\s+(?:gesti|progreso|auditor))|annual\s+report|quarterly\s+report|\bactas?\b|minutas?|organigrama|organization(?:al)?\s+chart|solicitud\s+de\s+empleo|employment|\bempleos?\b|comunicado|press\s+release|nota\s+de\s+prensa|adjudicaci|adjudicad|\baward(?:ed)?\b|notificacion\s+de\s+(?:adjud|aprob)|aprobacion|ordenanza|\bresoluci|carta\s+circular|edicto|estorbo|querella|estados?\s+financieros|financial\s+statements?|audit\s+report|nombramiento|cancelacion|resultados?\s+de|tabulaci|\bcv\b|curriculum`)

// weakNotCallRe: lo mismo pero solo si el enlace no parece convocatoria
// ("Subasta 4 ... conforme a la Ley 107" sí se baja).
var weakNotCallRe = regexp.MustCompile(`\bleyes?\b|\blaw\b|\bley-?\s?\d|presupuesto|\bbudget\b|formulario|\bforms?\b|planilla|\bmanual\b|plan\s+estrategico|plan\s+de\s+accion|politica\s+publica|brochure|folleto|boletin|newsletter|calendario|auditoria|certificacion|privacidad|privacy|terminos|terms\s+of|accesibilidad|titulo\s+vi\b`)

// callRe: el enlace o el archivo parecen una convocatoria.
var callRe = regexp.MustCompile(`subasta|licitaci|\brfp\b|\brfq\b|\brfi\b|\bifb\b|\bnofo\b|propuesta|convocatoria|cotizaci|invitaci[o]n|solicitud\s+de\s+(?:propuestas?|cualificaciones|ofertas|cotizaciones)|request\s+for|\bbids?\b|\bgrants?\b|fondos|\bbecas?\b|oportunidad\s+de\s+(?:fondos|financ)|\bsp-?\d{2}|\brfp-?\d`)

// uploadDateRe: /wp-content/uploads/2026/09/archivo.pdf
var uploadDateRe = regexp.MustCompile(`/uploads/((?:19|20)\d{2})/(0[1-9]|1[0-2])/`)

// PDFVerdict es la decisión de bajar o no un PDF antes de pedirlo.
type PDFVerdict struct {
	Download  bool
	LooksCall bool   // parece convocatoria: tope de tamaño mayor
	Reason    string // por qué no (para el registro)
	MaxBytes  int64
}

// ClassifyPDFLink decide, solo con el texto del enlace y la URL, si vale la
// pena bajar el PDF: no baja reglamentos, leyes, informes, presupuestos,
// actas, organigramas, formularios de empleo, comunicados, adjudicaciones ni
// archivos viejos; lo demás sí, con tope de 15 MB (40 MB si parece convocatoria).
func ClassifyPDFLink(anchor, rawURL string, now time.Time) PDFVerdict {
	name := rawURL
	if u, err := url.Parse(rawURL); err == nil {
		name, _ = url.PathUnescape(path.Base(u.Path))
		if m := uploadDateRe.FindStringSubmatch(u.Path); m != nil {
			y, _ := strconv.Atoi(m[1])
			mo, _ := strconv.Atoi(m[2])
			up := time.Date(y, time.Month(mo), 1, 0, 0, 0, 0, time.UTC)
			if now.AddDate(0, -maxPDFAgeMonths, 0).After(up) {
				return PDFVerdict{Reason: "archivo subido hace más de " + strconv.Itoa(maxPDFAgeMonths) + " meses"}
			}
		}
	}
	hint := fold(anchor + " " + strings.NewReplacer("-", " ", "_", " ", ".", " ").Replace(strings.TrimSuffix(strings.ToLower(name), ".pdf")))
	if notCallRe.MatchString(hint) {
		return PDFVerdict{Reason: "no es convocatoria: " + notCallRe.FindString(hint)}
	}
	if callRe.MatchString(hint) {
		return PDFVerdict{Download: true, LooksCall: true, MaxBytes: maxCallPDFBytes}
	}
	if weakNotCallRe.MatchString(hint) {
		return PDFVerdict{Reason: "no es convocatoria: " + weakNotCallRe.FindString(hint)}
	}
	return PDFVerdict{Download: true, MaxBytes: maxPDFBytes}
}
