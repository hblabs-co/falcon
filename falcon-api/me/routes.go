package me

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Routes implements server.RouteGroup for user configuration endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	r.GET("/me", handleGetMe)
	r.PUT("/me/config", handlePutConfig)

	if err := system.GetStorage().EnsureCompoundIndex(system.Ctx(), system.CompoundIndexSpec{
		Collection: constants.MongoUsersConfigurationsCollection,
		Fields:     []string{"user_id", "platform", "name"},
		Unique:     true,
	}); err != nil {
		logrus.Fatalf("configurations index: %v", err)
	}
}

// handleGetMe returns all configurations for a given user+platform as a flat map.
// Query params: user_id, platform
func handleGetMe(c *gin.Context) {
	userID := c.Query("user_id")
	platform := c.Query("platform")
	if userID == "" || platform == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "user_id and platform are required"})
		return
	}

	ctx := c.Request.Context()
	var configs []models.UserConfig
	err := system.GetStorage().GetMany(ctx, constants.MongoUsersConfigurationsCollection, bson.M{
		"user_id":  userID,
		"platform": platform,
	}, &configs)
	if err != nil {
		logrus.Errorf("get configs user=%s platform=%s: %v", userID, platform, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch configurations"})
		return
	}

	result := make(map[string]any, len(configs))
	for _, cfg := range configs {
		result[cfg.Name] = cfg.Value
	}

	c.JSON(http.StatusOK, gin.H{"configs": result})
}

type putConfigRequest struct {
	UserID   string `json:"user_id"  binding:"required"`
	Platform string `json:"platform" binding:"required"`
	Name     string `json:"name"     binding:"required"`
	Value    any    `json:"value"`
}

// handlePutConfig upserts a single configuration entry.
func handlePutConfig(c *gin.Context) {
	var req putConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	ctx := c.Request.Context()
	cfg := models.UserConfig{
		UserID:    req.UserID,
		Platform:  req.Platform,
		Name:      req.Name,
		Value:     req.Value,
		UpdatedAt: time.Now(),
	}

	filter := bson.M{
		"user_id":  req.UserID,
		"platform": req.Platform,
		"name":     req.Name,
	}
	if err := system.GetStorage().Set(ctx, constants.MongoUsersConfigurationsCollection, filter, cfg); err != nil {
		logrus.Errorf("put config user=%s platform=%s name=%s: %v", req.UserID, req.Platform, req.Name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save configuration"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}
