package helpers

import "strings"

// Deprecated: Use from platfomkit instead.
func NormalizeText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
