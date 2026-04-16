package computerfuturescom

import (
	"testing"
	"time"
)

func TestParseLastUpdated(t *testing.T) {
	now := time.Now().UTC()

	tests := []struct {
		name      string
		input     string
		expected  time.Duration // offset from now
		maxDrift  time.Duration // acceptable margin
		wantRaw   bool          // expect the raw trimmed string back
	}{
		// --- Seconds ---
		{name: "0 seconds ago", input: "Updated: 0 seconds ago", expected: 0, maxDrift: 2 * time.Second},
		{name: "1 second ago", input: "Updated: 1 second ago", expected: 1 * time.Second, maxDrift: 2 * time.Second},
		{name: "30 seconds ago", input: "Updated: 30 seconds ago", expected: 30 * time.Second, maxDrift: 2 * time.Second},
		{name: "99 seconds ago", input: "Updated: 99 seconds ago", expected: 99 * time.Second, maxDrift: 2 * time.Second},

		// --- Minutes ---
		{name: "1 minute ago", input: "Updated: 1 minute ago", expected: 1 * time.Minute, maxDrift: 2 * time.Second},
		{name: "45 minutes ago", input: "Updated: 45 minutes ago", expected: 45 * time.Minute, maxDrift: 2 * time.Second},
		{name: "59 minutes ago", input: "Updated: 59 minutes ago", expected: 59 * time.Minute, maxDrift: 2 * time.Second},

		// --- Hours ---
		{name: "1 hour ago", input: "Updated: 1 hour ago", expected: 1 * time.Hour, maxDrift: 2 * time.Second},
		{name: "5 hours ago", input: "Updated: 5 hours ago", expected: 5 * time.Hour, maxDrift: 2 * time.Second},
		{name: "23 hours ago", input: "Updated: 23 hours ago", expected: 23 * time.Hour, maxDrift: 2 * time.Second},

		// --- Days ---
		{name: "1 day ago", input: "Updated: 1 day ago", expected: 24 * time.Hour, maxDrift: 2 * time.Second},
		{name: "3 days ago", input: "Updated: 3 days ago", expected: 3 * 24 * time.Hour, maxDrift: 2 * time.Second},
		{name: "56 days ago", input: "Updated: 56 days ago", expected: 56 * 24 * time.Hour, maxDrift: 2 * time.Second},

		// --- Weeks ---
		{name: "1 week ago", input: "Updated: 1 week ago", expected: 7 * 24 * time.Hour, maxDrift: 2 * time.Second},
		{name: "4 weeks ago", input: "Updated: 4 weeks ago", expected: 28 * 24 * time.Hour, maxDrift: 2 * time.Second},

		// --- Months ---
		{name: "1 month ago", input: "Updated: 1 month ago", expected: 30 * 24 * time.Hour, maxDrift: 3 * 24 * time.Hour},
		{name: "6 months ago", input: "Updated: 6 months ago", expected: 180 * 24 * time.Hour, maxDrift: 5 * 24 * time.Hour},

		// --- Years ---
		{name: "1 year ago", input: "Updated: 1 year ago", expected: 365 * 24 * time.Hour, maxDrift: 2 * 24 * time.Hour},

		// --- Zero ---
		{name: "0 seconds ago (just now)", input: "Updated: 0 seconds ago", expected: 0, maxDrift: 2 * time.Second},
		{name: "0 days ago (today)", input: "Updated: 0 days ago", expected: 0, maxDrift: 2 * time.Second},

		// --- Compound formats: only the first component is parsed (conservative) ---
		{name: "compound hours+minutes parses as 1 hour", input: "Updated: 1 hour and 30 minutes ago", expected: 1 * time.Hour, maxDrift: 2 * time.Second},
		{name: "compound h+m+s parses as 1 hour", input: "Updated: 1 hour, 30 minutes and 45 seconds ago", expected: 1 * time.Hour, maxDrift: 2 * time.Second},

		// --- Tolerant: works without "Updated:" prefix ---
		{name: "no prefix still parses", input: "3 days ago", expected: 3 * 24 * time.Hour, maxDrift: 2 * time.Second},

		// --- Garbage / edge cases ---
		{name: "empty string", input: "", wantRaw: true},
		{name: "whitespace only", input: "   ", wantRaw: true},
		{name: "just now", input: "Updated: just now", wantRaw: true},
		{name: "negative number", input: "Updated: -1 days ago", wantRaw: true},
		{name: "non-numeric", input: "Updated: many days ago", wantRaw: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseLastUpdated(tt.input)

			if tt.wantRaw {
				// parseLastUpdated strips "Updated: " and " ago" before trying to
				// parse, so the fallback is the stripped version. Just verify it's
				// NOT valid RFC3339 — that confirms it wasn't parsed as a date.
				if _, err := time.Parse(time.RFC3339, result); err == nil {
					t.Errorf("expected raw/unparseable string, got valid RFC3339: %q", result)
				}
				return
			}

			parsed, err := time.Parse(time.RFC3339, result)
			if err != nil {
				t.Fatalf("parseLastUpdated(%q) = %q — not valid RFC3339: %v", tt.input, result, err)
			}

			expectedTime := now.Add(-tt.expected)
			drift := absDuration(parsed.Sub(expectedTime))
			if drift > tt.maxDrift {
				t.Errorf("parseLastUpdated(%q) = %s, expected ~%s (drift %s > max %s)",
					tt.input, result, expectedTime.Format(time.RFC3339), drift, tt.maxDrift)
			}
		})
	}
}

func TestParseLastUpdatedOutputFormat(t *testing.T) {
	result := parseLastUpdated("Updated: 3 days ago")

	parsed, err := time.Parse(time.RFC3339, result)
	if err != nil {
		t.Fatalf("not RFC3339: %q — %v", result, err)
	}

	if parsed.Location() != time.UTC {
		t.Errorf("expected UTC, got %v", parsed.Location())
	}
}

func TestParseSkills(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int // expected number of skills
	}{
		{name: "empty", input: "", want: 0},
		{name: "single", input: "React", want: 1},
		{name: "comma separated", input: "Java, Spring boot, JSF, JPA", want: 4},
		{name: "semicolon separated", input: "AWS; Lambda; DynamoDB; S3", want: 4},
		{name: "mixed separators", input: "C#, .NET; Angular", want: 3},
		{name: "trailing comma", input: "Java, Python,", want: 2},
		{name: "extra whitespace", input: "  React ,  Vue  , Angular  ", want: 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseSkills(tt.input)
			if len(got) != tt.want {
				t.Errorf("parseSkills(%q) returned %d skills, want %d: %v", tt.input, len(got), tt.want, got)
			}
		})
	}
}

func absDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -d
	}
	return d
}

func TestParseStartDate(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// --- empty / nothing to parse ---
		{name: "empty", input: "", want: ""},
		{name: "whitespace only", input: "   ", want: ""},

		// --- ASAP / immediate variants → canonical "ab sofort" ---
		{name: "ASAP uppercase", input: "ASAP", want: "ab sofort"},
		{name: "asap lowercase", input: "asap", want: "ab sofort"},
		{name: "Asap mixed case", input: "Asap", want: "ab sofort"},
		{name: "immediate", input: "immediate", want: "ab sofort"},
		{name: "sofort", input: "sofort", want: "ab sofort"},
		{name: "ab sofort (already canonical)", input: "ab sofort", want: "ab sofort"},
		{name: "Ab Sofort capitalized", input: "Ab Sofort", want: "ab sofort"},
		{name: "nächstmöglich", input: "nächstmöglich", want: "ab sofort"},
		{name: "trimmed ASAP", input: "  ASAP  ", want: "ab sofort"},

		// --- DD.MM.YYYY (dot) → canonical YYYY-MM-DD ---
		{name: "dot DD.MM.YYYY", input: "01.03.2026", want: "2026-03-01"},
		{name: "dot D.M.YYYY lenient", input: "1.7.2026", want: "2026-07-01"},
		{name: "dot D.MM.YYYY lenient", input: "1.07.2026", want: "2026-07-01"},
		{name: "dot DD.M.YYYY lenient", input: "01.7.2026", want: "2026-07-01"},
		{name: "dot trimmed", input: "  01.03.2026  ", want: "2026-03-01"},

		// --- DD/MM/YYYY (slash) → canonical YYYY-MM-DD ---
		{name: "slash DD/MM/YYYY", input: "15/03/2026", want: "2026-03-15"},
		{name: "slash 01/06/2026", input: "01/06/2026", want: "2026-06-01"},
		{name: "slash D/M/YYYY lenient", input: "1/7/2026", want: "2026-07-01"},
		{name: "slash trimmed", input: "  15/03/2026  ", want: "2026-03-15"},

		// --- calendar-invalid day → clamp to last day of month ---
		{name: "dot april 31 → clamped to 30", input: "31.04.2026", want: "2026-04-30"},
		{name: "slash april 31 → clamped to 30", input: "31/04/2026", want: "2026-04-30"},
		{name: "slash feb 30 non-leap → clamped to 28", input: "30/02/2025", want: "2025-02-28"},
		{name: "dot feb 30 leap → clamped to 29", input: "30.02.2024", want: "2024-02-29"},
		{name: "slash nov 31 → clamped to 30", input: "31/11/2026", want: "2026-11-30"},

		// --- unrecoverable garbage → drop ---
		{name: "month 13", input: "15.13.2026", want: ""},
		{name: "day 00", input: "00.01.2026", want: ""},
		{name: "month 00", input: "15.00.2026", want: ""},
		{name: "garbage text", input: "next month", want: ""},
		{name: "partial year only", input: "2026", want: ""},
		{name: "random number", input: "12345", want: ""},
		{name: "ISO date (wrong format for this platform)", input: "2026-03-01", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := parseStartDate(tt.input); got != tt.want {
				t.Errorf("parseStartDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
