package users

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// getUser returns the per-user header info: name (from CV), email,
// joined_at, and the counts the UI needs to decide which action
// buttons to render (active tokens, active sessions, devices, has_cv).
func getUser(c *gin.Context) {
	id := c.Param("id")
	ctx := c.Request.Context()
	user, ok := loadUserOr404(c, id)
	if !ok {
		return
	}
	detail := userDetail{
		userView: userView{UserID: user.ID, Email: user.Email, JoinedAt: user.CreatedAt},
	}
	if cv, err := findCVForUser(ctx, id); err == nil && cv != nil {
		detail.FirstName, detail.LastName = pickName(cv.Normalized)
		detail.HasCV = cv.MinioBucket != "" && cv.MinioKey != ""
		detail.CVFilename = cv.Filename
	}
	tokens, _ := listTestTokensFor(ctx, user)
	sessions, _ := listSessionsFor(ctx, user)
	devices, _ := listDevicesFor(ctx, id)
	detail.ActiveTokens = countActive(tokens)
	detail.ActiveSessions = countActive(sessions)
	detail.DeviceCount = len(devices)
	c.JSON(http.StatusOK, detail)
}

// loadUserOr404 reads the user document by id and writes a 404
// response if missing. Returns (user, true) on success, (nil, false)
// when the caller should bail out.
func loadUserOr404(c *gin.Context, id string) (*models.User, bool) {
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return nil, false
	}
	var u models.User
	if err := system.GetStorage().GetById(c.Request.Context(),
		constants.MongoUsersCollection, id, &u); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return nil, false
	}
	return &u, true
}

func findCVForUser(ctx context.Context, userID string) (*models.PersistedCV, error) {
	var cv models.PersistedCV
	if err := system.GetStorage().GetByField(ctx, constants.MongoCVsCollection,
		"user_id", userID, &cv); err != nil {
		return nil, err
	}
	return &cv, nil
}
