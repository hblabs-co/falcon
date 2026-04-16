package contractorde

import (
	"testing"
	"time"

	"hblabs.co/falcon/modules/platformkit"
)

// fixedNow pins platformkit.NowFunc to 2026-04-16 so month-only inputs
// like "Mai" or "Juni" resolve deterministically.
func fixedNow(t *testing.T) {
	t.Helper()
	prev := platformkit.NowFunc
	platformkit.NowFunc = func() time.Time {
		return time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	}
	t.Cleanup(func() { platformkit.NowFunc = prev })
}

func TestParseStartDate(t *testing.T) {
	fixedNow(t)

	tests := []struct {
		name  string
		input string
		want  string
	}{
		// --- empty / vague → drop ---
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"nach Absprache (vague)", "nach Absprache", ""},

		// --- immediate → canonical ---
		{"asap lowercase", "asap", "ab sofort"},
		{"Ab sofort capitalized", "Ab sofort", "ab sofort"},

		// --- already canonical ISO ---
		{"ISO 2026-07-01", "2026-07-01", "2026-07-01"},
		{"ISO 2025-05-01", "2025-05-01", "2025-05-01"},

		// --- DD.MM.YYYY (primary feed format) ---
		{"dot DD.MM.YYYY", "01.05.2026", "2026-05-01"},
		{"dot lenient", "1.5.2026", "2026-05-01"},
		{"dot calendar-invalid clamp", "31.04.2026", "2026-04-30"},

		// --- Month alone (year inferred) ---
		{"April alone", "April", "2026-04-01"},
		{"Mai alone", "Mai", "2026-05-01"},
		{"Juni alone", "Juni", "2026-06-01"},

		// --- Month + year text ---
		{"April 2026", "April 2026", "2026-04-01"},
		{"Mai 2026", "Mai 2026", "2026-05-01"},

		// --- Anfang / Mitte / Ende ---
		{"Anfang April", "Anfang April", "2026-04-01"},
		{"Mitte April", "Mitte April", "2026-04-15"},
		{"Ende August", "Ende August", "2026-08-31"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseStartDate(tt.input); got != tt.want {
				t.Errorf("parseStartDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseProjectID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Projekt-ID 30290", "30290"},
		{"Projekt-ID  12345", "12345"},
		{"  Projekt-ID 999  ", "999"},
		{"", ""},
		{"no prefix", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseProjectID(tt.input); got != tt.want {
				t.Errorf("parseProjectID(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
