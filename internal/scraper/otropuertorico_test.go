package scraper

import "testing"

func TestIsOtherPuertoRico(t *testing.T) {
	tests := []struct {
		text string
		want bool
	}{
		{"Inauguran escuela en Puerto Rico, Misiones", true},
		{"Intendente de Puerto Rico (Misiones) anunció obras", true},
		{"Lluvias en la provincia de Misiones afectan la ruta", true},
		{"Club Atlético Puerto Rico ganó el clásico", true},
		{"Puerto Rico FC visita al líder", true},
		{"El gobernador de Puerto Rico firmó la ley", false},
		{"Misiones espaciales de la NASA y Puerto Rico", false},
		{"Congreso discute fondos de FEMA para Puerto Rico", false},
	}
	for _, tc := range tests {
		if got := isOtherPuertoRico(tc.text); got != tc.want {
			t.Errorf("%q = %v, want %v", tc.text, got, tc.want)
		}
	}
}
