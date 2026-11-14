package crawler

import (
	"testing"

	"github.com/Saul-Punybz/folio/internal/extract"
)

// El parser de montos vive en internal/extract (uno solo en todo el repo).
// Estos son los casos que cubría el parseAmountCents viejo de este paquete.
func TestAmountCentsUnSoloParser(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"sin cifra", 0},
		{"$500", 50_000},
		{"$500 mensuales", 50_000}, // a mano se toma la cifra; la "m" de "mensuales" no multiplica
		{"$1,234,567.89", 123_456_789},
		{"$5M", 500_000_000},
		{"$1.5 million", 150_000_000},
		{"$2 millones", 200_000_000},
		{"$2B", 200_000_000_000},
		{"$50K", 5_000_000},
		{"Up to $250,000 for 2026 programs", 25_000_000},
		{"250,000 dólares", 25_000_000},
	}
	for _, tt := range tests {
		if got := extract.AmountCents(tt.in); got != tt.want {
			t.Errorf("AmountCents(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
