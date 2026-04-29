// Package admin provides a single source of truth for the
// platform's admin email list. Each environment declares
// ADMIN_EMAILS once and every service that needs to know "is this
// email an operator?" reads it via this package.
//
// Used today by:
//   - falcon-signal — to fan-out admin alerts (email + push) and
//     run admin-only test triggers.
//   - falcon-api    — to bypass the magic-link throttle so an
//     operator pidiendo varios links durante un demo no se queda
//     sin acceso.
//
// Format of ADMIN_EMAILS:
//
//	ADMIN_EMAILS=helmer@hblabs.co, ops@hblabs.co
//
// Comma-separated; surrounding whitespace is trimmed; emails are
// lower-cased so the lookup is case-insensitive. Unset / empty →
// no admins, every IsAdmin returns false.
package admin

import (
	"strings"

	"hblabs.co/falcon/packages/environment"
)

// Config is the in-memory representation of ADMIN_EMAILS. Built
// once per process via Load() and treated as immutable thereafter
// — if you want to add an admin, set the env var and restart.
type Config struct {
	emails []string // canonical (lowercased, trimmed) — preserves declared order
	set    map[string]struct{}
}

// Load parses ADMIN_EMAILS from the environment and returns a
// fully built Config. Cheap; safe to call once at process start.
func Load() Config {
	raw := environment.ReadOptional("ADMIN_EMAILS", "")
	cfg := Config{set: make(map[string]struct{})}
	if raw == "" {
		return cfg
	}

	for _, part := range strings.Split(raw, ",") {
		email := strings.ToLower(strings.TrimSpace(part))
		if email == "" {
			continue
		}
		if _, dup := cfg.set[email]; dup {
			continue
		}
		cfg.set[email] = struct{}{}
		cfg.emails = append(cfg.emails, email)
	}
	return cfg
}

// IsAdmin reports whether the given email is in the admin list.
// Case-insensitive, whitespace-tolerant.
func (c Config) IsAdmin(email string) bool {
	_, ok := c.set[strings.ToLower(strings.TrimSpace(email))]
	return ok
}

// List returns the canonical admin emails in declared order.
func (c Config) List() []string { return c.emails }

// Empty reports whether no admin emails are configured.
func (c Config) Empty() bool { return len(c.emails) == 0 }
