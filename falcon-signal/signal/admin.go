package signal

import (
	"strings"

	"hblabs.co/falcon/common/helpers"
)

// AdminConfig holds the set of email addresses that should receive operational
// alerts (markup drift, infrastructure failures, anything emitted as a high or
// critical AdminAlertEvent). Loaded once at startup from the ADMIN_EMAILS env
// var, which accepts a comma-separated list:
//
//	ADMIN_EMAILS=helmer@hblabs.co, ops@hblabs.co
//
// Emails are normalized to lowercase + trimmed so the lookup is case-insensitive.
// An empty or unset variable yields a config with no admins; the AdminNotifier
// then becomes a no-op.
type AdminConfig struct {
	emails []string // canonical (lowercased, trimmed) — preserves declared order
	set    map[string]struct{}
}

// LoadAdminConfig parses ADMIN_EMAILS from the environment.
func LoadAdminConfig() AdminConfig {
	raw := helpers.ReadEnvOptional("ADMIN_EMAILS", "")
	cfg := AdminConfig{set: make(map[string]struct{})}
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

// IsAdmin reports whether the given email is in the admin list. Case-insensitive.
func (c AdminConfig) IsAdmin(email string) bool {
	_, ok := c.set[strings.ToLower(strings.TrimSpace(email))]
	return ok
}

// List returns the canonical admin emails in declared order.
func (c AdminConfig) List() []string { return c.emails }

// Empty reports whether no admin emails are configured.
func (c AdminConfig) Empty() bool { return len(c.emails) == 0 }
