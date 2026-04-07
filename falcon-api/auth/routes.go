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
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
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
}

// handleMagic godoc
// POST /auth/magic
// Body: { "email": "user@example.com" }
// Generates a single-use magic link token and fires a signal.magic_link event
// so that falcon-signal delivers the email.
func handleMagic(c *gin.Context) {
	var body struct {
		Email string `json:"email" binding:"required,email"`
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
		TokenHash: hash,
		ExpiresAt: now.Add(magicTokenTTL),
		Used:      false,
		CreatedAt: now,
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

	magicURL := helpers.ReadEnvOptional("APP_SCHEME", "falcon") + "://auth?token=" + rawToken

	evt := models.MagicLinkRequestedEvent{
		Email:     body.Email,
		MagicLink: magicURL,
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

	if magic.Used {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token already used"})
		return
	}
	if time.Now().After(magic.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
		return
	}

	// Mark used immediately to prevent replay.
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

	userID, err := findOrCreateUser(ctx, magic.Email)
	if err != nil {
		logrus.Errorf("find/create user %s: %v", magic.Email, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal error"})
		return
	}

	token, err := issueJWT(ctx, userID, magic.Email)
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
func issueJWT(ctx context.Context, userID, email string) (string, error) {
	secret := system.MustEnv("JWT_SECRET")
	tokenID := gonanoid.Must()
	now := time.Now()
	exp := now.Add(jwtTTL)

	claims := jwt.MapClaims{
		"jti":   tokenID,
		"sub":   userID,
		"email": email,
		"exp":   exp.Unix(),
		"iat":   now.Unix(),
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	// Persist so we can revoke later.
	doc := models.Token{
		ID:        tokenID,
		Type:      models.TokenTypeJWT,
		UserID:    userID,
		Email:     email,
		TokenHash: tokenHash(signed),
		ExpiresAt: exp,
		CreatedAt: now,
	}
	if err := system.GetStorage().Set(ctx, constants.MongoTokensCollection, bson.M{"id": tokenID}, doc); err != nil {
		logrus.Warnf("persist jwt token: %v", err)
		// Non-fatal — JWT still works, just can't be revoked via DB.
	}

	return signed, nil
}

// tokenHash returns the hex-encoded SHA-256 of the raw token.
func tokenHash(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return fmt.Sprintf("%x", sum)
}
