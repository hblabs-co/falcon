package signal

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Routes implements server.RouteGroup for signal endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	r.POST("/device-token", handleRegisterIOSDeviceToken)
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
