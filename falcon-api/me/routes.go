package me

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/api/server"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// Routes implements server.RouteGroup for user configuration endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	g := r.Group("/me", server.JWTMiddleware())
	g.GET("", handleGetMe)
	g.PUT("/config", handlePutConfig)
}

// handleGetMe returns configurations and the active CV for the authenticated user.
// Query params:
//   - platform  (required)
//   - device_id (optional) — when present, the response merges user-wide configs
//     with this device's overrides (device-specific values win).
func handleGetMe(c *gin.Context) {
	userID, _ := c.Get("user_id")
	platform := c.Query("platform")
	deviceID := c.Query("device_id")
	if userID == nil || userID == "" || platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform is required"})
		return
	}
	uid := userID.(string)

	ctx := c.Request.Context()

	// Fetch user-wide configs + (optionally) this device's overrides in one query.
	configFilter := bson.M{
		"user_id":  uid,
		"platform": platform,
	}
	if deviceID != "" {
		configFilter["device_id"] = bson.M{"$in": []string{"", deviceID}}
	} else {
		configFilter["device_id"] = ""
	}

	var configs []models.UserConfig
	if err := system.GetStorage().GetMany(ctx, constants.MongoUsersConfigurationsCollection, configFilter, &configs); err != nil {
		logrus.Errorf("get configs user=%s platform=%s device=%s: %v", uid, platform, deviceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch configurations"})
		return
	}

	// Merge: put user-wide first, then let device-specific overwrite per name.
	cfgMap := make(map[string]any, len(configs))
	for _, cfg := range configs {
		if cfg.DeviceID == "" {
			cfgMap[cfg.Name] = cfg.Value
		}
	}
	for _, cfg := range configs {
		if cfg.DeviceID != "" {
			cfgMap[cfg.Name] = cfg.Value
		}
	}

	// Active CV (latest by user_id — there's at most one after dedup in falcon-storage)
	var cvs []models.PersistedCV
	_ = system.GetStorage().GetAllByField(ctx, constants.MongoCVsCollection, "user_id", uid, &cvs)

	var activeCVResponse any
	if len(cvs) > 0 {
		activeCVResponse = cvs[0]
	}

	c.JSON(http.StatusOK, gin.H{"configs": cfgMap, "cv": activeCVResponse})
}

type putConfigRequest struct {
	Platform string `json:"platform"  binding:"required"`
	// DeviceID is optional. Empty → user-wide default that applies to all of
	// the user's devices. Non-empty → per-device override for this setting.
	DeviceID string `json:"device_id"`
	Name     string `json:"name"      binding:"required"`
	Value    any    `json:"value"`
}

// handlePutConfig upserts a single configuration entry, scoped user-wide or
// per-device depending on whether device_id is present in the request.
func handlePutConfig(c *gin.Context) {
	uid, _ := c.Get("user_id")

	var req putConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	cfg := models.UserConfig{
		UserID:    uid.(string),
		Platform:  req.Platform,
		DeviceID:  req.DeviceID,
		Name:      req.Name,
		Value:     req.Value,
		UpdatedAt: time.Now(),
	}

	filter := bson.M{
		"user_id":   uid.(string),
		"platform":  req.Platform,
		"device_id": req.DeviceID,
		"name":      req.Name,
	}
	if err := system.GetStorage().Set(ctx, constants.MongoUsersConfigurationsCollection, filter, cfg); err != nil {
		logrus.Errorf("put config user=%s platform=%s device=%s name=%s: %v", uid, req.Platform, req.DeviceID, req.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
