package users

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

const (
	// TestTokenTTL — long enough to cover a full Apple review cycle
	// plus follow-up QA passes. Mongo's TTL index on `expires_at`
	// auto-deletes them after expiry, so no cleanup cron needed.
	TestTokenTTL = 30 * 24 * time.Hour

	// defaultAppScheme is the iOS app's URL scheme as declared in
	// Info.plist, used when APP_SCHEME isn't set.
	defaultAppScheme = "falcon"
)

// ── Per-user magic-link CRUD ──────────────────────────────────────

func listUserTokens(c *gin.Context) {
	id := c.Param("id")
	user, ok := loadUserOr404(c, id)
	if !ok {
		return
	}
	tokens, err := listTestTokensFor(c.Request.Context(), user)
	if err != nil {
		respondInternal(c, "list tokens", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(tokens), "tokens": tokensToView(tokens)})
}

// createUserToken mints a magic link tied to the user's email and
// stamps user_id on the token document. Body is optional; only
// device_id is honoured (defaults to a random `test-…` placeholder).
func createUserToken(c *gin.Context) {
	id := c.Param("id")
	user, ok := loadUserOr404(c, id)
	if !ok {
		return
	}
	var body struct {
		DeviceID string `json:"device_id"`
	}
	_ = c.ShouldBindJSON(&body)
	if body.DeviceID == "" {
		body.DeviceID = "test-" + gonanoid.Must(12)
	}
	doc, link, err := MintTestLink(c.Request.Context(), user.Email, user.ID, body.DeviceID)
	if err != nil {
		respondInternal(c, "mint token", err)
		return
	}
	logrus.Infof("[admin] created test link for user=%s email=%s id=%s",
		user.ID, user.Email, doc.ID)
	c.JSON(http.StatusCreated, gin.H{
		"id":         doc.ID,
		"email":      doc.Email,
		"device_id":  doc.DeviceID,
		"user_id":    doc.UserID,
		"link":       link,
		"expires_at": doc.ExpiresAt.Format(time.RFC3339),
	})
}

func deleteUserTokens(c *gin.Context) {
	id := c.Param("id")
	user, ok := loadUserOr404(c, id)
	if !ok {
		return
	}
	filter := bson.M{
		"test": true,
		"$or": []bson.M{
			{"user_id": id},
			{"email": user.Email},
		},
	}
	if err := system.GetStorage().DeleteMany(c.Request.Context(), constants.MongoTokensCollection, filter); err != nil {
		respondInternal(c, "delete user tokens", err)
		return
	}
	logrus.Infof("[admin] revoked all test tokens for user=%s", id)
	c.Status(http.StatusNoContent)
}

func deleteToken(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	// Safety rail: scope to test:true so a stray id can't nuke a
	// live user JWT.
	if err := system.GetStorage().DeleteMany(c.Request.Context(), constants.MongoTokensCollection,
		bson.M{"id": id, "test": true}); err != nil {
		respondInternal(c, "delete token", err)
		return
	}
	logrus.Infof("[admin] revoked test token %s", id)
	c.Status(http.StatusNoContent)
}

// ── Mint helper (shared with legacy /test-link in service.go) ─────

// ValidationError wraps a Token.Validate failure so callers can map
// it to HTTP 400 instead of 500. Exported for the legacy
// createTestLink in the admin package.
type ValidationError struct{ Err error }

func (v ValidationError) Error() string { return v.Err.Error() }

// MintTestLink builds the Token document, persists it, and returns
// the magic-URL the client should hand to the user. Shared by the
// legacy POST /test-link and the user-scoped POST /users/:id/tokens
// — both want the same shape, just different metadata.
//
// The deep link is stored on the document (Token.Link) — only for
// test tokens — so the admin UI can let an operator copy any
// existing row's link without having to re-issue it. The hash
// stays the source of truth for /auth/verify, so this dual storage
// is a UX add, not a security relaxation.
func MintTestLink(ctx context.Context, email, userID, deviceID string) (models.Token, string, error) {
	rawToken := gonanoid.Must(32)
	scheme := os.Getenv("APP_SCHEME")
	if scheme == "" {
		scheme = defaultAppScheme
	}
	link := scheme + "://auth?token=" + rawToken
	now := time.Now()
	doc := models.Token{
		ID:        gonanoid.Must(),
		Type:      models.TokenTypeMagicLink,
		Email:     email,
		UserID:    userID,
		DeviceID:  deviceID,
		Platform:  "ios",
		TokenHash: tokenHash(rawToken),
		ExpiresAt: now.Add(TestTokenTTL),
		Used:      false,
		Test:      true,
		CreatedAt: now,
		Link:      link,
	}
	if err := doc.Validate(); err != nil {
		return models.Token{}, "", ValidationError{Err: err}
	}
	if err := system.GetStorage().Set(ctx,
		constants.MongoTokensCollection, bson.M{"id": doc.ID}, doc); err != nil {
		return models.Token{}, "", err
	}
	return doc, link, nil
}

func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}

// ── Read helpers (used by detail.go too) ──────────────────────────

// listTestTokensFor pulls every test token tied to either the
// user_id (set on tokens minted via /users/:id/tokens) or the email
// (legacy tokens minted via /test-link before user_id was stamped).
// The OR is what makes "list tokens for this user" robust across
// both shapes.
func listTestTokensFor(ctx context.Context, user *models.User) ([]models.Token, error) {
	filter := bson.M{
		"test": true,
		"$or": []bson.M{
			{"user_id": user.ID},
			{"email": user.Email},
		},
	}
	var tokens []models.Token
	err := system.GetStorage().GetMany(ctx, constants.MongoTokensCollection, filter, &tokens)
	return tokens, err
}

// tokensToView projects every persisted field (minus token_hash)
// into the wire shape so the UI can show all metadata it has on a
// row. Empty fields ride through to JSON with omitempty so the
// browser can decide row-by-row what to render.
//
// Link is included only when present (test tokens) — JWT sessions
// don't have one, and the UI uses its presence to decide whether
// the row should be click-to-copy.
func tokensToView(tokens []models.Token) []tokenView {
	now := time.Now()
	out := make([]tokenView, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, tokenView{
			ID:        t.ID,
			Type:      string(t.Type),
			Email:     t.Email,
			DeviceID:  t.DeviceID,
			UserID:    t.UserID,
			Platform:  t.Platform,
			Used:      t.Used,
			Revoked:   t.Revoked,
			Expired:   t.ExpiresAt.Before(now),
			Test:      t.Test,
			CreatedAt: t.CreatedAt,
			ExpiresAt: t.ExpiresAt,
			Link:      t.Link,
		})
	}
	return out
}

func countActive(tokens []models.Token) int {
	now := time.Now()
	n := 0
	for _, t := range tokens {
		if !t.Revoked && !t.Used && t.ExpiresAt.After(now) {
			n++
		}
	}
	return n
}
