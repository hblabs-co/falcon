package freelancede

import (
	"errors"
	"testing"
)

// Real tokens captured from freelance.de. Both are expired — expiry is not
// what we are testing here; we are testing the rights-field validation only.
const (
	// tokenNullRights has rights: null — signals an invalid/unauthenticated session.
	tokenNullRights = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJmcmVlbGFuY2UiLCJleHAiOjE3NzUyMTUzMzMsImF1ZCI6ImZyZWVsYW5jZS5kZSIsInN1YiI6InNzcyIsInJpZ2h0cyI6bnVsbH0.zq7qYM72dxSARVKVpVkgGaqDg-PRrqx9j73Zub4OWD4"

	// tokenValidRights has a populated rights object — signals an authenticated session.
	tokenValidRights = "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9.eyJpc3MiOiJmcmVlbGFuY2UiLCJleHAiOjE3NzUxNzk2NDYsImF1ZCI6ImZyZWVsYW5jZS5kZSIsInN1YiI6MTExMTExLCJyaWdodHMiOnsicHJvZmlsZXMiOlsiYmxhY2tsaXN0IiwiYm9va21hcmsiLCJzaG93SG91cmx5UmF0ZSJdLCJwcm9maWxlcy5zZWFyY2guZmlsdGVycyI6WyJpbmNsdWRlRXhjbHVkZSIsImxhc3RVcGRhdGUiXSwicHJvZmlsZXMuc2VhcmNoIjpbImhpdEhpZ2hsaWdodCIsInByb2ZpU2VhcmNoIl0sInByb2plY3RzIjpbImJsYWNrbGlzdCIsImJvb2ttYXJrIiwic2hvd0NvbXBhbnlMb2dvIiwic2hvd0NvbXBhbnlOYW1lIl0sInByb2plY3RzLnNlYXJjaC5maWx0ZXJzIjpbImluY2x1ZGVFeGNsdWRlIiwicmVsYXRlZFRlcm1zIiwicmVtb3RlUHJlZmVyZW5jZSJdLCJwcm9qZWN0cy5zZWFyY2giOlsiaGl0SGlnaGxpZ2h0IiwicHJvZmlTZWFyY2giXX19.ujIUwy5LfKZwRoBdsmPzG_CO24zaVAUPrDirNpGBjW8"
)

func TestParseAccessToken(t *testing.T) {
	t.Run("null rights returns error", func(t *testing.T) {
		_, err := parseAccessToken(tokenNullRights)
		if !errors.Is(err, ErrNullRights) {
			t.Fatalf("expected ErrNullRights, got: %v", err)
		}
	})

	t.Run("valid rights parses successfully", func(t *testing.T) {
		token, err := parseAccessToken(tokenValidRights)
		if err != nil {
			t.Fatalf("unexpected error for token with valid rights: %v", err)
		}
		if token.Raw != tokenValidRights {
			t.Error("Raw token not preserved")
		}
		if token.ExpiresAt.IsZero() {
			t.Error("ExpiresAt should not be zero")
		}
		// Both tokens are expired — IsValid() must be false.
		if token.IsValid() {
			t.Error("expected IsValid() = false for an expired token")
		}
	})
}
