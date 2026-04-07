package me

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/api/server"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Routes implements server.RouteGroup for user configuration endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	g := r.Group("/me", server.JWTMiddleware())
	g.GET("", handleGetMe)
	g.PUT("/config", handlePutConfig)

	if err := system.GetStorage().EnsureCompoundIndex(system.Ctx(), system.CompoundIndexSpec{
		Collection: constants.MongoUsersConfigurationsCollection,
		Fields:     []string{"user_id", "platform", "name"},
		Unique:     true,
	}); err != nil {
		logrus.Fatalf("configurations index: %v", err)
	}
}

// handleGetMe returns configurations and the active CV for the authenticated user.
// Query params: platform
func handleGetMe(c *gin.Context) {
	userID, _ := c.Get("user_id")
	platform := c.Query("platform")
	if userID == nil || userID == "" || platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "platform is required"})
		return
	}
	uid := userID.(string)

	ctx := c.Request.Context()

	// Configs
	var configs []models.UserConfig
	err := system.GetStorage().GetMany(ctx, constants.MongoUsersConfigurationsCollection, bson.M{
		"user_id":  uid,
		"platform": platform,
	}, &configs)
	if err != nil {
		logrus.Errorf("get configs user=%s platform=%s: %v", uid, platform, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch configurations"})
		return
	}

	cfgMap := make(map[string]any, len(configs))
	for _, cfg := range configs {
		cfgMap[cfg.Name] = cfg.Value
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
	Platform string `json:"platform" binding:"required"`
	Name     string `json:"name"     binding:"required"`
	Value    any    `json:"value"`
}

// handlePutConfig upserts a single configuration entry.
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
		Name:      req.Name,
		Value:     req.Value,
		UpdatedAt: time.Now(),
	}

	filter := bson.M{
		"user_id":  uid.(string),
		"platform": req.Platform,
		"name":     req.Name,
	}
	if err := system.GetStorage().Set(ctx, constants.MongoUsersConfigurationsCollection, filter, cfg); err != nil {
		logrus.Errorf("put config user=%s platform=%s name=%s: %v", uid, req.Platform, req.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
