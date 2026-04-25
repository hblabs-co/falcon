package users

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

// Sessions and magic-link tokens share the `tokens` collection but
// differ on `type` and `test`: JWTs are `type=jwt, test=false`.
// Listing/revoking is a separate path so a stray click in the
// magic-links UI can't accidentally kill live mobile sessions and
// vice versa.

func listUserSessions(c *gin.Context) {
	id := c.Param("id")
	user, ok := loadUserOr404(c, id)
	if !ok {
		return
	}
	sessions, err := listSessionsFor(c.Request.Context(), user)
	if err != nil {
		respondInternal(c, "list sessions", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(sessions), "sessions": tokensToView(sessions)})
}

func deleteSession(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return
	}
	// Scope to type=jwt so a stray id can't nuke a magic-link token.
	if err := system.GetStorage().DeleteMany(c.Request.Context(), constants.MongoTokensCollection,
		bson.M{"id": id, "type": models.TokenTypeJWT}); err != nil {
		respondInternal(c, "delete session", err)
		return
	}
	logrus.Infof("[admin] revoked session %s", id)
	c.Status(http.StatusNoContent)
}

func deleteUserSessions(c *gin.Context) {
	id := c.Param("id")
	user, ok := loadUserOr404(c, id)
	if !ok {
		return
	}
	filter := bson.M{
		"type": models.TokenTypeJWT,
		"$or": []bson.M{
			{"user_id": id},
			{"email": user.Email},
		},
	}
	if err := system.GetStorage().DeleteMany(c.Request.Context(), constants.MongoTokensCollection, filter); err != nil {
		respondInternal(c, "delete sessions", err)
		return
	}
	logrus.Infof("[admin] revoked all sessions for user=%s", id)
	c.Status(http.StatusNoContent)
}

// listSessionsFor mirrors listTestTokensFor (tokens.go) but scoped
// to type=jwt.
func listSessionsFor(ctx context.Context, user *models.User) ([]models.Token, error) {
	filter := bson.M{
		"type": models.TokenTypeJWT,
		"$or": []bson.M{
			{"user_id": user.ID},
			{"email": user.Email},
		},
	}
	var sessions []models.Token
	err := system.GetStorage().GetMany(ctx, constants.MongoTokensCollection, filter, &sessions)
	return sessions, err
}
