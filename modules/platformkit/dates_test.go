package platformkit

import (
	"testing"
	"time"
)

func TestIsImmediateStart(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		// hits
		{"asap", true},
		{"ASAP", true},
		{"Asap", true},
		{"  asap  ", true},
		{"immediate", true},
		{"immediately", true},
		{"sofort", true},
		{"ab sofort", true},
		{"Ab Sofort", true},
		{"AB SOFORT", true},
		{"nächstmöglich", true},
		{"NÄCHSTMÖGLICH", true},
		{"nächstmöglichst", true},
		{"naechstmoeglich", true},
		{"zum nächstmöglichen Zeitpunkt", true},

		// misses
		{"", false},
		{"   ", false},
		{"ab April", false},
		{"01.05.2026", false},
		{"soon", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := IsImmediateStart(tt.input); got != tt.want {
				t.Errorf("IsImmediateStart(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestLastDayOfMonth(t *testing.T) {
	tests := []struct {
		year, month, wantDay int
	}{
		{2026, 1, 31},  // Jan
		{2026, 4, 30},  // Apr
		{2025, 2, 28},  // Feb non-leap
		{2024, 2, 29},  // Feb leap
		{2026, 11, 30}, // Nov
		{2026, 12, 31}, // Dec
	}
	for _, tt := range tests {
		got := LastDayOfMonth(time.Date(tt.year, time.Month(tt.month), 1, 0, 0, 0, 0, time.UTC))
		if got.Day() != tt.wantDay {
			t.Errorf("LastDayOfMonth(%d-%02d) = %d, want %d", tt.year, tt.month, got.Day(), tt.wantDay)
		}
	}
}

func TestParseEuropeanDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		sep   string
		want  string
		ok    bool
	}{
		// dot-separated ---
		{"strict dot DD.MM.YYYY", "01.03.2026", ".", "2026-03-01", true},
		{"lenient dot D.M.YYYY", "1.7.2026", ".", "2026-07-01", true},
		{"lenient dot D.MM.YYYY", "1.07.2026", ".", "2026-07-01", true},
		{"lenient dot DD.M.YYYY", "01.7.2026", ".", "2026-07-01", true},

		// slash-separated ---
		{"strict slash DD/MM/YYYY", "15/03/2026", "/", "2026-03-15", true},
		{"lenient slash D/M/YYYY", "1/7/2026", "/", "2026-07-01", true},

		// calendar-invalid day → clamp ---
		{"april 31 (dot) → clamp 30", "31.04.2026", ".", "2026-04-30", true},
		{"april 31 (slash) → clamp 30", "31/04/2026", "/", "2026-04-30", true},
		{"feb 30 leap → 29", "30.02.2024", ".", "2024-02-29", true},
		{"feb 30 non-leap → 28", "30/02/2025", "/", "2025-02-28", true},
		{"feb 29 non-leap → 28", "29.02.2025", ".", "2025-02-28", true},
		{"nov 31 → 30", "31.11.2026", ".", "2026-11-30", true},
		{"jun 31 → 30", "31/06/2026", "/", "2026-06-30", true},

		// unrecoverable ---
		{"empty", "", ".", "", false},
		{"not a date", "next month", ".", "", false},
		{"missing year", "01.03", ".", "", false},
		{"month 13", "15.13.2026", ".", "", false},
		{"month 0", "15.00.2026", ".", "", false},
		{"day 0", "00.01.2026", ".", "", false},
		// strict parse accepts any calendar-valid year; year bounds only apply
		// to the day-clamp recovery path.
		{"partial year-only", "2026", ".", "", false},
		{"wrong separator", "01.03.2026", "/", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseEuropeanDate(tt.input, tt.sep)
			if got != tt.want || ok != tt.ok {
				t.Errorf("ParseEuropeanDate(%q, %q) = (%q, %v), want (%q, %v)",
					tt.input, tt.sep, got, ok, tt.want, tt.ok)
			}
		})
	}
}
