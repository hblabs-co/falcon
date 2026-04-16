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
		{"asasp", true}, // common typo of "asap"
		{"ASASP", true},

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

func TestParseEuropeanDate2DigitYear(t *testing.T) {
	tests := []struct {
		name  string
		input string
		sep   string
		want  string
		ok    bool
	}{
		{"dot 01.05.26 → 2026", "01.05.26", ".", "2026-05-01", true},
		{"dot 31.12.99 → 1999", "31.12.99", ".", "1999-12-31", true},
		{"dot 01.01.00 → 2000", "01.01.00", ".", "2000-01-01", true},
		{"dot 01.01.49 → 2049", "01.01.49", ".", "2049-01-01", true},
		{"dot 01.01.50 → 1950", "01.01.50", ".", "1950-01-01", true},
		{"slash 01/05/26 → 2026", "01/05/26", "/", "2026-05-01", true},

		// calendar-invalid day clamp still works via ParseEuropeanDate
		{"invalid feb 30 non-leap → 28", "30.02.25", ".", "2025-02-28", true},
		{"invalid april 31 → 30", "31.04.26", ".", "2026-04-30", true},

		// rejects
		{"4-digit year (not 2-digit)", "01.05.2026", ".", "", false},
		{"garbage", "foo.bar.ba", ".", "", false},
		{"empty", "", ".", "", false},
		{"wrong separator", "01.05.26", "/", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseEuropeanDate2DigitYear(tt.input, tt.sep)
			if got != tt.want || ok != tt.ok {
				t.Errorf("got (%q,%v) want (%q,%v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestParseCompactDate(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{"20032026", "2026-03-20", true},
		{"01052026", "2026-05-01", true},
		{"31042026", "2026-04-30", true},  // invalid day clamp
		{"32012026", "2026-01-31", true},  // day > 31 clamp
		{"15132026", "", false},           // month 13 unrecoverable
		{"01002026", "", false},           // month 0 unrecoverable
		{"00012026", "", false},           // day 0 unrecoverable
		{"2003202", "", false},            // 7 digits
		{"200320266", "", false},          // 9 digits
		{"2003202A", "", false},           // non-digit
		{"", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, ok := ParseCompactDate(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Errorf("got (%q,%v) want (%q,%v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestParseGermanMonthYear(t *testing.T) {
	// Pin clock to 2026-04-16 so year inference is deterministic.
	prev := NowFunc
	NowFunc = func() time.Time { return time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { NowFunc = prev })

	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"Mai 2026", "Mai 2026", "2026-05-01", true},
		{"April 2026", "April 2026", "2026-04-01", true},
		{"Juni 2026", "Juni 2026", "2026-06-01", true},
		{"Juli 26 (2-digit year)", "Juli 26", "2026-07-01", true},
		{"Mai 99 → 1999", "Mai 99", "1999-05-01", true},
		{"Apr'26 apostrophe", "Apr '26", "2026-04-01", true},
		{"abbreviation", "Dez 2026", "2026-12-01", true},
		{"case-insensitive", "mai 2026", "2026-05-01", true},
		{"multi-month range → earliest", "April/Mai 2026", "2026-04-01", true},
		{"triple range", "Juni/Juli/August 26", "2026-06-01", true},
		{"month only (year inferred)", "Juni", "2026-06-01", true},                 // June ≥ April → current year
		{"month only past → next year", "Januar", "2027-01-01", true},              // Jan < April → next year
		{"trimmed", "  Mai 2026  ", "2026-05-01", true},

		// rejects
		{"empty", "", "", false},
		{"unknown month", "Foobar 2026", "", false},
		{"pure date (not month text)", "01.05.2026", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseGermanMonthYear(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Errorf("got (%q,%v) want (%q,%v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestParseGermanMonthPhrase(t *testing.T) {
	prev := NowFunc
	NowFunc = func() time.Time { return time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC) }
	t.Cleanup(func() { NowFunc = prev })

	tests := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{"Ende Mai", "Ende Mai", "2026-05-31", true},
		{"Anfang Mai", "Anfang Mai", "2026-05-01", true},
		{"Mitte Mai", "Mitte Mai", "2026-05-15", true},
		{"Ende Januar → next year", "Ende Januar", "2027-01-31", true},
		{"Ende Februar 2024 leap", "Ende Februar 2024", "2024-02-29", true},
		{"Ende Februar 2025 non-leap", "Ende Februar 2025", "2025-02-28", true},
		{"case-insensitive", "ende mai", "2026-05-31", true},

		// rejects
		{"no month phrase prefix", "Mai 2026", "", false},
		{"Ende without month", "Ende", "", false},
		{"Ende unknown", "Ende Foobar", "", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ParseGermanMonthPhrase(tt.input)
			if got != tt.want || ok != tt.ok {
				t.Errorf("got (%q,%v) want (%q,%v)", got, ok, tt.want, tt.ok)
			}
		})
	}
}
