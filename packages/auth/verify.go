package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/datasource"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// VerifyMagicLink handles `GET /auth/verify?token=<raw>`. Validates
// the magic-link token, marks it used (one-shot, except test
// tokens), finds-or-creates the user, issues a JWT, and stamps
// users.last_logged_in_at so reminder loops can tell apart "never
// signed in" from "signed in once but JWT TTL'd out".
func VerifyMagicLink(c *gin.Context) {
	rawToken := c.Query("token")
	if rawToken == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token required"})
		return
	}
	// Cap before hashing — issued tokens are exactly rawTokenLength
	// chars, so anything past maxRawTokenLength is malformed or an
	// attempt to make us SHA-256 a giant body for free (reports.md
	// rec #7).
	if len(rawToken) > maxRawTokenLength {
		c.JSON(http.StatusBadRequest, gin.H{"error": "token too long"})
		return
	}

	hash := tokenHash(rawToken)
	ctx := c.Request.Context()

	var magicToken models.Token
	err := system.GetStorage().GetByField(
		ctx,
		constants.MongoAuthTokensCollection,
		"token_hash", hash,
		&magicToken,
	)
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	case err != nil:
		// A real Mongo error (timeout, connection drop, etc.). Don't
		// respond 401 — that would make the user retry and burn the
		// 15-min TTL of their only valid magic-link without ever
		// seeing it succeed (reports.md B1).
		logrus.Errorf(logPrefix+" lookup magic token: %v", err)
		system.RespondInternal(c)
		return
	}

	// Regular magic links are single-use to block replay. Test
	// tokens issued by falcon-admin (`test: true`) stay multi-use
	// for their full 30-day TTL so an App Store reviewer can
	// uninstall / reinstall the app and log back in with the same
	// link.
	if magicToken.Used && !magicToken.Test {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token already used"})
		return
	}
	if time.Now().After(magicToken.ExpiresAt) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "token expired"})
		return
	}

	// Compare-and-swap: only the FIRST request to flip used=false →
	// true wins. The earlier `magicToken.Used` check is a fast-path
	// for already-used tokens; this CAS closes the race window
	// where two requests read used=false simultaneously and both
	// would otherwise issue a JWT for the same single-use link
	// (reports.md B2). Skipped for test tokens — those stay
	// multi-use by design.
	if !magicToken.Test {
		modified, err := system.GetStorage().UpdateOne(ctx,
			constants.MongoAuthTokensCollection,
			bson.M{"id": magicToken.ID, "used": false},
			bson.M{"$set": bson.M{"used": true, "updated_at": time.Now()}},
		)
		if err != nil {
			logrus.Errorf(logPrefix+" mark magic token used: %v", err)
			system.RespondInternal(c)
			return
		}
		if modified == 0 {
			// Lost the race — another verify request just flipped
			// used=true. Same response shape as the fast-path above.
			c.JSON(http.StatusUnauthorized, gin.H{"error": "token already used"})
			return
		}
	}

	userID, err := datasource.FindOrCreateUser(ctx, magicToken.Email)
	if err != nil {
		logrus.Errorf(logPrefix+" find/create user %s: %v", magicToken.Email, err)
		system.RespondInternal(c)
		return
	}

	magicToken.UserID = userID

	// Revoke previous JWTs for this user+device — one session per
	// device. Fail-closed: if revoke fails and we still issue a
	// fresh JWT, the old + new tokens coexist and break that
	// invariant (reports.md B9).
	if err := revokeDeviceJWTs(ctx, &magicToken); err != nil {
		logrus.Errorf(logPrefix+" revoke JWTs for user=%s device=%s: %v", magicToken.UserID, magicToken.DeviceID, err)
		system.RespondInternal(c)
		return
	}

	token, err := issueJWT(ctx, &magicToken)
	if err != nil {
		logrus.Errorf(logPrefix+" issue jwt for user %s: %v", userID, err)
		system.RespondInternal(c)
		return
	}

	// Stamp users.last_logged_in_at so the cv-reminder & login-
	// reminder loops can tell apart "never signed in" from "signed
	// in once but JWT TTL'd out". Best-effort: a transient Mongo
	// failure here just means the next reminder tick will check
	// the JWT fallback in the migration. Don't fail the verify on
	// this.
	if _, err := system.GetStorage().UpdateOne(ctx,
		constants.MongoUsersCollection,
		bson.M{"id": userID},
		bson.M{"$set": bson.M{"last_logged_in_at": time.Now(), "updated_at": time.Now()}},
	); err != nil {
		logrus.Warnf(logPrefix+" stamp last_logged_in_at for %s: %v", userID, err)
	}

	logrus.Infof(logPrefix+" user %s verified via magic link", magicToken.Email)
	c.JSON(http.StatusOK, gin.H{"token": token, "user_id": userID, "email": magicToken.Email})
}

