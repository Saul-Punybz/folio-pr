package models

import "testing"

func TestFundType(t *testing.T) {
	cases := []struct{ name, cat, ref, title, want string }{
		{"beca", "scholarship", "", "Beca de estudio", FundGrant},
		{"fondos", "funding", "", "Fondo comunitario", FundGrant},
		{"fellowship", "fellowship", "", "Programa", FundGrant},
		{"grant gana al texto", "grant", "SUBASTA 1", "x", FundGrant},
		{"rfp con subasta en la referencia", "rfp", "SUBASTA FORMAL 2026-27-53", "Construction", FundSubasta},
		{"licitación con tilde", "other", "", "Licitación pública 22", FundSubasta},
		{"IFB", "proposal", "IFB-2027-4", "Servicios", FundSubasta},
		{"ifb no es parte de otra palabra", "other", "", "Califbe servicios", FundOtros},
		{"rfp por texto", "other", "RFP 2027-9", "Servicios", FundRFP},
		{"solicitud de propuestas", "contract", "", "Solicitud de Propuestas para limpieza", FundRFP},
		{"RFQ", "other", "RFQ-77", "", FundRFP},
		{"contrato sin pista", "contract", "", "Mantenimiento", FundSubasta},
		{"rfp sin pista", "rfp", "", "Mantenimiento", FundRFP},
		{"proposal sin pista", "proposal", "", "Mantenimiento", FundRFP},
		{"otros", "other", "", "Aviso", FundOtros},
	}
	for _, c := range cases {
		if got := FundType(c.cat, c.ref, c.title); got != c.want {
			t.Errorf("%s: FundType(%q,%q,%q) = %q, quería %q", c.name, c.cat, c.ref, c.title, got, c.want)
		}
	}
}
