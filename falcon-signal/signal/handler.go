package signal

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// routes registers all HTTP routes on r for svc.
func routes(r *gin.Engine) {
	r.GET("/health", health)
	r.POST("/device-token", handleRegisterToken())
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// handleRegisterToken godoc
// POST /device-token
// Body: { "user_id": "...", "token": "<apns device token>" }
// Upserts the device token for the given user.
func handleRegisterToken() gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			UserID string `json:"user_id" binding:"required"`
			Token  string `json:"token"   binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		ctx := c.Request.Context()
		now := time.Now()

		dt := models.DeviceToken{
			ID:        gonanoid.Must(),
			UserID:    body.UserID,
			Token:     body.Token,
			CreatedAt: now,
			UpdatedAt: now,
		}

		if err := system.GetStorage().Set(ctx, constants.MongoDeviceTokensCollection,
			map[string]any{"user_id": body.UserID},
			dt,
		); err != nil {
			logrus.Errorf("register device token: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not register device token"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"status": "registered"})
	}
}

func ginLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		logrus.WithFields(logrus.Fields{
			"method": c.Request.Method,
			"path":   c.Request.URL.Path,
			"status": c.Writer.Status(),
		}).Info("request")
	}
}
