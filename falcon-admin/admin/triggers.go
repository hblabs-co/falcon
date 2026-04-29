package admin

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/system"
)

// triggerScanToday publishes a scrape.scan_today event. Scout picks
// it up and processes today's candidates. POST so a stray Safari
// paste / link prefetch can't accidentally trigger a scrape.
//
// Stays in this file (instead of moving to a `falcon-admin/scout/`
// module) because it's the only scout-domain admin trigger today —
// not enough surface to justify its own module yet. Promote later
// if a second one shows up.
func triggerScanToday(c *gin.Context) {
	if err := system.Publish(c.Request.Context(), constants.SubjectScrapeScanToday, map[string]string{
		"triggered_by": "admin",
	}); err != nil {
		logrus.Errorf("publish scan-today: %v", err)
		system.RespondInternal(c, "failed to trigger scan")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "scan triggered"})
}
