package platformkit

import (
	"strconv"
	"strings"
	"time"
)

// NowFunc returns the current time. Package-level so tests can stub the
// clock when asserting behavior that depends on year inference (e.g.
// "Ende Mai" → current vs. next year).
var NowFunc = time.Now

// GermanMonths maps lowercased German month names (full and common
// abbreviations) to their 1..12 integer.
var GermanMonths = map[string]int{
	"januar": 1, "jan": 1, "jän": 1,
	"februar": 2, "feb": 2,
	"märz": 3, "maerz": 3, "mär": 3, "mar": 3,
	"april": 4, "apr": 4,
	"mai": 5,
	"juni": 6, "jun": 6,
	"juli": 7, "jul": 7,
	"august": 8, "aug": 8,
	"september": 9, "sept": 9, "sep": 9,
	"oktober": 10, "okt": 10,
	"november": 11, "nov": 11,
	"dezember": 12, "dez": 12,
}

// GermanMonth looks up the 1..12 integer for a German month name. Input
// is matched case-insensitively after TrimSpace. Returns (0, false) for
// unknown names.
func GermanMonth(s string) (int, bool) {
	m, ok := GermanMonths[strings.ToLower(strings.TrimSpace(s))]
	return m, ok
}

// ParseGermanMonthPhrase resolves "Anfang|Mitte|Ende <Month> [YYYY]" into
// a canonical YYYY-MM-DD:
//
//   - "Anfang Mai"        → May 1st of inferred year
//   - "Mitte Mai"         → May 15th of inferred year
//   - "Ende Mai"          → last day of May of inferred year
//   - "Ende Februar 2024" → 2024-02-29 (explicit year; leap-year aware)
//
// Year inference (when omitted): current year if month ≥ current month,
// otherwise next year. "Ends in January" posted in April means next
// January. Returns ("", false) if s doesn't match this shape.
func ParseGermanMonthPhrase(s string) (string, bool) {
	lower := strings.ToLower(strings.TrimSpace(s))

	var day int // 0 = last-day-of-month sentinel
	var rest string
	switch {
	case strings.HasPrefix(lower, "anfang "):
		day = 1
		rest = strings.TrimSpace(lower[len("anfang "):])
	case strings.HasPrefix(lower, "mitte "):
		day = 15
		rest = strings.TrimSpace(lower[len("mitte "):])
	case strings.HasPrefix(lower, "ende "):
		day = 0
		rest = strings.TrimSpace(lower[len("ende "):])
	default:
		return "", false
	}

	parts := strings.Fields(rest)
	if len(parts) == 0 {
		return "", false
	}
	month, ok := GermanMonth(parts[0])
	if !ok {
		return "", false
	}

	year := 0
	if len(parts) >= 2 {
		if y, err := strconv.Atoi(parts[1]); err == nil && y >= 1900 && y <= 9999 {
			year = y
		}
	}
	if year == 0 {
		now := NowFunc()
		year = now.Year()
		if time.Month(month) < now.Month() {
			year++
		}
	}

	var t time.Time
	if day == 0 {
		t = LastDayOfMonth(time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC))
	} else {
		t = time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	}
	return t.Format("2006-01-02"), true
}

// ParseGermanMonthYear resolves a "<GermanMonth> <year>" expression into
// a canonical YYYY-MM-01 (first day of the month — the "earliest" day
// of the period, appropriate for start-date fields). Accepts:
//
//   - "Mai 2026"  → "2026-05-01"
//   - "April 26"  → "2026-04-01" (2-digit year, pivoted at 50)
//   - "Juli"      → first day of July, year inferred via NowFunc
//
// Also supports a multi-month list where we take the earliest:
//
//   - "April/Mai 2026"        → "2026-04-01"
//   - "Juni/Juli/August 26"   → "2026-06-01"
//
// Returns ("", false) when no known German month is recognized.
func ParseGermanMonthYear(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", false
	}
	// Split into tokens, preserving "/" boundaries by replacing with space.
	flat := strings.ReplaceAll(s, "/", " ")
	tokens := strings.Fields(flat)
	if len(tokens) == 0 {
		return "", false
	}
	// First token must be a month.
	month, ok := GermanMonth(tokens[0])
	if !ok {
		return "", false
	}
	// Scan remaining tokens for a year (first numeric ≥ 1900 or 2-digit).
	year := 0
	for _, t := range tokens[1:] {
		t = strings.TrimPrefix(t, "'")
		if n, err := strconv.Atoi(t); err == nil {
			if n >= 1900 && n <= 9999 {
				year = n
				break
			}
			if n >= 0 && n < 100 {
				if n < 50 {
					year = 2000 + n
				} else {
					year = 1900 + n
				}
				break
			}
		}
	}
	if year == 0 {
		now := NowFunc()
		year = now.Year()
		if time.Month(month) < now.Month() {
			year++
		}
	}
	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), true
}
