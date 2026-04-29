package auth

// Admin-scoped CRUD over JWT session rows in `auth_tokens`. Sessions
// and magic-link tokens share the collection but differ on `type`
// and `test`: JWTs are `type=jwt, test=false`. Listing/revoking is
// a separate path from the magic-link CRUD (test_tokens.go) so a
// stray click in the magic-links UI can't accidentally kill live
// mobile sessions and vice versa.

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// ── Read helper (used by user-detail handler too) ───────────────

// ListSessionsFor pulls every JWT session tied to either the user_id
// (set on tokens issued via /auth/verify) or the email (legacy rows
// minted before user_id was stamped). Mirrors `ListTestTokensFor`
// in test_tokens.go but scoped to type=jwt.
func ListSessionsFor(ctx context.Context, user *models.User) ([]models.Token, error) {
	var sessions []models.Token
	err := system.GetStorage().GetMany(ctx,
		constants.MongoAuthTokensCollection,
		userScopeFilter(user, bson.M{"type": models.TokenTypeJWT}),
		&sessions,
	)
	return sessions, err
}

// ── Handlers ─────────────────────────────────────────────────────

func AdminListSessionsByUserId(c *gin.Context) {
	listForUser(c, "sessions", ListSessionsFor)
}

func AdminDeleteSessionById(c *gin.Context) {
	id, ok := system.RequireParam(c, "id")
	if !ok {
		return
	}
	// Scope to type=jwt so a stray id can't nuke a magic-link token.
	if err := system.GetStorage().DeleteMany(c.Request.Context(),
		constants.MongoAuthTokensCollection,
		bson.M{"id": id, "type": models.TokenTypeJWT}); err != nil {
		logrus.Errorf(logPrefix+" delete session %s: %v", id, err)
		system.RespondInternal(c)
		return
	}
	logrus.Infof(logPrefix+" revoked session %s", id)
	c.Status(http.StatusNoContent)
}

func AdminDeleteSessionsByUserId(c *gin.Context) {
	deleteForUser(c, "sessions", bson.M{"type": models.TokenTypeJWT})
}
