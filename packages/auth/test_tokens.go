package auth

// Admin-scoped CRUD over the `auth_tokens` collection — the legacy
// `falcon-admin/users/tokens.go` lived under falcon-admin's HTTP
// surface, but every line here is auth-domain (mints `models.Token`,
// reads/writes `MongoAuthTokensCollection`, projects them for the UI).
// Centralised in `packages/auth` so the auth subsystem owns the
// definition of "what's a Token, who can mint one, how is it
// listed/deleted"; falcon-admin just mounts the Endpoints behind its
// bearer-protected group.

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/datasource"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// TestTokenTTL — long enough to cover a full Apple review cycle plus
// follow-up QA passes. Mongo's TTL index on `expires_at` auto-deletes
// them after expiry, so no cleanup cron is needed.
const TestTokenTTL = 30 * 24 * time.Hour

// ── Mint helper (shared with the legacy admin createTestLink) ────

// ValidationError wraps a `models.Token.Validate` failure so callers
// can map it to HTTP 400 instead of 500. Exported because the legacy
// `createTestLink` in falcon-admin still constructs a doc directly
// without the gin binding.
type ValidationError struct{ Err error }

func (v ValidationError) Error() string { return v.Err.Error() }

// MintTestLink builds a test Token document, persists it, and returns
// the magic-URL. Shared by both the legacy `POST /test-link` (admin)
// and `POST /users/:id/tokens` (user-scoped) — same shape, different
// metadata. The deep link is stored on the document (`Token.Link`)
// only for test tokens so the admin UI can copy any existing row's
// link without re-issuing it. The hash stays the source of truth for
// `/auth/verify`, so this dual storage is a UX add, not a security
// relaxation.
func MintTestLink(ctx context.Context, email, userID, deviceID string) (models.Token, string, error) {
	rawToken := gonanoid.Must(rawTokenLength)
	link := BuildMagicURL(rawToken)
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
		constants.MongoAuthTokensCollection, bson.M{"id": doc.ID}, doc); err != nil {
		return models.Token{}, "", err
	}
	return doc, link, nil
}

// ── Read helpers (used by the user-detail handler too) ──────────

// ListTestTokensFor pulls every test token tied to either the user_id
// (set on tokens minted via /users/:id/tokens) or the email (legacy
// tokens minted via /test-link before user_id was stamped). The OR
// makes "list tokens for this user" robust across both shapes.
func ListTestTokensFor(ctx context.Context, user *models.User) ([]models.Token, error) {
	var tokens []models.Token
	err := system.GetStorage().GetMany(ctx,
		constants.MongoAuthTokensCollection,
		userScopeFilter(user, bson.M{"test": true}),
		&tokens,
	)
	return tokens, err
}

// CountActiveTokens returns how many of the given tokens are still
// usable: not revoked, not used, not yet expired. Generic over both
// magic-link and JWT rows since the predicate is the same.
func CountActiveTokens(tokens []models.Token) int {
	now := time.Now()
	n := 0
	for _, t := range tokens {
		if !t.Revoked && !t.Used && t.ExpiresAt.After(now) {
			n++
		}
	}
	return n
}

// TokenView is the per-row representation used by the admin UI for
// both magic-link tokens and JWT sessions. Wraps `models.Token` with
// (a) `Expired` derived from expires_at vs now (so the client doesn't
// need its own clock skew handling) and (b) field-level `omitempty`
// so the UI can render row-by-row whatever the row carries. The raw
// token_hash is intentionally omitted — never leak it to the wire.
type TokenView struct {
	ID        string    `json:"id"`
	Type      string    `json:"type,omitempty"`
	Email     string    `json:"email,omitempty"`
	DeviceID  string    `json:"device_id,omitempty"`
	UserID    string    `json:"user_id,omitempty"`
	Platform  string    `json:"platform,omitempty"`
	Used      bool      `json:"used"`
	Revoked   bool      `json:"revoked"`
	Expired   bool      `json:"expired"`
	Test      bool      `json:"test,omitempty"`
	CreatedAt time.Time `json:"created_at,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// Link is the deep-link URL (`<scheme>://auth?token=<raw>`) for
	// test magic-link tokens. Absent on JWT sessions and on prod
	// magic links (those only persist the hash). The UI uses Link's
	// presence to decide whether the row is click-to-copy.
	Link string `json:"link,omitempty"`
}

// TokensToView projects every persisted field (minus token_hash)
// into the wire shape so the UI can show all metadata it has on a
// row. Used by both the magic-link list and the JWT-session list.
func TokensToView(tokens []models.Token) []TokenView {
	now := time.Now()
	out := make([]TokenView, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, TokenView{
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

// ── Handlers ─────────────────────────────────────────────────────

// listUserTokens — see ListUserTokensEndpoint.
func AdminListTokensByUserId(c *gin.Context) {
	listForUser(c, "tokens", ListTestTokensFor)
}

// createUserToken mints a magic link tied to the user's email and
// stamps user_id on the token document. Body is optional; only
// device_id is honoured (defaults to a random `test-…` placeholder).
func AdminCreateTokenForUserId(c *gin.Context) {
	user, ok := loadUserOr404(c)
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
		logrus.Errorf(logPrefix+" mint token for %s: %v", user.ID, err)
		system.RespondInternal(c)
		return
	}
	logrus.Infof(logPrefix+" created test link for user=%s email=%s id=%s",
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

func AdminDeleteTokensByUserId(c *gin.Context) {
	deleteForUser(c, "test tokens", bson.M{"test": true})
}

// AdminDeleteTokenById removes a single test token by its row id.
// Mounted on both the modern path (`DELETE /auth/tokens/:id`) and
// the legacy path (`DELETE /auth/test-link/:id`) — the legacy URL
// predates user_id stamping and stays for the original CLI flow,
// but the action is identical so a single handler serves both.
//
// Safety rail: scope to `test:true` so a stray id can't nuke a
// live user JWT.
func AdminDeleteTokenById(c *gin.Context) {
	id, ok := system.RequireParam(c, "id")
	if !ok {
		return
	}
	if err := system.GetStorage().DeleteMany(c.Request.Context(),
		constants.MongoAuthTokensCollection,
		bson.M{"id": id, "test": true}); err != nil {
		logrus.Errorf(logPrefix+" delete token %s: %v", id, err)
		system.RespondInternal(c)
		return
	}
	logrus.Infof(logPrefix+" revoked test token %s", id)
	c.Status(http.StatusNoContent)
}

// loadUserOr404 reads the user document by the `:id` URL param and
// writes a 404 response if missing. Returns (user, true) on success,
// (nil, false) when the caller should bail out. Thin gin wrapper
// over `datasource.FindUserByID` so the query primitive lives in one
// place (shared with falcon-admin's analog).
func loadUserOr404(c *gin.Context) (*models.User, bool) {
	id, ok := system.RequireParam(c, "id")
	if !ok {
		return nil, false
	}
	u, err := datasource.FindUserByID(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return nil, false
	}
	return &u, true
}

// ── Legacy global test-link handlers ─────────────────────────────

// createTestLink mints a long-lived multi-use magic token without
// a user_id binding. Body:
//
//	{ "email": "reviewer@apple.com", "device_id": "optional" }
//
// device_id is optional — if absent we mint a random `test-<nanoid>`
// placeholder so the doc's Validate() passes without tying the link
// to a specific phone. user_id is left empty here; the per-user
// flow (POST /users/:id/tokens) is the modern alternative that
// stamps it.
func AdminCreateTestLink(c *gin.Context) {
	var body struct {
		Email    string `json:"email" binding:"required,email"`
		DeviceID string `json:"device_id"`
	}
	if !system.BindJSONOrAbort(c, &body) {
		return
	}
	body.Email = datasource.NormalizeEmail(body.Email)
	if body.DeviceID == "" {
		body.DeviceID = "test-" + gonanoid.Must(12)
	}

	doc, link, err := MintTestLink(c.Request.Context(), body.Email, "", body.DeviceID)
	if err != nil {
		if vErr, ok := err.(ValidationError); ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": vErr.Error()})
			return
		}
		logrus.Errorf(logPrefix+" save test token: %v", err)
		system.RespondInternal(c)
		return
	}

	logrus.Infof(logPrefix+" created test link for %s (id=%s, expires=%s)",
		body.Email, doc.ID, doc.ExpiresAt.Format(time.RFC3339))
	c.JSON(http.StatusCreated, gin.H{
		"id":         doc.ID,
		"email":      doc.Email,
		"device_id":  doc.DeviceID,
		"link":       link,
		"expires_at": doc.ExpiresAt.Format(time.RFC3339),
	})
}

// listTestLinks returns every token with `test:true` across users.
// Hashes are never leaked — only metadata. Re-issuing a link means
// creating a new one. Kept for backward compat with the original
// CLI flow; the modern UI uses the per-user list endpoint instead.
func AdminListTestLinks(c *gin.Context) {
	var tokens []models.Token
	if err := system.GetStorage().GetMany(
		c.Request.Context(),
		constants.MongoAuthTokensCollection,
		bson.M{"test": true},
		&tokens,
	); err != nil {
		logrus.Errorf(logPrefix+" list test tokens: %v", err)
		system.RespondInternal(c)
		return
	}

	type view struct {
		ID        string    `json:"id"`
		Email     string    `json:"email"`
		DeviceID  string    `json:"device_id"`
		ExpiresAt time.Time `json:"expires_at"`
		CreatedAt time.Time `json:"created_at"`
		UserID    string    `json:"user_id,omitempty"`
	}
	out := make([]view, 0, len(tokens))
	for _, t := range tokens {
		out = append(out, view{t.ID, t.Email, t.DeviceID, t.ExpiresAt, t.CreatedAt, t.UserID})
	}
	c.JSON(http.StatusOK, gin.H{"count": len(out), "tokens": out})
}

// purgeAllTestLinks wipes every token with `test:true`. Use after
// App Store review wraps up so leftover long-lived links don't
// linger ~29 days before the TTL catches up.
func AdminPurgeTestLinks(c *gin.Context) {
	if err := system.GetStorage().DeleteMany(
		c.Request.Context(),
		constants.MongoAuthTokensCollection,
		bson.M{"test": true},
	); err != nil {
		logrus.Errorf(logPrefix+" purge test tokens: %v", err)
		system.RespondInternal(c)
		return
	}
	logrus.Infof(logPrefix + " purged all test tokens")
	c.Status(http.StatusNoContent)
}
