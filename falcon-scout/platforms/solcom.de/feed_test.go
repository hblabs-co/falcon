package solcomde

import (
	"testing"
	"time"

	"hblabs.co/falcon/scout/platformkit"
)

// fixedNow pins platformkit.NowFunc to 2026-04-16 so year-inference
// behavior ("Mai 2026" vs. "Januar" → next year) is deterministic.
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
		// --- empty / unparseable ---
		{"empty", "", ""},
		{"whitespace only", "   ", ""},
		{"flexibel vague", "flexibel", ""},
		// "April/Mai" without explicit year → earliest month, year inferred (2026).
		{"month range no year → infer current year", "April/Mai", "2026-04-01"},
		{"Mai/Juni no year → infer", "Mai/Juni", "2026-05-01"},

		// --- immediate variants → canonical ---
		{"asap lowercase", "asap", "ab sofort"},
		{"ASAP uppercase", "ASAP", "ab sofort"},
		{"Asap capitalized", "Asap", "ab sofort"},
		{"ab sofort", "ab sofort", "ab sofort"},
		{"schnellstmöglich", "schnellstmöglich", "ab sofort"},
		{"asasp typo → normalized", "asasp", "ab sofort"},

		// --- already canonical YYYY-MM-DD ---
		{"canonical ISO", "2026-05-04", "2026-05-04"},
		{"canonical leading whitespace", "  2026-05-04  ", "2026-05-04"},

		// --- DD.MM.YYYY ---
		{"dot DD.MM.YYYY", "15.05.2026", "2026-05-15"},

		// --- 2-digit year ---
		{"DD.MM.YY → 20xx", "01.05.26", "2026-05-01"},

		// --- compact 8-digit ---
		{"DDMMYYYY compact", "20032026", "2026-03-20"},

		// --- compound: "early / late" or "early, spätestens late" ---
		{"slash compound w/ spät", "04.05.2026/ spät. 25.05.2026", "2026-05-04"},
		{"simple slash compound", "01.05.2026/01.06.2026", "2026-05-01"},
		{"spaced slash", "11.05.2026 / 01.06.2026", "2026-05-11"},
		{"comma + spätestens", "04.05.2026, spätestens 01.06.2026", "2026-05-04"},
		{"space + spätestens", "27.04.2026 spätestens 01.06.2026", "2026-04-27"},
		{"asap comma late", "asap, spätestens 15.05.2026", "ab sofort"},
		{"asap slash date", "asap / 01.05.2026", "ab sofort"},
		{"asap slash spät", "asap / spät.: 01.06.2026", "ab sofort"},
		{"asap oder n.V.", "asap oder n.V.", "ab sofort"},
		{"ASAP slash spät", "ASAP/ spät. 01.05.2026", "ab sofort"},
		{"ASAP Spät: 2-digit", "ASAP Spät: 15.05.26", "ab sofort"},
		{"compound 2-digit year both sides", "01.05.26/Spät: 01.06.26", "2026-05-01"},
		{"asap comma month-year", "asap, Mai 26", "ab sofort"},
		{"asap spätestens month", "asap, spätestens Mai 2026", "ab sofort"},
		{"asap spätestens zum date", "asap, spätestens zum 01.05.2026", "ab sofort"},

		// --- leading "latest" prefix without early component ---
		{"spät. date only", "spät. 15.04.2026", "2026-04-15"},

		// --- "Anfang / Mitte / Ende <Month>" ---
		{"Mitte April (this year)", "Mitte April", "2026-04-15"},
		{"Ende Mai (this year)", "Ende Mai", "2026-05-31"},
		{"Anfang Juni (this year)", "Anfang Juni", "2026-06-01"},

		// --- "<Month> <year>" text → first day ---
		{"Mai 2026 → first day", "Mai 2026", "2026-05-01"},
		{"April 2026", "April 2026", "2026-04-01"},
		{"Juni 2026", "Juni 2026", "2026-06-01"},
		{"Juli 2026", "Juli 2026", "2026-07-01"},
		{"2-digit year month", "Mai 26", "2026-05-01"},
		{"multi-month earliest w/ year", "April/Mai 2026", "2026-04-01"},
		{"triple-month earliest 2-digit", "Juni/Juli/August 26", "2026-06-01"},

		// --- leading German "earliest start" prefix → strip + retry ---
		{"frühster Start in Mai", "frühster Start in Mai", "2026-05-01"},
		{"im April 2026", "im April 2026", "2026-04-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseStartDate(tt.input); got != tt.want {
				t.Errorf("parseStartDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
