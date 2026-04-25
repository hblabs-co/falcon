package scrape

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// Routes implements server.RouteGroup for scrape endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	r.POST("/scrape", handleScrape)
}

// handleScrape godoc
// POST /scrape
// Body: { "platform": "freelance.de", "url": "https://..." }
// Queues an on-demand scrape request via NATS.
func handleScrape(c *gin.Context) {
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
	subject := fmt.Sprintf("%s.%s", constants.SubjectScrapeRequested, event.Platform)

	if err := system.Publish(c.Request.Context(), subject, event); err != nil {
		logrus.Errorf("publish %s: %v", subject, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to queue scrape request"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "queued", "platform": event.Platform, "url": event.URL})
}
