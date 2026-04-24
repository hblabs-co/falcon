package system

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"hblabs.co/falcon/common/ownhttp"
)

// bannerInnerWidth is the character width between the two │ bars —
// must match the number of ─ characters in the box edges. Keep in
// sync with the ╭...╮ and ╰...╯ strings below.
const bannerInnerWidth = 50

// PrintStartupBanner writes a small boxed banner to stdout listing the
// service name, the loopback URL, every reachable LAN URL, and any
// extra lines the caller passes (e.g. "watching: ...", "config: ...").
//
// Useful for dev servers and one-off CLIs where the operator wants
// to scan in two seconds "what is this and where do I open it". For
// production services the standard logrus output is enough — call
// this only when there's a human in the loop (local dev, debug
// commands, etc.).
//
// Example:
//
//	system.PrintStartupBanner("Falcon · Preview dev server", 8083,
//	    "watching: server, ../assets/screenshots")
func PrintStartupBannerAndPort(title string, port int, extras ...string) {
	fmt.Println()
	fmt.Println("  ╭──────────────────────────────────────────────────╮")
	fmt.Printf("  │%s│\n", centerInWidth(title, bannerInnerWidth))
	fmt.Println("  ╰──────────────────────────────────────────────────╯")
	// port <= 0 signals "this service has no HTTP listener" — e.g.
	// NATS-only consumers (scout, match-engine, normalizer, …).
	// Skip the URL lines so the banner doesn't lie about what's
	// reachable.
	if port > 0 {
		fmt.Printf("  ➜  Local:    http://localhost:%d\n", port)
		for _, ip := range ownhttp.LanIPs() {
			fmt.Printf("  ➜  Network:  http://%s:%d\n", ip, port)
		}
	}
	for _, line := range extras {
		fmt.Printf("  %s\n", line)
	}
	fmt.Println()
}

func PrintStartupBanner(title string, extras ...string) {
	PrintStartupBannerAndPort(title, 0, extras...)
}

// centerInWidth returns s padded with spaces on both sides so the
// result is exactly `width` characters wide. Uses rune counts so a
// title with emoji or accented chars doesn't miscount against the
// box width. When the title is wider than the box it passes through
// verbatim (the box expands visually via terminal wrapping — caller's
// problem if they pick a >50 char title).
func centerInWidth(s string, width int) string {
	n := utf8.RuneCountInString(s)
	if n >= width {
		return s
	}
	leftPad := (width - n) / 2
	rightPad := width - n - leftPad
	return strings.Repeat(" ", leftPad) + s + strings.Repeat(" ", rightPad)
}
