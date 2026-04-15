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
