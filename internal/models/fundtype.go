package models

import (
	"regexp"
	"strings"
)

// Tipos de fondo para el filtro de Guardados.
const (
	FundGrant   = "grant"
	FundSubasta = "subasta"
	FundRFP     = "rfp"
	FundOtros   = "otros"
)

var (
	reSubasta = regexp.MustCompile(`\b(subasta|licitacion|ifb)\b`)
	reRFPText = regexp.MustCompile(`\b(rfp|rfq|solicitud de propuestas)\b`)
	accents   = strings.NewReplacer("á", "a", "é", "e", "í", "i", "ó", "o", "ú", "u", "ü", "u", "ñ", "n")
)

// FundType es la ÚNICA regla que decide si un fondo es Grant, Subasta, RFP u
// Otros (la pantalla Fondos solo lee el resultado). En orden:
//
//  1. category grant, funding, fellowship o scholarship → "grant".
//  2. La referencia o el título dicen «subasta», «licitación» o «IFB» → "subasta".
//  3. Dicen «RFP», «RFQ» o «solicitud de propuestas» → "rfp".
//  4. Sin pista en el texto: category contract → "subasta"; rfp o proposal → "rfp".
//  5. Lo demás → "otros".
func FundType(category, reference, title string) string {
	switch category {
	case "grant", "funding", "fellowship", "scholarship":
		return FundGrant
	}
	text := accents.Replace(strings.ToLower(reference + " " + title))
	switch {
	case reSubasta.MatchString(text):
		return FundSubasta
	case reRFPText.MatchString(text):
		return FundRFP
	}
	switch category {
	case "contract":
		return FundSubasta
	case "rfp", "proposal":
		return FundRFP
	}
	return FundOtros
}
