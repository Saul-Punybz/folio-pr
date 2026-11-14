package extract

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

var now0 = time.Date(2026, 11, 14, 12, 0, 0, 0, ast)

type tc struct {
	name string
	text string
	now  time.Time // cero = now0
	real bool      // texto descargado de una fuente real (ver fixtures_test.go)

	cand     bool
	minScore int
	maxScore int // 0 = sin tope
	cat      string
	title    string // subcadena esperada; "" = no se revisa
	ref      string // exacto; "" = debe estar vacío
	deadline string // "2006-01-02 15:04" en AST; "" = debe ser nil
	amount   string // exacto; "" = debe estar vacío
	cents    int64
	email    string
	phone    string
}

var cases = []tc{
	// ---------- Textos reales (descargados el 14 nov 2026) ----------
	{name: "real/CEDD RFP 2026-001, vigente (hoy 20 oct)", text: fxRFPCEDD001, real: true, now: time.Date(2026, 10, 20, 9, 0, 0, 0, ast),
		cand: true, minScore: 70, cat: "rfp", title: "Academia de Testimonios", ref: "RFP 2026-001", deadline: "2026-11-05 16:00",
		email: "ceddpr21@gmail.com", phone: "787-722-0590"}, // la hora sale de la mención "…2026 a las 4:00 p.m."
	{name: "real/CEDD RFP 2026-001, ya vencido (hoy 14 nov)", text: fxRFPCEDD001, real: true,
		cand: false, maxScore: 30, cat: "rfp", ref: "RFP 2026-001", deadline: "2026-11-05 16:00",
		email: "ceddpr21@gmail.com", phone: "787-722-0590"},
	{name: "real/CEDD RFP 2026-005, vigente", text: fxRFPCEDD005, real: true, now: time.Date(2026, 10, 20, 9, 0, 0, 0, ast),
		cand: true, minScore: 60, cat: "rfp", title: "transición a la vida adulta", ref: "RFP 2026-005", deadline: "2026-11-05 23:59"},
	{name: "real/CEDD adenda a ocho RFP (no es convocatoria nueva)", text: fxAdendaCEDD, real: true,
		cand: false, maxScore: 44, cat: "rfp", ref: "RFP 2026-001", email: "ceddpr21@gmail.com", phone: "787-722-0590"},
	{name: "real/RUS subasta formal AEP Culebra", text: fxRUSItem, real: true,
		cand: true, minScore: 55, cat: "rfp", title: "FIRST RESPONDERS SAFE ROOM", ref: "SUBASTA FORMAL 2026-27-53", deadline: "2027-01-14 23:59"},
	{name: "real/RUS RFP-006 de la AAA (PMO)", text: fxRUSRFP, real: true,
		cand: true, minScore: 55, cat: "rfp", ref: "27-RFP-006", deadline: "2027-01-13 23:59"},
	{name: "real/RUS listado completo de subastas", text: fxRUSListado, real: true,
		cand: false, maxScore: 20, cat: "rfp"},
	{name: "real/ASG portada con próximas licitaciones", text: fxASGHome, real: true,
		cand: false, maxScore: 20, cat: "rfp"},
	{name: "real/DDEC avisos públicos y RFP (lista)", text: fxDDECAvisos, real: true,
		cand: false, maxScore: 20, cat: "rfp"},
	{name: "real/Caguas menú de navegación con 'subastas'", text: fxCaguasNav, real: true,
		cand: false, maxScore: 20},
	{name: "real/Vivienda menú con solicitudes y fondos", text: fxViviendaNav, real: true,
		cand: false, maxScore: 25},
	{name: "real/Grants.gov Title X (campos de la API)", text: fxGrantsGov, real: true,
		cand: true, minScore: 70, cat: "grant", title: "Title X Family Planning Services Grants", ref: "Opportunity Number PA-FPH-27-001",
		deadline: "2027-01-11 23:59", amount: "$257,000,000", cents: 25_700_000_000, email: "Eric.West@hhs.gov"},
	{name: "real/Federal Register suplemento SOR (noticia a beneficiarios)", text: fxFedRegSOR, real: true,
		cand: false, maxScore: 40, cat: "grant", amount: "$500,000", cents: 50_000_000},

	// ---------- Sintéticos: avisos completos ----------
	{name: "es/subasta formal municipal completa",
		text: "SUBASTA FORMAL NÚM. 24-015\nMunicipio de Humacao\nLas propuestas se recibirán hasta el 30 de noviembre de 2026 a las 2:00 p.m. en la Oficina de Secretaría.\nReunión pre-subasta: 18 de noviembre de 2026.\nPresupuesto estimado: $1.5 millones.\nInformación: subastas@humacao.pr.gov, (787) 850-1234.",
		cand: true, minScore: 75, cat: "rfp", title: "SUBASTA FORMAL", ref: "SUBASTA FORMAL 24-015", deadline: "2026-11-30 14:00",
		amount: "$1.5 millones", cents: 150_000_000, email: "subastas@humacao.pr.gov", phone: "787-850-1234"},
	{name: "es/subasta informal, fecha con abreviatura",
		text: "Aviso de Subasta Informal 2026-12\nAdquisición de equipo de oficina. Fecha límite: 30 nov. 2026, 10:30 a.m.\nSolicitar pliegos en la Oficina de Compras.",
		cand: true, minScore: 55, cat: "rfp", ref: "Aviso de Subasta Informal 2026-12", deadline: "2026-11-30 10:30"},
	{name: "es/solicitud de propuestas DDEC con mayúsculas y PDF sucio",
		text: "SOLICITUD  DE  PROPUESTAS  NUM.  DDEC-2026-17\n\n  FECHA LIMITE:   15 DE DICIEMBRE\nDE 2026   HASTA LAS 4:00 P.M.\nCUANTIA  MAXIMA:\n$75,000   por propuesta\nLas entidades sin fines de lucro deberán someter su propuesta.",
		cand: true, minScore: 70, cat: "rfp", ref: "SOLICITUD DE PROPUESTAS DDEC-2026-17", deadline: "2026-12-15 16:00",
		amount: "$75,000", cents: 7_500_000},
	{name: "es/guion de fin de renglón en PDF (propues-\\ntas, 24-\\n015)",
		text: "AVISO DE SUBASTA FORMAL NUM. 24-\n015\nSe recibirán propues-\ntas hasta el 5 de enero de 2027. Los licitadores deberán someter sus ofertas.",
		cand: true, minScore: 50, cat: "rfp", ref: "AVISO DE SUBASTA FORMAL 24-015", deadline: "2027-01-05 23:59"},
	{name: "en/NOFO con fecha en inglés y contacto",
		text: "Notice of Funding Opportunity\nNOFO Number: HHS-2026-ACL-AOA-0001\nApplications are due by January 15, 2027 at 5:00 PM ET. Award ceiling: $500,000 per award. Total funding available: $4.5 million.\nQuestions: grants@acl.gov or 202-555-0147.",
		cand: true, minScore: 75, cat: "grant", ref: "NOFO Number HHS-2026-ACL-AOA-0001", deadline: "2027-01-15 17:00",
		amount: "$4.5 million", cents: 450_000_000, email: "grants@acl.gov", phone: "202-555-0147"},
	{name: "es/beca con monto mensual (no confundir con millones)",
		text: "Convocatoria de becas 2026-2027. Fecha límite para solicitar: 15 de enero de 2027. Los estudiantes seleccionados recibirán $500 mensuales durante el año académico. Requisitos de elegibilidad en el formulario.",
		cand: true, minScore: 50, cat: "scholarship", deadline: "2027-01-15 23:59"},
	{name: "es/fondos competitivos, 'hasta $50,000'",
		text: "Fondos competitivos para organizaciones sin fines de lucro. Se aceptan propuestas hasta el 12/15/2026. Cada organización puede recibir hasta $50,000. Contacto: fondos@fundacion.org.",
		cand: true, minScore: 55, cat: "grant", deadline: "2026-12-15 23:59", amount: "$50,000", cents: 5_000_000, email: "fondos@fundacion.org"},
	{name: "es/fecha límite extendida: gana la vigente",
		text: "RFP-2026-003 Servicios de consultoría. Fecha límite original: 10 de noviembre de 2026. La fecha límite fue extendida hasta el 20 de noviembre de 2026. Solicitud de propuestas abierta.",
		cand: true, minScore: 45, cat: "rfp", ref: "RFP-2026-003", deadline: "2026-11-20 23:59"},
	{name: "es/reunión pre-subasta no es la fecha de entrega",
		text: "Subasta Formal Núm. 24-020. Reunión pre-subasta el 18 de noviembre de 2026 a las 10:00 a.m. Las ofertas deberán entregarse en o antes del 2 de diciembre de 2026 a las 3:00 p.m.",
		cand: true, minScore: 55, cat: "rfp", ref: "Subasta Formal 24-020", deadline: "2026-12-02 15:00"},
	{name: "es/hora antes de la fecha",
		text: "Solicitud de propuestas. Las propuestas se aceptan hasta las 2:00 p.m. del 30 de noviembre de 2026. RFQ 26-018.",
		cand: true, minScore: 45, ref: "RFQ 26-018", deadline: "2026-11-30 14:00"},

	// ---------- Montos ----------
	{name: "monto/US$250,000.00 con contexto", text: "Request for proposals. Funding available: US$250,000.00 for community projects. Proposals due 12/01/2026.",
		cand: true, minScore: 45, cat: "rfp", deadline: "2026-12-01 23:59", amount: "US$250,000.00", cents: 25_000_000},
	{name: "monto/$2M pegado", text: "Grant program: awards of up to $2M. Deadline: 2026-12-20.",
		cand: true, minScore: 45, cat: "grant", deadline: "2026-12-20 23:59", amount: "$2M", cents: 200_000_000},
	{name: "monto/$500 mil son 500,000", text: "Subasta Formal 25-001. Presupuesto de $500 mil. Fecha límite: 10 de diciembre de 2026.",
		cand: true, minScore: 55, ref: "Subasta Formal 25-001", deadline: "2026-12-10 23:59", amount: "$500 mil", cents: 50_000_000},
	{name: "monto/fianza y pliego no cuentan, sí el presupuesto", text: "Subasta Formal 25-002. Fianza de licitación: $5,000. Costo del pliego: $150. Presupuesto: $250,000. Fecha límite: 10 de diciembre de 2026.",
		cand: true, minScore: 55, ref: "Subasta Formal 25-002", deadline: "2026-12-10 23:59", amount: "$250,000", cents: 25_000_000},
	{name: "monto/solo tarifas: vacío", text: "Solicitud de propuestas 25-003: fianza de $10,000 y cargo de registro de $300. Fecha límite: 10 de diciembre de 2026.",
		cand: true, minScore: 45, ref: "Solicitud de propuestas 25-003", deadline: "2026-12-10 23:59"},
	{name: "monto/formato ambiguo $1.500.000 queda vacío", text: "Convocatoria de propuestas. Fondos disponibles: $1.500.000 (un millón quinientos mil). Fecha límite: 10 de diciembre de 2026.",
		cand: true, minScore: 45, cat: "proposal", deadline: "2026-12-10 23:59"},
	{name: "monto/dos cifras sin contexto: vacío", text: "Programa de subvenciones. Se otorgaron $12,000 y $30,000 en el pasado. Fecha límite: 10 de diciembre de 2026.",
		cand: true, minScore: 45, cat: "grant", deadline: "2026-12-10 23:59"},

	// ---------- Fechas ----------
	{name: "fecha/sin contexto de límite queda nil", text: "RFP-2026-009. Publicado el 3 de noviembre de 2026 y actualizado el 10 de noviembre de 2026.",
		cand: false, maxScore: 44, ref: "RFP-2026-009"},
	{name: "fecha/año absurdo se descarta", text: "Solicitud de propuestas. Fecha límite: 30 de noviembre de 2099. Beca vigente hasta el 12/31/1999.",
		cand: false, maxScore: 44},
	{name: "fecha/12:00 a.m. es medianoche", text: "Request for proposals. Deadline: December 5, 2026 12:00 a.m.",
		cand: true, minScore: 45, deadline: "2026-12-05 00:00"},
	{name: "fecha/24 horas", text: "Solicitud de propuestas. Fecha límite: 7 de diciembre de 2026 a las 16:30.",
		cand: true, minScore: 45, deadline: "2026-12-07 16:30"},
	{name: "fecha/29 de febrero inexistente", text: "Solicitud de propuestas. Fecha límite: 29 de febrero de 2027.",
		cand: false, maxScore: 44},

	// ---------- Negativos ----------
	{name: "neg/noticia que menciona una subasta pasada", text: "El municipio celebró la subasta formal 24-015 el pasado 3 de octubre de 2026 y adjudicó el contrato por $450,000, informó el alcalde en conferencia de prensa.",
		cand: false, maxScore: 44, amount: "$450,000", cents: 45_000_000, ref: "subasta formal 24-015"},
	{name: "neg/menú de navegación", text: "Inicio\nNoticias\nSubastas\nLicitaciones\nTransparencia\nContacto\nMapa del sitio\nPolítica de privacidad",
		cand: false, maxScore: 20, cat: "rfp"},
	{name: "neg/aviso ya vencido", text: "AVISO DE SUBASTA FORMAL 24-010. Fecha límite: 1 de octubre de 2026. Presupuesto: $90,000.",
		cand: false, maxScore: 30, ref: "AVISO DE SUBASTA FORMAL 24-010", deadline: "2026-10-01 23:59", amount: "$90,000", cents: 9_000_000},
	{name: "neg/$500 mensuales no son millones", text: "Los participantes del taller reciben $500 mensuales. Becas disponibles.",
		cand: false, maxScore: 30},
	{name: "neg/texto sin relación", text: "Receta de arroz con gandules: sofrito, pimiento, sazón. Cocinar 45 minutos. Precio $12.50.",
		cand: false, maxScore: 5, cat: "other"},
	{name: "neg/vacío", text: "", cand: false, cat: "other"},
	{name: "neg/solo espacios y bytes inválidos", text: " \n\t\xff\xfe\x00 ", cand: false, cat: "other"},
}

func TestOpportunity(t *testing.T) {
	if len(cases) < 25 {
		t.Fatalf("se piden al menos 25 casos, hay %d", len(cases))
	}
	for _, c := range cases {
		c := c
		t.Run(c.name, func(t *testing.T) {
			now := c.now
			if now.IsZero() {
				now = now0
			}
			f := Opportunity(c.text, now)
			if f.IsCandidate != c.cand {
				t.Errorf("IsCandidate = %v, quería %v (score %d)", f.IsCandidate, c.cand, f.Score)
			}
			if f.Score < c.minScore || (c.maxScore > 0 && f.Score > c.maxScore) || f.Score < 0 || f.Score > 100 {
				t.Errorf("Score = %d, quería [%d,%d]", f.Score, c.minScore, c.maxScore)
			}
			if c.cat != "" && f.Category != c.cat {
				t.Errorf("Category = %q, quería %q", f.Category, c.cat)
			}
			if c.title != "" && !strings.Contains(f.Title, c.title) {
				t.Errorf("Title = %q, quería que contenga %q", f.Title, c.title)
			}
			if n := utf8.RuneCountInString(f.Title); n > 160 || !utf8.ValidString(f.Title) {
				t.Errorf("Title de %d runas o UTF-8 inválido", n)
			}
			if f.Reference != c.ref {
				t.Errorf("Reference = %q, quería %q", f.Reference, c.ref)
			}
			got := ""
			if f.Deadline != nil {
				got = f.Deadline.In(ast).Format("2006-01-02 15:04")
			}
			if got != c.deadline {
				t.Errorf("Deadline = %q, quería %q", got, c.deadline)
			}
			if f.Amount != c.amount || f.AmountCents != c.cents {
				t.Errorf("Amount = %q/%d, quería %q/%d", f.Amount, f.AmountCents, c.amount, c.cents)
			}
			if f.ContactEmail != c.email {
				t.Errorf("ContactEmail = %q, quería %q", f.ContactEmail, c.email)
			}
			if f.ContactPhone != c.phone {
				t.Errorf("ContactPhone = %q, quería %q", f.ContactPhone, c.phone)
			}
			// La evidencia debe existir para cada campo encontrado, ser un
			// fragmento literal del texto y no pasar de 200 runas.
			for k, v := range f.Evidence {
				if n := utf8.RuneCountInString(v); n == 0 || n > 200 {
					t.Errorf("Evidence[%s] de %d runas", k, n)
				}
				if k != "title" && !strings.Contains(c.text, v) {
					t.Errorf("Evidence[%s] no es literal: %q", k, v)
				}
			}
			for _, k := range []struct {
				key string
				set bool
			}{{"deadline", f.Deadline != nil}, {"amount", f.Amount != ""}, {"reference", f.Reference != ""}, {"title", f.Title != ""}} {
				if k.set != (f.Evidence[k.key] != "") {
					t.Errorf("Evidence[%s] no concuerda con el campo (campo=%v)", k.key, k.set)
				}
			}
		})
	}
}

// Una fecha en cada formato soportado; todas con contexto de límite.
func TestFormatosDeFecha(t *testing.T) {
	formats := []struct{ in, want string }{
		{"Fecha límite: 30 de noviembre de 2026", "2026-11-30 23:59"},
		{"Fecha límite: 30 de noviembre del 2026", "2026-11-30 23:59"},
		{"Fecha límite: 30 nov. 2026", "2026-11-30 23:59"},
		{"Fecha límite: 30 NOV 2026", "2026-11-30 23:59"},
		{"Fecha límite: noviembre 30, 2026", "2026-11-30 23:59"},
		{"Fecha límite: 11/30/2026", "2026-11-30 23:59"},
		{"Fecha límite: 30/11/2026", "2026-11-30 23:59"}, // 30 no puede ser mes
		{"Fecha límite: 2026-11-30", "2026-11-30 23:59"},
		{"Deadline: November 30, 2026", "2026-11-30 23:59"},
		{"Deadline: Nov 30th, 2026", "2026-11-30 23:59"},
		{"Due: 30 November 2026 at 11:59 PM", "2026-11-30 23:59"},
		{"Cierre: 1ro de diciembre de 2026", ""}, // "1ro": formato no soportado, se deja vacío
		{"Cierre: 1 de diciembre de 2026 a las 1:00 p.m.", "2026-12-01 13:00"},
		{"Cierre: martes, 1 de diciembre de 2026", "2026-12-01 23:59"},
		{"Closing date: Sept. 30, 2026", "2026-09-30 23:59"},
		{"Fecha límite: 30 de septiembre de 2026", "2026-09-30 23:59"},
		{"Fecha límite: 30 de noviembre", ""}, // sin año: no se inventa
	}
	for _, c := range formats {
		f := Opportunity("RFP 2026-001. Solicitud de propuestas. "+c.in+".", time.Date(2026, 9, 1, 9, 0, 0, 0, ast))
		got := ""
		if f.Deadline != nil {
			got = f.Deadline.In(ast).Format("2006-01-02 15:04")
		}
		if got != c.want {
			t.Errorf("%q: Deadline = %q, quería %q", c.in, got, c.want)
		}
	}
}

func TestMultiplicadorSoloPegado(t *testing.T) {
	cs := []struct {
		in    string
		cents int64
	}{
		{"Presupuesto: $500 mensuales", 0},
		{"Presupuesto: $500 millones de dólares", 50_000_000_000},
		{"Presupuesto: $2M", 200_000_000},
		{"Presupuesto: $2 M", 0}, // "M" separada de la cifra: no se multiplica ni se acepta
		{"Presupuesto: $1,250,000", 125_000_000},
		{"Presupuesto: $3 billones", 300_000_000_000},
		{"Presupuesto: $80", 0}, // < $100
		{"Presupuesto: $750K", 75_000_000},
	}
	for _, c := range cs {
		f := Opportunity("Subasta formal 25-009. "+c.in+". Fecha límite: 10 de diciembre de 2026.", now0)
		if f.AmountCents != c.cents {
			t.Errorf("%q: AmountCents = %d (%q), quería %d", c.in, f.AmountCents, f.Amount, c.cents)
		}
	}
}

func TestRobustezUnicode(t *testing.T) {
	long := strings.Repeat("Subasta Formal Núm. 24-015 — año ñandú 😀 ", 3000)
	f := Opportunity(long, now0)
	if f.Score < 0 || f.Score > 100 || !utf8.ValidString(f.Title) {
		t.Errorf("resultado inválido: %+v", f)
	}
	// Tildes decompuestas (NFD) y espacios no separables.
	nfd := "SOLICITUD DE PROPUESTAS RFP-2026-044. Fecha límite: 30 de noviembre de 2026."
	g := Opportunity(nfd, now0)
	if g.Reference != "SOLICITUD DE PROPUESTAS RFP-2026-044" || g.Deadline == nil || g.Deadline.Day() != 30 {
		t.Errorf("NFD/NBSP mal tratados: ref=%q deadline=%v", g.Reference, g.Deadline)
	}
}
