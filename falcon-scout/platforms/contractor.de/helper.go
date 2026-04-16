package contractorde

import (
	"strings"
	"time"

	"hblabs.co/falcon/modules/platformkit"
)

// parseProjectID extracts the numeric ID from text like "Projekt-ID 30290".
func parseProjectID(text string) string {
	text = strings.TrimSpace(text)
	const prefix = "Projekt-ID"
	if !strings.HasPrefix(text, prefix) {
		return ""
	}
	return strings.TrimSpace(text[len(prefix):])
}

// parseStartDate normalizes a contractor.de start-date string into one
// of the closed canonical shapes shared across falcon-scout platforms:
//
//   - ""             — empty or unparseable (e.g. "nach Absprache")
//   - "ab sofort"    — ASAP / "Ab Sofort" / etc.
//   - "YYYY-MM-DD"   — calendar date
//
// Handles every pattern observed in the real feed:
//   - "2026-07-01"    (already canonical ISO)
//   - "01.05.2026"    (DD.MM.YYYY)
//   - "asap" / "Ab sofort"
//   - "April"         (month alone → first day, year inferred)
//   - "April 2026"    (month + year → first day)
//   - "Anfang April"  → day 1
//   - "Mitte April"   → day 15
//   - "Ende August"   → last day of August (year inferred)
//   - "nach Absprache"/other vague text → ""
func parseStartDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if platformkit.IsImmediateStart(s) {
		return platformkit.CanonicalImmediateStart
	}
	// Primary format in the feed: DD.MM.YYYY.
	if out, ok := platformkit.ParseEuropeanDate(s, "."); ok {
		return out
	}
	// Already canonical YYYY-MM-DD.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.Format("2006-01-02")
	}
	// "Anfang April" / "Mitte April" / "Ende August".
	if out, ok := platformkit.ParseGermanMonthPhrase(s); ok {
		return out
	}
	// "April", "Mai 2026", "April/Mai 2026".
	if out, ok := platformkit.ParseGermanMonthYear(s); ok {
		return out
	}
	return ""
}

// parseLocation extracts the remote flag from text that may contain "Remote"
// (e.g. "Freiburg/Remote", "Remote/Düsseldorf", "Remote"). The location
// string is preserved as-is — no truncation — so the original value from the
// HTML is kept for display and for the LLM normalizer to work with.
func parseLocation(text string) (location string, remote bool) {
	text = strings.TrimSpace(text)
	if strings.Contains(strings.ToLower(text), "remote") {
		remote = true
	}
	location = text
	return
}

// parseCandidateDate parses a contractor.de listing date (already normalized
// to YYYY-MM-DD by parseStartDate) into a UTC time.Time.
func parseCandidateDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}
