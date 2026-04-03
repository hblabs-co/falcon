package helpers

import "strings"

// normalizeText collapses surrounding whitespace and internal runs of
// whitespace/newlines into a single space.
// Used when comparing fields that the scraper stores with TrimSpace only
// (title, company, description).
func NormalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
