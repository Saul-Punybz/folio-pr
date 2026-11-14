package crawler

import (
	"testing"
	"time"
)

func TestParseAmountCents(t *testing.T) {
	tests := []struct {
		in   string
		want int64
	}{
		{"", 0},
		{"sin cifra", 0},
		{"$500", 50_000},
		{"$500 mensuales", 50_000},        // bug viejo: daba $500 millones
		{"hasta $2,500 por mes", 250_000}, // "m" de "mes" no multiplica
		{"$1,234,567.89", 123_456_789},
		{"$5M", 500_000_000},
		{"$5MM", 500_000_000},
		{"$1.5 million", 150_000_000},
		{"$2 millones", 200_000_000},
		{"$1 millón", 100_000_000},
		{"US$3 mil millones", 300_000_000_000},
		{"$2B", 200_000_000_000},
		{"$1.2 billion", 120_000_000_000},
		{"$4 billones", 400_000_000_000},
		{"$50K", 5_000_000},
		{"Up to $250,000 for 2026 programs", 25_000_000}, // prefiere la cifra con $
		{"250000", 25_000_000},
		{"5 Mayo", 500},           // la M de "Mayo" no va pegada como sufijo
		{"$10 membership", 1_000}, // "m" separada no multiplica
	}
	for _, tt := range tests {
		if got := parseAmountCents(tt.in); got != tt.want {
			t.Errorf("parseAmountCents(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestComputeOpportunityStatus(t *testing.T) {
	now := time.Now()
	at := func(d time.Duration) *time.Time { x := now.Add(d); return &x }
	tests := []struct {
		name     string
		deadline *time.Time
		want     string
	}{
		{"sin fecha", nil, "unknown"},
		{"ya pasó", at(-time.Hour), "closed"},
		{"hace un mes", at(-30 * 24 * time.Hour), "closed"},
		{"mañana", at(24 * time.Hour), "closing_soon"},
		{"en 6 días", at(6 * 24 * time.Hour), "closing_soon"},
		{"en 8 días", at(8 * 24 * time.Hour), "open"},
		{"en 3 meses", at(90 * 24 * time.Hour), "open"},
	}
	for _, tt := range tests {
		if got := computeOpportunityStatus(tt.deadline); got != tt.want {
			t.Errorf("%s: computeOpportunityStatus = %q, want %q", tt.name, got, tt.want)
		}
	}
}
