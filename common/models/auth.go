package models

import (
	"fmt"
	"time"
)

// TokenType identifies the purpose of a persisted token document.
type TokenType string

const (
	TokenTypeMagicLink TokenType = "magic_link"
	TokenTypeJWT       TokenType = "jwt"
)

// Token is the MongoDB document for any auth token (magic link or JWT session).
type Token struct {
	ID        string    `json:"id"         bson:"id"`
	Type      TokenType `json:"type"       bson:"type"`
	UserID    string    `json:"user_id,omitempty"    bson:"user_id,omitempty"`
	DeviceID  string    `json:"device_id,omitempty"  bson:"device_id,omitempty"`
	Platform  string    `json:"platform"             bson:"platform"`
	Email     string    `json:"email"      bson:"email"`
	TokenHash string    `json:"token_hash" bson:"token_hash"`
	ExpiresAt time.Time `json:"expires_at" bson:"expires_at"`
	Used      bool      `json:"used"       bson:"used"`
	Revoked   bool      `json:"revoked"    bson:"revoked"`
	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	// Test is true for tokens created by falcon-authorizer — long-lived
	// (30 days), multi-use magic links for App Store reviewers or QA.
	// falcon-api's /auth/verify skips the "used" marker when this is
	// set, so the same link survives a reinstall + relogin cycle.
	// Purge all test tokens at once with DELETE /test-links on the
	// authorizer.
	Test bool `json:"test,omitempty" bson:"test,omitempty"`
}

// SupportedPlatforms lists the platforms that can request auth tokens.
var SupportedPlatforms = map[string]bool{
	"ios": true,
}

// Validate checks that the platform is supported and device_id is present when required.
func (t *Token) Validate() error {
	if !SupportedPlatforms[t.Platform] {
		return fmt.Errorf("unsupported platform %q", t.Platform)
	}
	if (t.Platform == "ios" || t.Platform == "android") && t.DeviceID == "" {
		return fmt.Errorf("device_id is required for platform %q", t.Platform)
	}
	return nil
}
