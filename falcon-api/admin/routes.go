package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/system"
)

// Routes implements server.RouteGroup for admin endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	g := r.Group("/admin")
	g.POST("/scout/scan-today", handleScanToday)
}

// handleScanToday publishes a scrape.scan_today event to NATS.
// The scout picks it up and processes all of today's candidates.
func handleScanToday(c *gin.Context) {
	if err := system.Publish(c.Request.Context(), constants.SubjectScrapeScanToday, map[string]string{
		"triggered_by": "admin",
	}); err != nil {
		logrus.Errorf("publish scan-today: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to trigger scan"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "scan triggered"})
}
