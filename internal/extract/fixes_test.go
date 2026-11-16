package extract

import (
	"strings"
	"testing"
	"time"
)

// Fragmentos reales (CEDD: cedd.pr.gov, PDF del RFP; subastas.pr.gov; asg.pr.gov),
// descargados el 14-16 nov 2026. Se conserva el texto tal cual salió del PDF/HTML.
const (
	fxCEDD001Frag = `RFP 2026-001: Academia de Testimonios
Emitida: 5 de octubre de 2026. | Fecha límite: 5 de noviembre de 2026.
Solicitud de Propuestas de Servicios Profesionales (RFP): Academia de
Testimonios para personas con discapacidades intelectuales y/o en el desarrollo
(DID) y sus familiares.
La fecha límite para someter las propuestas a través del enlace de formulario
provisto por el CEDD es el 5 de noviembre de 2026 a las 4:00 p.m. Todas las
propuestas deberán ser sometidas electrónicamente mediante dicho enlace,
no más tarde de la fecha y hora límite.
Mes 4
• No más tarde del 30 de septiembre de 2027, completar y documentar el
reclutamiento de al menos ocho (8) personas con DID y seis (6) familiares.
Entregar al CEDD evidencia del reclutamiento mediante los formularios de
ingreso del proyecto, que incluirán los acomodos y apoyos identificados
• No más tarde del 30 de septiembre de 2027, ejecutar el primer taller y su
evaluación.`

	fxCEDD004Frag = `RFP 2026-004: Investigación sobre empleo
Emitida: 5 de octubre de 2026. | Fecha límite: 5 de noviembre de 2026.
Solicitud de Propuestas de Servicios Profesionales (RFP): Llevar a cabo una
investigación sobre las mejores prácticas que optimicen y promuevan un mejor
entorno laboral para las personas con discapacidades intelectuales y/o del
desarrollo (DID) en Puerto Rico. (RFP-2026-004)
La fecha límite para someter las propuestas a través del enlace de formulario
provisto por el CEDD es el 5 de noviembre de 2026 a las 4:00 p.m. Todas las
propuestas deberán ser sometidas electrónicamente mediante dicho enlace.
El contratista deberá completar, no más tarde del 30 de septiembre de 2027, las
actividades, productos y cantidades mínimas correspondientes al Año 1 (Fase I).
A partir del 1 de octubre de 2027, deberá ejecutar las actividades y entregar los
productos correspondientes al Año 2 (Fase II).`

	fxCEDD005Frag = `RFP 2026-005: Investigación sobre la transición a la vida adulta
Emitida: 5 de octubre de 2026. | Fecha límite: 5 de noviembre de 2026.
La fecha límite para someter las propuestas a través del enlace de formulario
provisto por el CEDD es el 5 de noviembre de 2026 a las 4:00 p.m.
Aunque el término inicial del contrato será de doce meses, su ejecución
comprenderá parte de dos años fiscales del Plan Estatal Quinquenal 2027–2031.
El contratista deberá completar, no más tarde del 30 de septiembre de 2027, las
actividades, productos y cantidades mínimas correspondientes al Año 1 (Fase 1).
A partir del 1 de octubre de 2027, deberá ejecutar las actividades y entregar los
productos correspondientes al Año 2 (Fase 2).`

	// Solo el hito (sin el encabezado "Fecha límite"): no debe salir fecha.
	fxCEDDSoloHito = `RFP 2026-004: Investigación sobre empleo
Solicitud de Propuestas de Servicios Profesionales (RFP).
El contratista deberá completar, no más tarde del 30 de septiembre de 2027, las
actividades, productos y cantidades mínimas correspondientes al Año 1 (Fase I).`

	// subastas.pr.gov: entrada de la Autoridad de los Puertos.
	fxRUSPuertos = ` ​Request for Proposals (RFP) for Professional Architecture and Engineering (A/E) Services for the FEMA Public Assistance (PA) Disaster Recovery Project at Mercedita International Airport (PSE) – Ponce under Hurricane Fiona.​
 (RFP PRPA-FEMA-PA-DR-4671-2026-001) 
 Estatus: Abierta
 Agencia: Autoridad de los Puertos (AP)
 Localización: Ponce
 Fecha apertura: miércoles, 2 de diciembre de 2026
 Fecha pre-subasta: miércoles, 21 de octubre de 2026
 Fecha pliegos: lunes, 5 de octubre de 2026`

	// subastas.pr.gov: entrada de la ASG para PRIDCO.
	fxRUSPRIDCO = ` DEMOLICIÓN Y DISPOSICIÓN DE EDIFICIOS INDUSTRIALES DE LA COMPAÑÍA DE FOMENTO INDUSTRIAL T-1469-0-90 UBICADOS EN EL MUNICIPIO DE BARRANQUITAS
DEL GOBIERNO DE PUERTO RICO​
 (27J-06555) 
 Estatus: Abierta
 Agencia: Administración de Servicios Generales (ASG)
 Localización: Gobierno de Puerto Rico
 Fecha apertura: jueves, 3 de diciembre de 2026
 Fecha pre-subasta: lunes, 9 de noviembre de 2026
 Fecha pliegos: miércoles, 4 de noviembre de 2026`

	// asg.pr.gov: "Próximas Licitaciones", una sola fila.
	fxASGProxima = `Próximas Licitaciones
Acto de Apertura
LOTERIA ELECTRONICA DE PUERTO RICO
27J-01601
PARA ADQUISICION DE SISTEMA DE ARCHIVO PARA LA LOTERIA ELECTRONICA DE PUERTO RICO ADSCRITA AL DEPARTAMENTO DE HACIENDA DE PUERTO RICO
9 de noviembre de 2026    1:00p.m.
Acceder`

	// subastas.pr.gov: enmienda a una solicitud de propuestas (2024).
	fxRUSEnmienda = ` ​​Enmienda 1 a la Solicitud de Propuestas OCE-RFP-24-03 Mejoras de la seguridad física de la Oficina del Contralor Electoral de Puerto Rico
 (OCE-RFP-24-03-Enmienda1) 
 Estatus: Abierta
 Agencia: Oficina del Contralor Electoral (OCE)
 Localización: San Juan
 Fecha apertura: viernes, 6 de diciembre de 2024
 Fecha pre-subasta: lunes, 2 de diciembre de 2024`
)

func dl(f Found) string {
	if f.Deadline == nil {
		return ""
	}
	return f.Deadline.In(ast).Format("2006-01-02 15:04")
}

// 1. El hito de ejecución no es la fecha límite.
func TestHitoNoEsFechaLimite(t *testing.T) {
	vencido := time.Date(2026, 11, 16, 9, 0, 0, 0, ast)
	vigente := time.Date(2026, 10, 20, 9, 0, 0, 0, ast)
	for name, txt := range map[string]string{"001": fxCEDD001Frag, "004": fxCEDD004Frag, "005": fxCEDD005Frag} {
		f := Opportunity(txt, vencido)
		if got := dl(f); got != "2026-11-05 16:00" {
			t.Errorf("RFP %s (16 nov): Deadline = %q, quería la entrega 2026-11-05 16:00", name, got)
		}
		if f.IsCandidate || f.Score > 30 {
			t.Errorf("RFP %s (16 nov): vencido debe quedar no candidato y Score<=30, salió cand=%v score=%d", name, f.IsCandidate, f.Score)
		}
		if strings.Contains(f.Evidence["deadline"], "septiembre de 2027") {
			t.Errorf("RFP %s: la evidencia de la fecha cita el hito: %q", name, f.Evidence["deadline"])
		}
		g := Opportunity(txt, vigente)
		if dl(g) != "2026-11-05 16:00" || !g.IsCandidate {
			t.Errorf("RFP %s (20 oct): Deadline=%q cand=%v, quería 2026-11-05 16:00 y candidato", name, dl(g), g.IsCandidate)
		}
	}
	if f := Opportunity(fxCEDDSoloHito, vencido); f.Deadline != nil {
		t.Errorf("solo el hito: no debe haber fecha, salió %q", dl(f))
	}
	// Vigencia y período de desempeño tampoco son entrega.
	for _, in := range []string{
		"Solicitud de propuestas RFP 26-001. La vigencia del contrato será hasta el 30 de junio de 2027.",
		"Request for proposals. Period of performance: no later than June 30, 2027. Deliverables due March 1, 2027.",
	} {
		if f := Opportunity(in, vencidoAntes()); f.Deadline != nil {
			t.Errorf("%q: no debe haber fecha límite, salió %q", in, dl(f))
		}
	}
	// Entrega vencida + fecha futura sin contexto de entrega: gana la de entrega.
	f := Opportunity("Solicitud de propuestas RFP 26-002. Las propuestas deberán ser entregadas hasta el 5 de noviembre de 2026. El proyecto estará disponible hasta el 30 de marzo de 2027.", vencido)
	if dl(f) != "2026-11-05 23:59" || f.IsCandidate {
		t.Errorf("entrega vencida vs fecha futura sin contexto: Deadline=%q cand=%v", dl(f), f.IsCandidate)
	}
}

func vencidoAntes() time.Time { return time.Date(2026, 11, 16, 9, 0, 0, 0, ast) }

// 2. Referencias completas y normalizables.
func TestReferenciaCompleta(t *testing.T) {
	f := Opportunity(fxRUSPuertos, now0)
	if f.Reference != "RFP PRPA-FEMA-PA-DR-4671-2026-001" {
		t.Errorf("Reference = %q", f.Reference)
	}
	if dl(f) != "2026-12-02 23:59" || !f.IsCandidate {
		t.Errorf("Puertos: Deadline=%q cand=%v score=%d", dl(f), f.IsCandidate, f.Score)
	}
	// Espacio sobrante de PDF antes del guion, y la forma corta no duplica.
	g := Opportunity("Solicitud de propuestas. RFP PRPA-FEMA-PA-DR-4671 -2026-001. Fecha límite: 2 de diciembre de 2026.", now0)
	if g.Reference != "RFP PRPA-FEMA-PA-DR-4671-2026-001" {
		t.Errorf("con espacio de PDF: Reference = %q", g.Reference)
	}
	h := Opportunity("RFP PRPA-FEMA-PA-DR-4671-2026-001 (también PRPA-FEMA-PA-DR-4671). RFP 2026-007 - Destrezas financieras. Fecha límite: 2 de diciembre de 2026.", now0)
	if h.Reference != "RFP PRPA-FEMA-PA-DR-4671-2026-001" {
		t.Errorf("forma corta y larga: Reference = %q", h.Reference)
	}
	if r := Opportunity("RFP 2026-007 - Destrezas financieras. Fecha límite: 2 de diciembre de 2026.", now0).Reference; r != "RFP 2026-007" {
		t.Errorf("el guion de puntuación no es del id: %q", r)
	}
}

func TestNormalizeReference(t *testing.T) {
	iguales := [][]string{
		{"RFP PRPA-FEMA-PA-DR-4671-2026-001", "rfp prpa-fema-pa-dr-4671 -2026-001", "Solicitud de Propuestas Núm. PRPA-FEMA-PA-DR-4671 - 2026-001", "RFP-PRPA-FEMA-PA-DR-4671-2026-001", "Request for Proposals No. PRPA-FEMA-PA-DR-4671-2026-001"},
		{"SUBASTA FORMAL 2026-27-53", "Subasta Formal Núm. 2026-27-53", "Aviso de Subasta 2026-27-53", "subasta formal  núm.2026-27-53", "SUBASTA #2026-27-53"},
		{"RFP 2026-001", "Solicitud de Propuestas 2026-001", "rfp-2026-001", "RFP No. 2026 - 001"},
		{"27J-06555", "27j-06555"},
		{"Opportunity Number PA-FPH-27-001", "NOFO PA-FPH-27-001", "Funding Opportunity Number: PA-FPH-27-001"},
	}
	for _, g := range iguales {
		want := NormalizeReference(g[0])
		if want == "" {
			t.Fatalf("NormalizeReference(%q) vacío", g[0])
		}
		for _, x := range g[1:] {
			if got := NormalizeReference(x); got != want {
				t.Errorf("NormalizeReference(%q) = %q, quería %q", x, got, want)
			}
		}
	}
	if NormalizeReference("RFP 2026-001") == NormalizeReference("RFP 2026-004") || NormalizeReference("") != "" {
		t.Errorf("referencias distintas deben seguir distintas y vacío = vacío")
	}
}

// 3. Montos del fondo, no cargos.
func TestMontosDelFondo(t *testing.T) {
	cs := []struct{ name, in, amount string }{
		{"penalidad sola no es el monto", "Solicitud de propuestas RFP 26-010. Por incumplimiento se impondrá una penalidad de $1,000 por cada día de atraso. Fecha límite: 10 de diciembre de 2026.", ""},
		{"multa y póliza", "Subasta Formal 26-011. El licitador presentará una póliza de seguro de responsabilidad pública de $1,000,000. La multa será de $5,000. Fecha límite: 10 de diciembre de 2026.", ""},
		{"depósito y costo del pliego", "Subasta Formal 26-012. Depósito de $2,500. Costo del pliego: $200. Fecha límite: 10 de diciembre de 2026.", ""},
		{"penalidad antes, presupuesto después", "Subasta Formal 26-013. Se impondrá una multa de $1,000 por incumplimiento; presupuesto asignado de $350,000. Fecha límite: 10 de diciembre de 2026.", "$350,000"},
		{"no excederá de", "Solicitud de propuestas RFP 26-014. La subvención por proyecto no excederá de $75,000. Fecha límite: 10 de diciembre de 2026.", "$75,000"},
		{"hasta $X", "Convocatoria de propuestas. Se otorgarán hasta $120,000 a cada entidad. Fecha límite: 10 de diciembre de 2026.", "$120,000"},
		{"fondos disponibles", "Solicitud de propuestas RFP 26-015. Fondos disponibles: $2.5 millones. Fecha límite: 10 de diciembre de 2026.", "$2.5 millones"},
		{"award ceiling gana a total", "Funding opportunity. Estimated Total Program Funding: $9,000,000. Award Ceiling: $450,000. Award Floor: $50,000. Closing date: December 10, 2026.", "$450,000"},
		{"solo floor y total: gana el total", "Funding opportunity. Estimated Total Program Funding: $9,000,000. Award Floor: $50,000. Closing date: December 10, 2026.", "$9,000,000"},
		{"ambiguo: dos cifras sin rótulo", "Solicitud de propuestas RFP 26-016. Hay $40,000 y $60,000. Fecha límite: 10 de diciembre de 2026.", ""},
	}
	for _, c := range cs {
		if got := Opportunity(c.in, now0).Amount; got != c.amount {
			t.Errorf("%s: Amount = %q, quería %q", c.name, got, c.amount)
		}
	}
	// Las convocatorias reales del CEDD: $2,506,930 es acumulado federal y no el
	// presupuesto del proyecto ("no representa el presupuesto disponible").
	if a := Opportunity(fxRFPCEDD001, now0).Amount; a != "" {
		t.Errorf("CEDD 001: Amount = %q, quería vacío", a)
	}
}

// 4. Una enmienda no es candidata pero entrega la nueva fecha.
func TestEnmiendaEntregaNuevaFecha(t *testing.T) {
	ahora := time.Date(2024, 11, 10, 9, 0, 0, 0, ast)
	f := Opportunity(fxRUSEnmienda, ahora)
	if f.IsCandidate || f.Evidence["amendment"] != "1" {
		t.Errorf("enmienda real: cand=%v amendment=%q", f.IsCandidate, f.Evidence["amendment"])
	}
	if dl(f) != "2024-12-06 23:59" {
		t.Errorf("enmienda real: Deadline = %q", dl(f))
	}
	if NormalizeReference(f.Reference) != "RFP OCE-RFP-24-03" {
		t.Errorf("enmienda real: Reference = %q (normalizada %q)", f.Reference, NormalizeReference(f.Reference))
	}
	g := Opportunity("ADENDA NÚM. 2 a la Subasta Formal 26-020. Se extiende la fecha límite para la entrega de ofertas hasta el 18 de diciembre de 2026 a las 3:00 p.m. Fecha límite original: 2 de diciembre de 2026.", now0)
	if g.IsCandidate || g.Evidence["amendment"] != "1" || dl(g) != "2026-12-18 15:00" || NormalizeReference(g.Reference) != "SUBASTA 26-020" {
		t.Errorf("adenda sintética: cand=%v amend=%q dl=%q ref=%q", g.IsCandidate, g.Evidence["amendment"], dl(g), g.Reference)
	}
	// Una adenda sin fecha (real, CEDD) no marca amendment.
	if f := Opportunity(fxAdendaCEDD, now0); f.Evidence["amendment"] != "" || f.IsCandidate {
		t.Errorf("adenda CEDD sin fecha: cand=%v amend=%q", f.IsCandidate, f.Evidence["amendment"])
	}
	// Un RFP completo que menciona "adenda" en el cuerpo sigue siendo convocatoria.
	cuerpo := fxCEDD001Frag + "\nCualquier cambio a este RFP se hará mediante adenda publicada en la página del CEDD. Contacto: ceddpr21@gmail.com, 787-722-0590."
	if f := Opportunity(cuerpo, time.Date(2026, 10, 20, 9, 0, 0, 0, ast)); !f.IsCandidate || f.Evidence["amendment"] != "" {
		t.Errorf("RFP que solo menciona adenda: cand=%v amend=%q score=%d", f.IsCandidate, f.Evidence["amendment"], f.Score)
	}
	// Un aviso normal no lleva la marca.
	if f := Opportunity(fxRUSPuertos, now0); f.Evidence["amendment"] != "" {
		t.Errorf("aviso normal marcado como enmienda")
	}
}

// 5. Fechas sin año y ordinales.
func TestFechasSinAnio(t *testing.T) {
	cs := []struct {
		in   string
		now  time.Time
		want string
	}{
		{"Solicitud de propuestas. Las propuestas se recibirán hasta el 30 de noviembre.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), "2026-11-30 23:59"},
		{"Fecha límite para someter propuestas: 1ro de diciembre.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), "2026-12-01 23:59"},
		{"Fecha límite para someter propuestas: 1ero de diciembre a las 2:00 p.m.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), "2026-12-01 14:00"},
		{"Fecha límite para someter propuestas: 1º de diciembre.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), "2026-12-01 23:59"},
		// ya pasó hace menos de 60 días: sigue siendo este año (aviso vencido)
		{"Fecha límite para someter propuestas: 30 de septiembre.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), "2026-09-30 23:59"},
		// ya pasó hace más de 60 días: el año siguiente
		{"Fecha límite para someter propuestas: 15 de enero.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), "2027-01-15 23:59"},
		{"Fecha límite para someter propuestas: 15 de enero.", time.Date(2026, 3, 20, 9, 0, 0, 0, ast), "2027-01-15 23:59"}, // 64 días atrás
		{"Fecha límite para someter propuestas: 15 de enero.", time.Date(2026, 3, 10, 9, 0, 0, 0, ast), "2026-01-15 23:59"},
		{"Fecha límite para someter propuestas: 15 de enero.", time.Date(2026, 3, 30, 9, 0, 0, 0, ast), "2027-01-15 23:59"}, // 74 días atrás
		{"Request for proposals. Proposals are due December 4.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), "2026-12-04 23:59"},
		// sin contexto de entrega no se inventa
		{"Solicitud de propuestas. Publicada el 3 de noviembre. Reunión informativa el 18 de noviembre.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), ""},
		{"Solicitud de propuestas. El contratista deberá completar el trabajo no más tarde del 30 de noviembre.", time.Date(2026, 11, 16, 9, 0, 0, 0, ast), ""},
	}
	for _, c := range cs {
		if got := dl(Opportunity(c.in, c.now)); got != c.want {
			t.Errorf("%q (hoy %s): Deadline = %q, quería %q", c.in, c.now.Format("2006-01-02"), got, c.want)
		}
	}
}

// Avisos reales de subastas.pr.gov y asg.pr.gov: la fecha de apertura es la de entrega.
func TestAvisosRealesSubastas(t *testing.T) {
	f := Opportunity(fxRUSPRIDCO, now0)
	if f.Reference != "27J-06555" || dl(f) != "2026-12-03 23:59" || !f.IsCandidate {
		t.Errorf("PRIDCO: ref=%q dl=%q cand=%v score=%d", f.Reference, dl(f), f.IsCandidate, f.Score)
	}
	a := Opportunity(fxASGProxima, now0)
	// En asg.pr.gov el rótulo ("Acto de Apertura" / "Reunión Pre Subasta") va
	// ANTES de una descripción larga y la fecha después: no se adivina cuál es
	// (limitación conocida); la referencia sí sale y la fila no es candidata.
	if a.Reference != "27J-01601" || a.Deadline != nil || a.IsCandidate {
		t.Errorf("ASG próxima licitación: ref=%q dl=%q cand=%v", a.Reference, dl(a), a.IsCandidate)
	}
}
