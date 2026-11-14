package models

import (
	"testing"
	"time"
)

// Antes vivía en internal/crawler como computeOpportunityStatus; ahora es la
// única regla de estado (crawler, PUT y, en SQL, RefreshStatuses).
func TestOpportunityStatus(t *testing.T) {
	now := time.Date(2026, 11, 14, 9, 0, 0, 0, prAST)
	day := func(d int) *time.Time { x := time.Date(2026, 11, 14+d, 23, 59, 0, 0, prAST); return &x }
	dbDate := func(d int) *time.Time { x := time.Date(2026, 11, 14+d, 0, 0, 0, 0, time.UTC); return &x }
	tests := []struct {
		name     string
		deadline *time.Time
		want     string
	}{
		{"sin fecha", nil, "unknown"},
		{"ayer", day(-1), "closed"},
		{"hace un mes", day(-30), "closed"},
		{"hoy (aún no vence)", day(0), "closing_soon"},
		{"hoy, leída de la base (DATE en UTC)", dbDate(0), "closing_soon"},
		{"en 7 días", day(7), "closing_soon"},
		{"en 8 días", day(8), "open"},
		{"en 3 meses", day(90), "open"},
	}
	for _, tt := range tests {
		if got := OpportunityStatus(tt.deadline, now); got != tt.want {
			t.Errorf("%s: OpportunityStatus = %q, want %q", tt.name, got, tt.want)
		}
	}
}
