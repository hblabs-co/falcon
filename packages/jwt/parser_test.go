package jwt

import (
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

var testSecret = []byte("test-secret")

// sign creates a signed HS256 token with the given claims for use in tests.
func sign(t *testing.T, claims gojwt.Claims) string {
	t.Helper()
	raw, err := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims).SignedString(testSecret)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return raw
}

func TestParseUnverified(t *testing.T) {
	parser := NewParser()

	t.Run("valid token with future expiry", func(t *testing.T) {
		exp := time.Now().Add(time.Hour)
		raw := sign(t, gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(exp),
		})

		token, err := parser.ParseUnverified(raw, &gojwt.RegisteredClaims{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !token.ExpiresAt.Equal(exp.Truncate(time.Second)) && !token.ExpiresAt.Round(time.Second).Equal(exp.Round(time.Second)) {
			t.Errorf("ExpiresAt = %v, want ~%v", token.ExpiresAt, exp)
		}
		if token.Raw != raw {
			t.Errorf("Raw not preserved")
		}
	})

	t.Run("expired token still parses (no claim validation in ParseUnverified)", func(t *testing.T) {
		exp := time.Now().Add(-time.Hour)
		raw := sign(t, gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(exp),
		})

		token, err := parser.ParseUnverified(raw, &gojwt.RegisteredClaims{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token.IsValid() {
			t.Error("expected IsValid() = false for expired token")
		}
	})

	t.Run("token expiring within buffer is parsed but IsValid returns false", func(t *testing.T) {
		exp := time.Now().Add(30 * time.Second)
		raw := sign(t, gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(exp),
		})

		token, err := parser.ParseUnverified(raw, &gojwt.RegisteredClaims{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token.IsValid() {
			t.Error("expected IsValid() = false for token within buffer window")
		}
	})

	t.Run("malformed token string", func(t *testing.T) {
		_, err := parser.ParseUnverified("not.a.jwt", &gojwt.RegisteredClaims{})
		if err == nil {
			t.Fatal("expected error for malformed token")
		}
	})

	t.Run("missing exp claim", func(t *testing.T) {
		// RegisteredClaims with no ExpiresAt set
		raw := sign(t, gojwt.RegisteredClaims{
			Issuer: "test",
		})

		_, err := parser.ParseUnverified(raw, &gojwt.RegisteredClaims{})
		if err == nil {
			t.Fatal("expected error when exp claim is missing")
		}
	})
}
