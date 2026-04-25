package platformkit

import (
	"strconv"
	"strings"
	"time"
)

// CanonicalImmediateStart is the single value scouts emit whenever the
// upstream expresses "start as soon as possible". Collapsing every
// variant ("ASAP", "Ab Sofort", "nächstmöglich", …) to this one string
// keeps downstream (DB, LLM, UI) dealing with a small closed set.
const CanonicalImmediateStart = "ab sofort"

// immediateStartKeywords lists every variant we treat as "start ASAP".
// Lookup is on `ToLower(TrimSpace(s))`, so case / padding don't matter.
// Union of the forms observed across scout platforms.
var immediateStartKeywords = map[string]struct{}{
	"asap":                          {},
	"asasp":                         {}, // common typo of "asap"
	"immediate":                     {},
	"immediately":                   {},
	"sofort":                        {},
	"ab sofort":                     {},
	"schnellstmöglich":              {},
	"schnellstmoeglich":             {},
	"nächstmöglich":                 {},
	"nächstmöglichst":               {},
	"naechstmoeglich":               {},
	"naechstmoeglichst":             {},
	"nächstmöglicher zeitpunkt":     {},
	"zum nächstmöglichen zeitpunkt": {},
}

// IsImmediateStart returns true when s (after trim + lowercase) matches
// a known "start ASAP" keyword.
func IsImmediateStart(s string) bool {
	_, ok := immediateStartKeywords[strings.ToLower(strings.TrimSpace(s))]
	return ok
}

// LastDayOfMonth returns the final calendar day of t's month. Implemented
// via "day 0 of next month", which Go's time package normalizes to the
// last day of the current month (28/29/30/31 handled, leap-year aware).
func LastDayOfMonth(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month()+1, 0, 0, 0, 0, 0, time.UTC)
}

// ParseEuropeanDate parses a DD<sep>MM<sep>YYYY string into canonical
// YYYY-MM-DD. sep is typically "." (German) or "/" (UK/ISO-slash).
//
// Handles three shapes in priority order:
//
//  1. strict zero-padded ("02<sep>01<sep>2006")
//  2. lenient single-digit ("2<sep>1<sep>2006") so "1.7.2026" works
//  3. calendar-invalid day → day clamped to last valid day of month
//     ("31.04.2026" → "2026-04-30", "30/02/2024" → "2024-02-29")
//
// Returns ("", false) when:
//   - the shape doesn't match (no recovery attempted)
//   - the month is outside 1..12 (we can't guess which month was meant)
//   - the day is < 1 (no natural "previous day" fallback)
//   - the year is outside 1900..9999
func ParseEuropeanDate(s, sep string) (string, bool) {
	if t, err := time.Parse("02"+sep+"01"+sep+"2006", s); err == nil {
		return t.Format("2006-01-02"), true
	}
	if t, err := time.Parse("2"+sep+"1"+sep+"2006", s); err == nil {
		return t.Format("2006-01-02"), true
	}
	parts := strings.Split(s, sep)
	if len(parts) != 3 {
		return "", false
	}
	day, errD := strconv.Atoi(parts[0])
	month, errM := strconv.Atoi(parts[1])
	year, errY := strconv.Atoi(parts[2])
	if errD != nil || errM != nil || errY != nil {
		return "", false
	}
	if month < 1 || month > 12 || year < 1900 || year > 9999 || day < 1 {
		return "", false
	}
	last := LastDayOfMonth(time.Date(year, time.Month(month), 1, 0, 0, 0, 0, time.UTC)).Day()
	if day > last {
		day = last
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Format("2006-01-02"), true
}

// ParseEuropeanDate2DigitYear parses DD<sep>MM<sep>YY where YY is a
// 2-digit year. The century is inferred: YY < 50 → 20YY, else 19YY.
// Delegates to ParseEuropeanDate for calendar validation (incl. clamp
// recovery). sep is typically "." or "/".
//
// Examples:
//   - "01.05.26" → "2026-05-01"
//   - "31.12.99" → "1999-12-31"
//   - "01/05/26" → "2026-05-01"
func ParseEuropeanDate2DigitYear(s, sep string) (string, bool) {
	parts := strings.Split(s, sep)
	if len(parts) != 3 {
		return "", false
	}
	if len(parts[2]) != 2 {
		return "", false
	}
	yy, err := strconv.Atoi(parts[2])
	if err != nil || yy < 0 {
		return "", false
	}
	// Pivot at 50: <50 → 20xx, ≥50 → 19xx.
	century := 2000
	if yy >= 50 {
		century = 1900
	}
	expanded := parts[0] + sep + parts[1] + sep + strconv.Itoa(century+yy)
	return ParseEuropeanDate(expanded, sep)
}

// ParseCompactDate parses 8 digits with no separators as DDMMYYYY.
// Common upstream typo where the input is entered without dots.
//
// Example:
//   - "20032026" → "2026-03-20"
func ParseCompactDate(s string) (string, bool) {
	s = strings.TrimSpace(s)
	if len(s) != 8 {
		return "", false
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return "", false
		}
	}
	expanded := s[:2] + "." + s[2:4] + "." + s[4:]
	return ParseEuropeanDate(expanded, ".")
}
