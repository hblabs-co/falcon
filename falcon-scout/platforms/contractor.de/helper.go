package contractorde

import (
	"strings"
	"time"
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

// parseStartDate normalizes a contractor.de start date (DD.MM.YYYY) to
// canonical YYYY-MM-DD. Returns the input unchanged if it cannot be parsed.
func parseStartDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse("02.01.2006", s); err == nil {
		return t.Format("2006-01-02")
	}
	return s
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
