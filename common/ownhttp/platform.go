package ownhttp

import "strings"

// DetectPlatform extracts the client platform from a User-Agent string.
// Returns "ios", "android", or "web".
func DetectPlatform(userAgent string) string {
	ua := strings.ToLower(userAgent)
	switch {
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad") || strings.Contains(ua, "darwin"):
		return "ios"
	case strings.Contains(ua, "android"):
		return "android"
	default:
		return "web"
	}
}
