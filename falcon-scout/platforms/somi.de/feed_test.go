package somide

import (
	"encoding/json"
	"testing"
	"time"

	"hblabs.co/falcon/scout/platformkit"
)

// fixedNow pins platformkit.NowFunc to 2026-04-16 for deterministic
// month-phrase year inference ("Ende Mai" → 2026-05-31, "Ende Januar" →
// 2027-01-31).
func fixedNow(t *testing.T) {
	t.Helper()
	prev := platformkit.NowFunc
	platformkit.NowFunc = func() time.Time {
		return time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	}
	t.Cleanup(func() { platformkit.NowFunc = prev })
}

func TestBuildDuration(t *testing.T) {
	tests := []struct {
		name string
		date *apiDate
		want string
	}{
		{
			name: "nil date",
			date: nil,
			want: "",
		},
		{
			name: "both full dates — 12 months",
			date: &apiDate{Start: "01.05.2026", End: "01.05.2027"},
			want: "12 Monate",
		},
		{
			name: "both full dates — 5 months",
			date: &apiDate{Start: "01.04.2026", End: "30.09.2026"},
			want: "5 Monate",
		},
		{
			name: "end is duration string (Monate + Verlängerung)",
			date: &apiDate{Start: "Ab Sofort", End: "6 Monate + Verlängerung"},
			want: "6 Monate + Verlängerung",
		},
		{
			name: "end is range of months",
			date: &apiDate{Start: "Ab Sofort", End: "3-6 Monate"},
			want: "3-6 Monate",
		},
		{
			name: "end is only year",
			date: &apiDate{Start: "ab sofort", End: "2028"},
			want: "bis 2028",
		},
		{
			name: "end is month-year",
			date: &apiDate{Start: "18.05.2026", End: "01.2027"},
			want: "bis 01.2027",
		},
		{
			name: "end is null",
			date: &apiDate{Start: "ab sofort", End: ""},
			want: "",
		},
		{
			name: "description appended to computed duration",
			date: &apiDate{Start: "01.05.2026", End: "31.07.2026", Description: "Mit Option auf Verlängerung"},
			want: "3 Monate · Mit Option auf Verlängerung",
		},
		{
			name: "description only (no dates)",
			date: &apiDate{Start: "ab sofort", End: "", Description: "Mit Option auf Verlängerung"},
			want: "Mit Option auf Verlängerung",
		},
		{
			name: "invalid dates (end before start) returns empty",
			date: &apiDate{Start: "01.12.2026", End: "01.01.2026"},
			want: "",
		},

		// --- API error / garbage cases ---
		{
			name: "both start and end are 'asap' (api error)",
			date: &apiDate{Start: "asap", End: "asap"},
			want: "",
		},
		{
			name: "both start and end are 'ab sofort'",
			date: &apiDate{Start: "ab sofort", End: "ab sofort"},
			want: "",
		},
		{
			name: "both start and end are 'nächstmöglich'",
			date: &apiDate{Start: "nächstmöglich", End: "nächstmöglich"},
			want: "",
		},
		{
			name: "end is uppercase ASAP (not a date)",
			date: &apiDate{Start: "01.05.2026", End: "ASAP"},
			want: "",
		},
		{
			name: "end is arbitrary garbage 'TBD'",
			date: &apiDate{Start: "01.05.2026", End: "TBD"},
			want: "",
		},
		{
			name: "whitespace-only start and end",
			date: &apiDate{Start: "   ", End: "   "},
			want: "",
		},
		{
			name: "whitespace-only dates but description present",
			date: &apiDate{Start: "  ", End: "  ", Description: "Mit Option auf Verlängerung"},
			want: "Mit Option auf Verlängerung",
		},
		{
			name: "description also whitespace-only",
			date: &apiDate{Start: "", End: "", Description: "   "},
			want: "",
		},
		{
			name: "all three fields empty",
			date: &apiDate{Start: "", End: "", Description: ""},
			want: "",
		},
		{
			name: "garbage end with garbage description",
			date: &apiDate{Start: "asap", End: "asap", Description: "asap"},
			want: "asap",
		},

		// --- mixed valid+invalid combinations ---
		{
			name: "asap start with year end → bis YYYY",
			date: &apiDate{Start: "asap", End: "2028"},
			want: "bis 2028",
		},
		{
			name: "ab sofort start with month-year end + description",
			date: &apiDate{Start: "ab sofort", End: "06.2027", Description: "plus Option auf Verlängerung"},
			want: "bis 06.2027 · plus Option auf Verlängerung",
		},
		{
			name: "full date start, month-year end (no full-date pair)",
			date: &apiDate{Start: "01.05.2026", End: "06.2027"},
			want: "bis 06.2027",
		},
		{
			name: "duration string in both start and end (end wins)",
			date: &apiDate{Start: "3 Monate", End: "6 Monate"},
			want: "6 Monate",
		},
		{
			name: "duration prefixed with 'ca.'",
			date: &apiDate{Start: "ab sofort", End: "ca. 12 Monate"},
			want: "ca. 12 Monate",
		},
		{
			name: "jahr-based duration",
			date: &apiDate{Start: "Ab Sofort", End: "1 Jahr"},
			want: "1 Jahr",
		},
		{
			name: "end is ISO timestamp (not handled by isFullDate → ignored)",
			date: &apiDate{Start: "asap", End: "2026-04-14T16:06:38"},
			want: "",
		},
		{
			name: "only description, rest all nonsense",
			date: &apiDate{Start: "asap", End: "asap", Description: "Laufzeit nach Absprache"},
			want: "Laufzeit nach Absprache",
		},
		{
			name: "same day start/end yields 0 months → empty (no 'bis' fallback)",
			date: &apiDate{Start: "15.05.2026", End: "15.05.2026"},
			want: "",
		},
		{
			name: "full-date pair with <1 month distance but description present",
			date: &apiDate{Start: "01.05.2026", End: "15.05.2026", Description: "kurzer Einsatz"},
			want: "kurzer Einsatz",
		},
		{
			name: "invalid calendar date in end",
			date: &apiDate{Start: "01.05.2026", End: "31.04.2027"},
			want: "",
		},
		{
			name: "empty start with year end",
			date: &apiDate{Start: "", End: "2028"},
			want: "bis 2028",
		},
		{
			name: "empty start with month-year end",
			date: &apiDate{Start: "", End: "01.2027"},
			want: "bis 01.2027",
		},
		{
			name: "empty start with duration end",
			date: &apiDate{Start: "", End: "6 Monate"},
			want: "6 Monate",
		},
		{
			name: "garbage start with duration end",
			date: &apiDate{Start: "foo", End: "6 Monate"},
			want: "6 Monate",
		},
		{
			name: "trim surrounding spaces everywhere",
			date: &apiDate{Start: " 01.05.2026 ", End: " 31.07.2026 ", Description: " Mit Option auf Verlängerung "},
			want: "3 Monate · Mit Option auf Verlängerung",
		},

		// --- more calendar-invalid shapes ---
		{
			name: "end is calendar-invalid day 32",
			date: &apiDate{Start: "01.05.2026", End: "32.01.2026"},
			want: "",
		},
		{
			name: "end is calendar-invalid month 13",
			date: &apiDate{Start: "01.05.2026", End: "15.13.2026"},
			want: "",
		},
		{
			name: "end is calendar-invalid feb 29 non-leap",
			date: &apiDate{Start: "01.01.2025", End: "29.02.2025"},
			want: "",
		},
		{
			name: "calendar-invalid end but description saves it",
			date: &apiDate{Start: "01.05.2026", End: "31.04.2027", Description: "Laufzeit nach Absprache"},
			want: "Laufzeit nach Absprache",
		},

		// --- other garbage-start + valid-end combos ---
		{
			name: "garbage start with year end",
			date: &apiDate{Start: "foo", End: "2028"},
			want: "bis 2028",
		},
		{
			name: "garbage start with month-year end",
			date: &apiDate{Start: "foo", End: "01.2027"},
			want: "bis 01.2027",
		},
		{
			name: "empty start with full date end",
			date: &apiDate{Start: "", End: "31.12.2026"},
			want: "bis 31.12.2026",
		},

		// --- trimming (already covered for full dates; cover other shapes) ---
		{
			name: "trimmed duration-string end",
			date: &apiDate{Start: "ab sofort", End: " 6 Monate + Verlängerung "},
			want: "6 Monate + Verlängerung",
		},
		{
			name: "trimmed year end",
			date: &apiDate{Start: "ab sofort", End: " 2028 "},
			want: "bis 2028",
		},

		// --- duration string variants (singular / dash flavors / "ca") ---
		{
			name: "singular Monat",
			date: &apiDate{Start: "ab sofort", End: "1 Monat"},
			want: "1 Monat",
		},
		{
			name: "plural Jahre",
			date: &apiDate{Start: "ab sofort", End: "2 Jahre"},
			want: "2 Jahre",
		},
		{
			name: "singular Woche",
			date: &apiDate{Start: "ab sofort", End: "1 Woche"},
			want: "1 Woche",
		},
		{
			name: "singular Tag",
			date: &apiDate{Start: "ab sofort", End: "1 Tag"},
			want: "1 Tag",
		},
		{
			name: "en-dash range 3–6 Monate",
			date: &apiDate{Start: "ab sofort", End: "3–6 Monate"},
			want: "3–6 Monate",
		},
		{
			name: "em-dash range 3 — 6 Monate",
			date: &apiDate{Start: "ab sofort", End: "3 — 6 Monate"},
			want: "3 — 6 Monate",
		},
		{
			name: "spaced dash range 3 - 6 Monate",
			date: &apiDate{Start: "ab sofort", End: "3 - 6 Monate"},
			want: "3 - 6 Monate",
		},
		{
			name: "ca without dot",
			date: &apiDate{Start: "ab sofort", End: "ca 12 Monate"},
			want: "ca 12 Monate",
		},
		{
			name: "ca without space after dot",
			date: &apiDate{Start: "ab sofort", End: "ca.12 Monate"},
			want: "ca.12 Monate",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildDuration(tt.date)
			if got != tt.want {
				t.Errorf("buildDuration(%+v) = %q, want %q", tt.date, got, tt.want)
			}
		})
	}
}

func TestParseSomiDate(t *testing.T) {
	fixedNow(t)
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "trimmed full date", input: " 01.05.2026 ", want: "2026-05-01"},
		{name: "april 31 → clamped to 30", input: "31.04.2027", want: "2027-04-30"},
		{name: "trimmed year → last day", input: " 2028 ", want: "2028-12-31"},
		{name: "trimmed month-year → last day", input: " 01.2027 ", want: "2027-01-31"},
		{name: "DD.MM.YYYY", input: "01.05.2026", want: "2026-05-01"},
		{name: "ISO timestamp", input: "2026-04-14T16:06:38", want: "2026-04-14"},
		{name: "already canonical", input: "2026-04-14", want: "2026-04-14"},
		{name: "ab sofort", input: "ab sofort", want: "ab sofort"},
		{name: "Ab Sofort normalized", input: "Ab Sofort", want: "ab sofort"},
		{name: "asap normalized", input: "asap", want: "ab sofort"},
		{name: "nächstmöglich normalized", input: "nächstmöglich", want: "ab sofort"},
		{name: "nächstmöglichst normalized", input: "nächstmöglichst", want: "ab sofort"},
		{name: "sofort normalized", input: "sofort", want: "ab sofort"},
		{name: "naechstmoeglich ASCII normalized", input: "naechstmoeglich", want: "ab sofort"},
		{name: "zum nächstmöglichen zeitpunkt", input: "zum nächstmöglichen Zeitpunkt", want: "ab sofort"},
		{name: "month-year → last day", input: "01.2027", want: "2027-01-31"},
		{name: "year only → last day", input: "2028", want: "2028-12-31"},
		{name: "empty", input: "", want: ""},

		// --- calendar-invalid day → clamped to last valid day of month ---
		{name: "day 32 → last day of jan (31)", input: "32.01.2026", want: "2026-01-31"},
		{name: "feb 29 non-leap → clamped to 28", input: "29.02.2025", want: "2025-02-28"},
		{name: "feb 32 leap → clamped to 29", input: "32.02.2024", want: "2024-02-29"},
		{name: "feb 32 non-leap → clamped to 28", input: "32.02.2025", want: "2025-02-28"},
		{name: "nov 31 → clamped to 30", input: "31.11.2026", want: "2026-11-30"},
		{name: "jun 31 → clamped to 30", input: "31.06.2026", want: "2026-06-30"},
		{name: "sep 31 → clamped to 30", input: "31.09.2026", want: "2026-09-30"},

		// --- unrecoverable: day 0 (no natural fallback) ---
		{name: "day 00 stays dropped", input: "00.01.2026", want: ""},

		// --- unrecoverable: month out of range (can't guess which month) ---
		{name: "month 13 stays dropped", input: "15.13.2026", want: ""},
		{name: "month 00 stays dropped", input: "15.00.2026", want: ""},
		{name: "invalid month + day combo stays dropped", input: "32.13.2026", want: ""},

		// --- non-standard shapes that can't be inferred → drop to empty ---
		{name: "text month unsupported", input: "Mai 2027", want: ""},

		// --- partial shapes → last day of period (always YYYY-MM-DD) ---
		{name: "iso month → last day of month", input: "2027-01", want: "2027-01-31"},
		{name: "slash MM/YYYY → last day", input: "05/2027", want: "2027-05-31"},
		{name: "slash M/YYYY → last day", input: "5/2027", want: "2027-05-31"},
		{name: "slash 11/2025 → 2025-11-30 (nov=30)", input: "11/2025", want: "2025-11-30"},
		{name: "slash 12/2025 → 2025-12-31 (dec=31)", input: "12/2025", want: "2025-12-31"},
		{name: "slash 06/2026 → 2026-06-30 (jun=30)", input: "06/2026", want: "2026-06-30"},
		{name: "slash 02/2024 → 2024-02-29 (leap)", input: "02/2024", want: "2024-02-29"},
		{name: "slash 02/2025 → 2025-02-28 (non-leap)", input: "02/2025", want: "2025-02-28"},
		{name: "dot 1.2027 → 2027-01-31", input: "1.2027", want: "2027-01-31"},
		{name: "single-digit day D.MM.YYYY", input: "3.11.2025", want: "2025-11-03"},
		{name: "single-digit D.M.YYYY", input: "3.1.2025", want: "2025-01-03"},

		// --- "Ab dem ..." / "ab ..." prefix stripping ---
		{name: "Ab dem DD.MM.YYYY", input: "Ab dem 01.03.2026", want: "2026-03-01"},
		{name: "ab dem D.MM.YYYY lenient", input: "ab dem 3.11.2025", want: "2025-11-03"},
		{name: "ab prefix with full date", input: "ab 01.05.2026", want: "2026-05-01"},
		{name: "ab prefix with year → last day of year", input: "ab 2025", want: "2025-12-31"},
		{name: "Ab dem slash month-year → last day", input: "Ab dem 06/2026", want: "2026-06-30"},

		// --- German month phrases: "Ende|Anfang|Mitte <Month> [YYYY]" ---
		// (nowFunc pinned to 2026-04-16 via fixedNow; May≥April → current year)
		{name: "Ende Mai (infer 2026)", input: "Ende Mai", want: "2026-05-31"},
		{name: "Ende mai lowercase", input: "ende mai", want: "2026-05-31"},
		{name: "Anfang Mai (infer 2026)", input: "Anfang Mai", want: "2026-05-01"},
		{name: "Mitte Mai (infer 2026)", input: "Mitte Mai", want: "2026-05-15"},
		{name: "Ende Januar → next year", input: "Ende Januar", want: "2027-01-31"},
		{name: "Ende Februar 2024 (leap)", input: "Ende Februar 2024", want: "2024-02-29"},
		{name: "Ende Februar 2025 (non-leap)", input: "Ende Februar 2025", want: "2025-02-28"},
		{name: "Ende Juni (30 days)", input: "Ende Juni", want: "2026-06-30"},
		{name: "Ende Dezember", input: "Ende Dezember", want: "2026-12-31"},
		{name: "Anfang April → current month", input: "Anfang April", want: "2026-04-01"},
		{name: "Ende Mai explicit year", input: "Ende Mai 2028", want: "2028-05-31"},
		{name: "month abbreviation Dez", input: "Ende Dez", want: "2026-12-31"},
		{name: "month abbreviation Jan → next year", input: "Ende Jan", want: "2027-01-31"},
		{name: "März with umlaut", input: "Ende März", want: "2027-03-31"},
		{name: "maerz ASCII fallback", input: "Ende maerz", want: "2027-03-31"},
		{name: "Ende without month → drop", input: "Ende", want: ""},
		{name: "Ende Foobar → drop", input: "Ende Foobar", want: ""},

		// --- still-vague / duration / non-date garbage → drop ---
		{name: "ab April (no day, no 'Ende/Anfang/Mitte')", input: "ab April", want: ""},
		{name: "langfristig", input: "langfristig", want: ""},
		{name: "Langfristig", input: "Langfristig", want: ""},
		{name: "plus 24 Monate", input: "+ 24 Monate", want: ""},
		{name: "duration string 12 Monate", input: "12 Monate", want: ""},
		{name: "range duration 3-6 Monate", input: "3-6 Monate", want: ""},
		{name: "1 Jahr in date field", input: "1 Jahr", want: ""},
		{name: "6 Monate + Verlängerung", input: "6 Monate + Verlängerung", want: ""},

		// --- case-insensitive variants (normalized to "ab sofort") ---
		{name: "capitalized Ab sofort", input: "Ab sofort", want: "ab sofort"},
		{name: "upper AB SOFORT", input: "AB SOFORT", want: "ab sofort"},
		{name: "capitalized Asap", input: "Asap", want: "ab sofort"},
		{name: "upper ASAP", input: "ASAP", want: "ab sofort"},
		{name: "upper NÄCHSTMÖGLICH", input: "NÄCHSTMÖGLICH", want: "ab sofort"},

		// --- trimmed keywords (normalized) ---
		{name: "trimmed ab sofort", input: "  ab sofort  ", want: "ab sofort"},
		{name: "trimmed asap", input: "  asap  ", want: "ab sofort"},
		{name: "trimmed nächstmöglich", input: "  nächstmöglich  ", want: "ab sofort"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSomiDate(tt.input)
			if got != tt.want {
				t.Errorf("parseSomiDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestIsDurationString(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"1 Monat", true},
		{"2 Jahre", true},
		{"3-6 Monate", true},
		{"6 Monate + Verlängerung", true},
		{"1 Jahr", true},
		{"2 Wochen", true},
		{"30 Tage", true},
		{"1 Woche", true},
		{"1 Tag", true},

		// range / dash variants
		{"3 - 6 Monate", true},
		{"3–6 Monate", true},   // en-dash
		{"3 — 6 Monate", true}, // em-dash

		// "ca." variants
		{"ca. 12 Monate", true},
		{"ca 12 Monate", true},
		{"ca.12 Monate", true},
		{"ca 6 Monate + Verlängerung", true},

		// non-matches
		{"01.05.2026", false},
		{"2028", false},
		{"01.2027", false},
		{"Mai 2027", false},
		{"foo", false},
		{"", false},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := isDurationString(tt.input); got != tt.want {
				t.Errorf("isDurationString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestBuildRate(t *testing.T) {
	tests := []struct {
		name       string
		payment    *apiPayment
		wantOnsite rateBlock
		wantRemote rateBlock
	}{
		{
			name:    "nil payment → empty result",
			payment: nil,
		},
		{
			name: "both blocks all-zero → empty blocks",
			payment: &apiPayment{
				Onsite: &apiPaymentBlock{From: float64(0), To: float64(0), Type: ""},
				Remote: &apiPaymentBlock{From: float64(0), To: float64(0), Type: ""},
			},
		},
		{
			name: "both sides populated with different rates",
			payment: &apiPayment{
				Onsite: &apiPaymentBlock{From: "65", To: float64(0), Type: "hourly"},
				Remote: &apiPaymentBlock{From: "60", To: float64(0), Type: "hourly"},
			},
			wantOnsite: rateBlock{From: 65, Type: "hourly"},
			wantRemote: rateBlock{From: 60, Type: "hourly"},
		},
		{
			name: "onsite only with range, remote empty",
			payment: &apiPayment{
				Onsite: &apiPaymentBlock{From: "60", To: "65", Type: "hourly"},
				Remote: &apiPaymentBlock{From: float64(0), To: float64(0), Type: ""},
			},
			wantOnsite: rateBlock{From: 60, To: 65, Type: "hourly"},
		},
		{
			name: "numeric from/to — real API variant",
			payment: &apiPayment{
				Onsite: &apiPaymentBlock{From: float64(60), To: float64(70), Type: "hourly"},
			},
			wantOnsite: rateBlock{From: 60, To: 70, Type: "hourly"},
		},
		{
			name: "european decimal comma",
			payment: &apiPayment{
				Onsite: &apiPaymentBlock{From: "60,50", To: "75,25", Type: "hourly"},
			},
			wantOnsite: rateBlock{From: 60.5, To: 75.25, Type: "hourly"},
		},
		{
			name: "description trimmed and preserved",
			payment: &apiPayment{
				Onsite: &apiPaymentBlock{From: "65", To: float64(0), Type: "hourly", Description: "  plus Spesen  "},
			},
			wantOnsite: rateBlock{From: 65, Type: "hourly", Description: "plus Spesen"},
		},
		{
			name: "daily rate on remote side only",
			payment: &apiPayment{
				Remote: &apiPaymentBlock{From: "600", To: "700", Type: "daily"},
			},
			wantRemote: rateBlock{From: 600, To: 700, Type: "daily"},
		},
		{
			name: "garbage strings → zero (block still tracked but no data)",
			payment: &apiPayment{
				Onsite: &apiPaymentBlock{From: "tbd", To: "n/a", Type: "hourly"},
			},
			wantOnsite: rateBlock{Type: "hourly"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRate(tt.payment)
			if got.Onsite != tt.wantOnsite {
				t.Errorf("Onsite = %+v, want %+v", got.Onsite, tt.wantOnsite)
			}
			if got.Remote != tt.wantRemote {
				t.Errorf("Remote = %+v, want %+v", got.Remote, tt.wantRemote)
			}
		})
	}
}

func TestFormatRateDisplay(t *testing.T) {
	tests := []struct {
		name         string
		onsite       rateBlock
		remote       rateBlock
		preferRemote bool
		want         string
	}{
		{
			name: "both empty → Auf Anfrage",
			want: "Auf Anfrage",
		},
		{
			name:   "onsite only",
			onsite: rateBlock{From: 60, To: 65, Type: "hourly"},
			want:   "60-65 €/h",
		},
		{
			name:   "remote only",
			remote: rateBlock{From: 55, To: 60, Type: "hourly"},
			want:   "55-60 €/h",
		},
		{
			name:   "onsite and remote identical → collapsed",
			onsite: rateBlock{From: 60, To: 65, Type: "hourly"},
			remote: rateBlock{From: 60, To: 65, Type: "hourly"},
			want:   "60-65 €/h",
		},
		{
			name:   "different blocks → compound display",
			onsite: rateBlock{From: 70, To: 75, Type: "hourly"},
			remote: rateBlock{From: 65, To: 70, Type: "hourly"},
			want:   "70-75 €/h (onsite) · 65-70 €/h (remote)",
		},
		{
			name:   "different 'ab' rates",
			onsite: rateBlock{From: 65, Type: "hourly"},
			remote: rateBlock{From: 60, Type: "hourly"},
			want:   "ab 65 €/h (onsite) · ab 60 €/h (remote)",
		},
		{
			name:         "preferRemote but onsite-only → onsite",
			onsite:       rateBlock{From: 70, To: 75, Type: "hourly"},
			preferRemote: true,
			want:         "70-75 €/h",
		},
		{
			name:   "different types onsite/remote",
			onsite: rateBlock{From: 600, To: 700, Type: "daily"},
			remote: rateBlock{From: 65, To: 75, Type: "hourly"},
			want:   "600-700 €/Tag (onsite) · 65-75 €/h (remote)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatRateDisplay(tt.onsite, tt.remote, tt.preferRemote)
			if got != tt.want {
				t.Errorf("formatRateDisplay() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPickPrimaryBlock(t *testing.T) {
	onsite := rateBlock{From: 65, Type: "hourly"}
	remote := rateBlock{From: 60, Type: "hourly"}
	empty := rateBlock{}

	tests := []struct {
		name         string
		onsite       rateBlock
		remote       rateBlock
		preferRemote bool
		want         rateBlock
	}{
		{"prefer remote and remote has data", onsite, remote, true, remote},
		{"prefer remote but remote empty → onsite", onsite, empty, true, onsite},
		{"prefer onsite (default)", onsite, remote, false, onsite},
		{"onsite empty, remote has data", empty, remote, false, remote},
		{"both empty → returns remote (zero)", empty, empty, false, empty},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pickPrimaryBlock(tt.onsite, tt.remote, tt.preferRemote)
			if got != tt.want {
				t.Errorf("got %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestRateToFloat(t *testing.T) {
	tests := []struct {
		name string
		in   any
		want float64
	}{
		{"nil", nil, 0},
		{"float64", float64(65.5), 65.5},
		{"int", 65, 65},
		{"int64", int64(70), 70},
		{"string number", "65", 65},
		{"string with comma", "60,50", 60.5},
		{"empty string", "", 0},
		{"whitespace string", "   ", 0},
		{"padded string", "  65  ", 65},
		{"garbage string", "tbd", 0},
		{"json.Number", json.Number("72"), 72},
		{"unsupported bool", true, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := rateToFloat(tt.in); got != tt.want {
				t.Errorf("rateToFloat(%#v) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func TestUnmarshalPaymentRealAPI(t *testing.T) {
	// Exercise the real shapes seen in examples/response.json, where
	// from/to can be either a JSON string ("65") or a JSON number (0).
	raw := []byte(`{
		"onsite": {"from": "65", "to": 0, "type": "hourly", "description": null},
		"remote": {"from": "60", "to": "75", "type": "hourly", "description": null}
	}`)
	var p apiPayment
	if err := json.Unmarshal(raw, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if rateToFloat(p.Onsite.From) != 65 {
		t.Errorf("onsite.from = %v, want 65", p.Onsite.From)
	}
	if rateToFloat(p.Onsite.To) != 0 {
		t.Errorf("onsite.to = %v, want 0", p.Onsite.To)
	}
	if rateToFloat(p.Remote.From) != 60 {
		t.Errorf("remote.from = %v, want 60", p.Remote.From)
	}
	if rateToFloat(p.Remote.To) != 75 {
		t.Errorf("remote.to = %v, want 75", p.Remote.To)
	}
}

func TestMonthsBetween(t *testing.T) {
	tests := []struct {
		start, end string
		want       int
	}{
		{"01.05.2026", "01.05.2027", 12},
		{"01.04.2026", "30.09.2026", 5},
		{"01.05.2026", "31.07.2026", 3},
		{"01.05.2026", "15.05.2026", 0}, // less than a month
		{"invalid", "01.05.2026", 0},
		{"01.05.2026", "invalid", 0},
		{"01.12.2026", "01.01.2026", 0}, // end before start
		{"01.05.2026", "31.04.2027", 0}, // calendar-invalid end
		{"31.01.2026", "28.02.2026", 0}, // 28 days (<30) → 0

		// more calendar-invalid shapes
		{"01.05.2026", "32.01.2026", 0}, // invalid day
		{"01.05.2026", "15.13.2026", 0}, // invalid month
		{"01.05.2026", "29.02.2025", 0}, // feb 29 non-leap

		// calendar edge cases (month-end rollover)
		{"31.01.2026", "01.03.2026", 1}, // 29 days spanning jan→mar
		{"30.01.2026", "28.02.2026", 0}, // 29 days → not quite 1 month
		{"28.02.2026", "28.03.2026", 1}, // exact 1-month anniversary
		{"29.02.2024", "29.03.2024", 1}, // leap-year anniversary
	}
	for _, tt := range tests {
		t.Run(tt.start+"→"+tt.end, func(t *testing.T) {
			if got := monthsBetween(tt.start, tt.end); got != tt.want {
				t.Errorf("monthsBetween(%q, %q) = %d, want %d", tt.start, tt.end, got, tt.want)
			}
		})
	}
}
