package auth

// Shared handler bodies for the user-scoped admin endpoints over
// `auth_tokens` (tokens + sessions). Both the list and the delete
// endpoints follow the same shape: load the user from the URL
// param, then fan out to a kind-specific loader/scope. Centralised
// here so the 4 handlers (List/Delete × Tokens/Sessions) become
// one-line redirects and any change to the shared shape (auth
// gates, response envelope, log format) lands once.

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

// listForUser is the shared body of "list <X> for user :id" admin
// handlers. Loads the user via `loadUserOr404`, calls `load` to
// fetch the rows, projects them through `TokensToView`, and writes
// `{"count": N, <responseKey>: [...]}`.
//
// `responseKey` doubles as the noun in the error log ("list
// <responseKey> for <userID>: <err>") so the wire shape and log
// stay aligned without a separate label arg.
func listForUser(
	c *gin.Context,
	responseKey string,
	load func(ctx context.Context, u *models.User) ([]models.Token, error),
) {
	user, ok := loadUserOr404(c)
	if !ok {
		return
	}
	rows, err := load(c.Request.Context(), user)
	if err != nil {
		logrus.Errorf(logPrefix+" list %s for %s: %v", responseKey, user.ID, err)
		system.RespondInternal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{"count": len(rows), responseKey: TokensToView(rows)})
}

// deleteForUser is the shared body of "wipe <X> for user :id"
// admin handlers. Loads the user, then runs `DeleteMany` against
// `auth_tokens` with the scope-narrowed user filter (see
// `userScopeFilter`). `label` is the noun used in both the error
// and success log lines so they read naturally ("delete sessions
// for X" / "revoked all sessions for user=X").
func deleteForUser(c *gin.Context, label string, scope bson.M) {
	user, ok := loadUserOr404(c)
	if !ok {
		return
	}
	if err := system.GetStorage().DeleteMany(c.Request.Context(),
		constants.MongoAuthTokensCollection,
		userScopeFilter(user, scope),
	); err != nil {
		logrus.Errorf(logPrefix+" delete %s for %s: %v", label, user.ID, err)
		system.RespondInternal(c)
		return
	}
	logrus.Infof(logPrefix+" revoked all %s for user=%s", label, user.ID)
	c.Status(http.StatusNoContent)
}
