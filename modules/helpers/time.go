package helpers

import (
	"strings"
	"time"
)

// hasTimeComponent reports whether t carries any non-zero hour/min/sec/nanosecond.
func HasTimeComponent(t time.Time) bool {
	return t.Hour() != 0 || t.Minute() != 0 || t.Second() != 0 || t.Nanosecond() != 0
}

// sameDate reports whether a and b fall on the same UTC calendar day.
func SameDate(a, b time.Time) bool {
	a, b = a.UTC(), b.UTC()
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	return ay == by && am == bm && ad == bd
}

// parsePlatformTime parses any platform-supplied timestamp string into a UTC time.Time.
// Tries multiple common formats; returns the zero value if none match.
func ParsePlatformTime(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.999Z",
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02.01.2006", // German DD.MM.YYYY (defensive fallback)
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
