package freelancede

import (
	"strconv"
	"strings"

	"hblabs.co/falcon/common/constants"
)

// parseRate parses a raw rate string scraped from freelance.de into a structured Rate.
// Strings that don't match the expected pattern (e.g. "auf Anfrage") are returned
// with only Raw set.
func parseRate(raw string) *ProjectRate {
	r := ProjectRate{Raw: raw}
	m := rateRe.FindStringSubmatch(strings.TrimSpace(raw))
	if m == nil {
		return &r
	}
	// German number format: "." is thousands separator, "," is decimal separator.
	numStr := strings.ReplaceAll(m[1], ".", "")
	numStr = strings.ReplaceAll(numStr, ",", ".")
	if v, err := strconv.ParseFloat(numStr, 64); err == nil {
		r.Amount = &v
	}
	switch m[2] {
	case "€":
		r.Currency = "EUR"
	case "$":
		r.Currency = "USD"
	default:
		r.Currency = strings.ToUpper(m[2])
	}
	switch strings.ToLower(m[3]) {
	case "tagessatz":
		r.Type = constants.RateTypeDaily
	case "stundensatz":
		r.Type = constants.RateTypeHourly
	}
	return &r
}
