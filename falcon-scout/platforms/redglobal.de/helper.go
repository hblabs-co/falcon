package redglobalde

import (
	"strings"
	"time"
)

// referenceLabels holds the normalized strings that identify the reference ID
// row on a redglobal detail page. The site is German so the label may be in
// either English or German depending on locale and future redesigns.
var referenceLabels = map[string]bool{
	"reference": true,
	"referenz":  true,
	"ref":       true,
}

// normalizeReferenceLabel cleans up a label string from the detail page so it can
// be compared against referenceLabels regardless of casing, trailing colon, or
// surrounding whitespace.
func normalizeReferenceLabel(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, ":")
	s = strings.TrimSpace(s)
	return strings.ToLower(s)
}

// isReferenceLabel reports whether s (after normalization) is recognized as a
// reference ID label. Pass through normalizeReferenceLabel before calling.
func isReferenceLabel(s string) bool {
	return referenceLabels[s]
}

// parseCandidateDate parses a redglobal listing date (already normalized to YYYY-MM-DD
// by parseListingDate in scraper.go) into a UTC time.Time. Returns the zero value if it
// cannot be parsed.
func parseCandidateDate(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

// parseListingDate normalizes a redglobal listing date to canonical YYYY-MM-DD.
// The listing uses German DD.MM.YYYY format (e.g. "19.03.2026"); JSON-LD detail
// pages use ISO YYYY-MM-DD. Normalizing the listing aligns both, so the filter
// can compare apples to apples and MongoDB stores ordered, indexable dates.
// Returns the input unchanged if it cannot be parsed.
func parseListingDate(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if t, err := time.Parse("02.01.2006", s); err == nil {
		return t.Format("2006-01-02")
	}
	return s
}

// parseJobHref extracts platformID and slug from a path like /jobs/job/<slug>/<platformID>.
// The platformID for the ProjectCandidate of redglobal ist the cms id. not the real projectId
// the real projectId is referenced as ReferenceId in the Projects collection
func (s *Scraper) parseJobHref(href string) (platformID, slug string) {
	// Trim trailing slash
	href = strings.TrimSuffix(href, "/")
	parts := strings.Split(href, "/")
	// Expected: ["", "jobs", "job", "<slug>", "<platformID>"]
	if len(parts) < 5 {
		return "", ""
	}
	return parts[len(parts)-1], parts[len(parts)-2]
}
