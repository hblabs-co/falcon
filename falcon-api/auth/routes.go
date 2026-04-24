package auth

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	environment "hblabs.co/falcon/common/environment"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/ownhttp"
	"hblabs.co/falcon/common/system"
)

const (
	magicTokenTTL = 15 * time.Minute
	jwtTTL        = 30 * 24 * time.Hour
)

// Routes implements server.RouteGroup for auth endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	g := r.Group("/auth")
	g.POST("/magic", handleMagic)
	g.GET("/verify", handleVerify)

	// TTL index: MongoDB auto-deletes expired tokens.
	if err := system.GetStorage().EnsureTTLIndex(system.Ctx(), constants.MongoTokensCollection, "expires_at"); err != nil {
		logrus.Fatalf("ensure tokens TTL index: %v", err)
	}
}

// handleMagic godoc
// POST /auth/magic
// Body: { "email": "user@example.com" }
// Generates a single-use magic link token and fires a signal.magic_link event
// so that falcon-signal delivers the email.
func handleMagic(c *gin.Context) {
	var body struct {
		Email    string `json:"email"     binding:"required,email"`
		DeviceID string `json:"device_id"`
		Platform string `json:"platform"  binding:"required,oneof=ios"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	rawToken := gonanoid.Must(32)
	hash := tokenHash(rawToken)
	now := time.Now()

	doc := models.Token{
		ID:        gonanoid.Must(),
		Type:      models.TokenTypeMagicLink,
		Email:     body.Email,
		DeviceID:  body.DeviceID,
		Platform:  body.Platform,
		TokenHash: hash,
		ExpiresAt: now.Add(magicTokenTTL),
		Used:      false,
		CreatedAt: now,
	}

	if err := doc.Validate(); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	if err := system.GetStorage().Set(
		ctx,
		constants.MongoTokensCollection,
		bson.M{"id": doc.ID},
		doc,
	); err != nil {
		logrus.Errorf("save magic token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create magic link"})
		return
	}

	magicURL := environment.ReadOptional("APP_SCHEME", "falcon") + "://auth?token=" + rawToken

	evt := models.MagicLinkRequestedEvent{
		Email:     body.Email,
		MagicLink: magicURL,
		Platform:  ownhttp.DetectPlatform(c.GetHeader("User-Agent")),
	}
	if err := system.Publish(ctx, constants.SubjectSignalMagicLink, evt); err != nil {
		logrus.Errorf("publish signal.magic_link: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not send magic link"})
		return
	}

	logrus.Infof("[auth] magic link sent to %s", body.Email)
	c.JSON(http.StatusAccepted, gin.H{"message": "magic link sent"})
}

// handleVerify godoc
// GET /auth/verify?token=<raw>
// Validates the token, marks it used, finds-or-creates the user, and returns a JWT.
func handleVerify(c *gin.Context) {
	rawToken := c.Query("token")
	if rawToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}

	hash := tokenHash(rawToken)
	ctx := c.Request.Context()

	var magic models.Token
	if err := system.GetStorage().GetByField(
		ctx,
		constants.MongoTokensCollection,
		"token_hash", hash,
		&magic,
	); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	// Regular magic links are single-use to block replay. Test tokens
	// issued by falcon-authorizer (`test: true`) stay multi-use for
	// their full 30-day TTL so an App Store reviewer can uninstall /
	// reinstall the app and log back in with the same link.
	if magic.Used && !magic.Test {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token already used"})
		return
	}
	if time.Now().After(magic.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
		return
	}

	// Mark used immediately to prevent replay — skipped for test tokens
	// (handled above).
	if !magic.Test {
		if err := system.GetStorage().SetById(
			ctx,
			constants.MongoTokensCollection,
			magic.ID,
			bson.M{"used": true, "updated_at": time.Now()},
		); err != nil {
			logrus.Errorf("mark magic token used: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
			return
		}
	}

	userID, err := findOrCreateUser(ctx, magic.Email)
	if err != nil {
		logrus.Errorf("find/create user %s: %v", magic.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	magic.UserID = userID

	// Revoke previous JWTs for this user+device — one session per device.
	revokeDeviceJWTs(ctx, &magic)

	token, err := issueJWT(ctx, &magic)
	if err != nil {
		logrus.Errorf("issue jwt for user %s: %v", userID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	logrus.Infof("[auth] user %s verified via magic link", magic.Email)
	c.JSON(http.StatusOK, gin.H{"token": token, "user_id": userID, "email": magic.Email})
}

// findOrCreateUser returns the existing user ID for the given email, or creates a new user.
func findOrCreateUser(ctx context.Context, email string) (string, error) {
	var user models.User
	err := system.GetStorage().GetByField(ctx, constants.MongoUsersCollection, "email", email, &user)
	if err == nil {
		return user.ID, nil
	}

	now := time.Now()
	user = models.User{
		ID:        gonanoid.Must(),
		Email:     email,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := system.GetStorage().Set(
		ctx,
		constants.MongoUsersCollection,
		bson.M{"email": email},
		user,
	); err != nil {
		return "", err
	}
	return user.ID, nil
}

// issueJWT signs a JWT, persists it in the tokens collection, and returns the signed string.
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
	if err := system.GetStorage().Set(ctx, constants.MongoTokensCollection, bson.M{"id": tokenID}, doc); err != nil {
		logrus.Warnf("persist jwt token: %v", err)
	}

	return signed, nil
}

// revokeDeviceJWTs deletes previous JWT tokens for a user+device combo.
// Allows multiple sessions on different devices, but only one per device.
func revokeDeviceJWTs(ctx context.Context, magic *models.Token) {
	if magic.DeviceID == "" {
		return
	}
	if err := system.GetStorage().DeleteMany(ctx, constants.MongoTokensCollection, bson.M{
		"user_id":   magic.UserID,
		"device_id": magic.DeviceID,
		"type":      models.TokenTypeJWT,
	}); err != nil {
		logrus.Warnf("[auth] revoke JWTs for user=%s device=%s: %v", magic.UserID, magic.DeviceID, err)
	}
}

// tokenHash returns the hex-encoded SHA-256 of the raw token.
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}
