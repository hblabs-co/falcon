package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// jwtTTL is how long an issued JWT stays valid before the user has
// to re-verify a fresh magic-link. 30 days balances "annoying to
// log in" (long enough that occasional users don't notice) with
// blast radius if a token leaks (short enough that a stolen JWT
// becomes useless within a month). Combined with
// `revokeDeviceJWTs`, this also bounds session reuse from old
// devices.
const jwtTTL = 30 * 24 * time.Hour

// issueJWT signs a JWT, persists it in the tokens collection, and
// returns the signed string. The persisted row mirrors the magic-
// link's identity (user_id, device_id, platform, email) so the
// admin UI can list active sessions per user.
func issueJWT(ctx context.Context, magic *models.Token) (string, error) {
	secret := system.MustEnv("JWT_SECRET")
	tokenID := gonanoid.Must()
	now := time.Now()
	exp := now.Add(jwtTTL)

	claims := jwt.MapClaims{
		"jti":   tokenID,
		"sub":   magic.UserID,
		"email": magic.Email,
		"exp":   exp.Unix(),
		"iat":   now.Unix(),
	}
	if magic.DeviceID != "" {
		claims["device_id"] = magic.DeviceID
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	doc := models.Token{
		ID:        tokenID,
		Type:      models.TokenTypeJWT,
		UserID:    magic.UserID,
		DeviceID:  magic.DeviceID,
		Platform:  magic.Platform,
		Email:     magic.Email,
		TokenHash: tokenHash(signed),
		ExpiresAt: exp,
		CreatedAt: now,
	}
	// Persist must succeed before we return the JWT to the caller.
	// If it fails and we still hand out the token, the client ends
	// up with a cryptographically valid JWT that has no row in
	// `tokens` — invisible to the admin sessions UI and impossible
	// to revoke via revokeDeviceJWTs. Better to error out and let
	// the user retry the verify (the magic-link is single-use, so
	// they'd request a fresh one).
	if err := system.GetStorage().Set(ctx, constants.MongoAuthTokensCollection, bson.M{"id": tokenID}, doc); err != nil {
		return "", fmt.Errorf("persist jwt token: %w", err)
	}

	return signed, nil
}

// revokeDeviceJWTs deletes previous JWT tokens for a user+device
// combo. Allows multiple sessions on different devices, but only
// one per device.
//
// Returns the underlying Mongo error so the caller can abort the
// verify flow: handing out a fresh JWT after a failed revoke
// breaks the "one session per device" invariant — the new and old
// tokens would coexist (reports.md B9). DeviceID == "" is a no-op
// (no device to scope by) and returns nil.
func revokeDeviceJWTs(ctx context.Context, magic *models.Token) error {
	if magic.DeviceID == "" {
		return nil
	}
	return system.GetStorage().DeleteMany(ctx, constants.MongoAuthTokensCollection, bson.M{
		"user_id":   magic.UserID,
		"device_id": magic.DeviceID,
		"type":      models.TokenTypeJWT,
	})
}

// tokenHash returns the hex-encoded SHA-256 of the raw token.
// Used for both magic-link rawToken and JWT signed string so the
// `tokens` collection never persists the plaintext credential.
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}
