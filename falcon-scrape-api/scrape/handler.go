package scrape

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/models"
)

// routes registers all HTTP routes on r for svc.
func routes(r *gin.Engine, svc *Service) {
	r.GET("/health", health)
	r.POST("/scrape", handleScrape(svc))
}

func health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func handleScrape(svc *Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var body struct {
			Platform string `json:"platform" binding:"required"`
			URL      string `json:"url"      binding:"required"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		event := models.ScrapeRequestedEvent{
			Platform:    strings.ToLower(strings.TrimSpace(body.Platform)),
			URL:         strings.TrimSpace(body.URL),
			RequestedAt: time.Now(),
		}

		if err := svc.Publish(c.Request.Context(), event); err != nil {
			logrus.Errorf("publish scrape.requested: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue scrape request"})
			return
		}

		c.JSON(http.StatusAccepted, gin.H{"status": "queued", "platform": event.Platform, "url": event.URL})
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
