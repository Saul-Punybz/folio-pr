package watch

import (
	"testing"

	"github.com/Saul-Punybz/folio/internal/models"
)

func TestMatchesOrg(t *testing.T) {
	org := models.WatchlistOrg{Name: "Fundación Ángel Ramos", Keywords: []string{"FAR", "x", " "}}
	tests := []struct {
		text string
		want bool
	}{
		{"La FUNDACIÓN ÁNGEL RAMOS dona", true},
		{"Premio de la far en Ponce", true},
		{"x marca el lugar", false}, // palabras clave de 1 letra se ignoran
		{"Nada que ver", false},
	}
	for _, tt := range tests {
		if got := MatchesOrg(tt.text, org); got != tt.want {
			t.Errorf("MatchesOrg(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
