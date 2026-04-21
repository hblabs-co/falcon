package signal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Routes implements server.RouteGroup for signal endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	r.POST("/device-token", handleRegisterIOSDeviceToken)
	r.POST("/device-token/logout", handleLogoutIOSDeviceToken)
	r.POST("/live-activity-update-token", handleLiveActivityUpdateToken)
}

// handleRegisterToken godoc
// POST /device-token
// Body: {
//   "user_id":  "...",
//   "device_id": "...",
//   "token":    "<apns device token>",
//   "live_activity_token": "<iOS 17.2+ push-to-start token>"  (optional)
// }
// Publishes signal.device_token.register to NATS — falcon-signal persists it.
func handleRegisterIOSDeviceToken(c *gin.Context) {
	var body struct {
		UserID            string `json:"user_id"             binding:"required"`
		DeviceID          string `json:"device_id"           binding:"required"`
		Token             string `json:"token"               binding:"required"`
		LiveActivityToken string `json:"live_activity_token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	evt := models.IOSDeviceTokenRegisterEvent{
		UserID:            body.UserID,
		DeviceID:          body.DeviceID,
		Token:             body.Token,
		LiveActivityToken: body.LiveActivityToken,
	}
	if err := system.Publish(c.Request.Context(), constants.SubjectSignalDeviceTokenRegister, evt); err != nil {
		logrus.Errorf("publish device_token.register: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "registered"})
}

// handleLogoutIOSDeviceToken godoc
// POST /device-token/logout
// Body: { "device_id": "...", "user_id": "..." }
// Publishes signal.device_token.logout so falcon-signal unbinds the row
// (clears user_id + live_activity_token) without deleting the APNs token.
// user_id is optional but strongly recommended — it scopes the unbind so
// a concurrent re-register by a different user on the same device isn't
// accidentally overwritten.
func handleLogoutIOSDeviceToken(c *gin.Context) {
	var body struct {
		DeviceID string `json:"device_id" binding:"required"`
		UserID   string `json:"user_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Delete JWT session rows for this (user, device). Kept minimal —
	// we want the tokens collection to stop recognising this session
	// immediately, not just after expiry. Missing rows is a no-op.
	if body.UserID != "" {
		if err := system.GetStorage().DeleteMany(c.Request.Context(), constants.MongoTokensCollection, bson.M{
			"user_id":   body.UserID,
			"device_id": body.DeviceID,
			"type":      models.TokenTypeJWT,
		}); err != nil {
			logrus.Warnf("[signal] logout: delete JWTs for user=%s device=%s: %v", body.UserID, body.DeviceID, err)
		}
	}

	evt := models.IOSDeviceTokenLogoutEvent{
		DeviceID: body.DeviceID,
		UserID:   body.UserID,
	}
	if err := system.Publish(c.Request.Context(), constants.SubjectSignalDeviceTokenLogout, evt); err != nil {
		logrus.Errorf("publish device_token.logout: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to logout device"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "logged_out"})
}

// handleLiveActivityUpdateToken godoc
// POST /live-activity-update-token
// Body: { "device_id": "...", "token": "<update token or empty to clear>" }
// Publishes signal.live_activity_update_token so signal persists it per-device.
// iOS calls this whenever a Live Activity starts (gets an update token) or ends
// (token becomes invalid, posts empty).
func handleLiveActivityUpdateToken(c *gin.Context) {
	var body struct {
		DeviceID string `json:"device_id" binding:"required"`
		Token    string `json:"token"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	evt := models.IOSLiveActivityUpdateTokenEvent{
		DeviceID: body.DeviceID,
		Token:    body.Token,
	}
	if err := system.Publish(c.Request.Context(), constants.SubjectSignalLiveActivityUpdate, evt); err != nil {
		logrus.Errorf("publish live_activity_update_token: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to persist token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}
