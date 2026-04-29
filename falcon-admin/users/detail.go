package users

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/packages/auth"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/datasource"
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
	tokens, _ := auth.ListTestTokensFor(ctx, user)
	sessions, _ := auth.ListSessionsFor(ctx, user)
	devices, _ := listDevicesFor(ctx, id)
	detail.ActiveTokens = auth.CountActiveTokens(tokens)
	detail.ActiveSessions = auth.CountActiveTokens(sessions)
	detail.DeviceCount = len(devices)
	// Surface the cv-reminder bookkeeping when a row exists. Absent
	// row = signal hasn't sent yet for this user (either pre-grace
	// or new user since the loop started). We don't synthesise an
	// empty summary; UI infers "no reminders yet" from missing field.
	if rem, ok := loadCVReminder(ctx, id); ok {
		detail.CVReminder = &cvReminderSummary{
			Count:   rem.Count,
			FirstAt: rem.FirstAt,
			LastAt:  rem.LastAt,
			Stopped: rem.Stopped,
		}
	}
	c.JSON(http.StatusOK, detail)
}

// loadCVReminder reads the cv-reminder row for a user. Returns
// (zero, false) when none exists yet — that's expected for users
// who registered after signal restarted but before the first tick,
// or for users still inside the 1-day grace period.
func loadCVReminder(ctx context.Context, userID string) (models.UserReminder, bool) {
	var rem models.UserReminder
	err := system.GetStorage().Get(ctx, constants.MongoUserRemindersCollection,
		bson.M{"user_id": userID, "kind": string(models.UserReminderKindCVUpload)},
		&rem)
	if err != nil {
		if !errors.Is(err, mongo.ErrNoDocuments) {
			// Logging skipped — this is a best-effort enrichment for
			// the UI; a transient Mongo hiccup shouldn't fail the
			// main user fetch.
			_ = err
		}
		return models.UserReminder{}, false
	}
	return rem, true
}

// loadUserOr404 reads the user document by id and writes a 404
// response if missing. Returns (user, true) on success, (nil, false)
// when the caller should bail out. Thin gin wrapper over
// `datasource.FindUserByID` so the query primitive lives in one
// place (shared with the analog in `packages/auth/test_tokens.go`).
func loadUserOr404(c *gin.Context, id string) (*models.User, bool) {
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id required"})
		return nil, false
	}
	u, err := datasource.FindUserByID(c.Request.Context(), id)
	if err != nil {
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
