package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Routes implements server.RouteGroup for admin endpoints.
type Routes struct{}

func (Routes) Mount(r *gin.Engine) {
	g := r.Group("/admin")

	// /scout/scan-today stays POST because it's only hit by ops
	// automation (cron, CI), never by a human pasting the URL.
	g.POST("/scout/scan-today", handleScanToday)

	// GET on the two /signal/test-* endpoints is an intentional REST
	// bend: Apple's App Review team pastes the URL into Safari on the
	// test iPhone to trigger a push that exercises the notification
	// path (otherwise the reviewer sees an empty app with no way to
	// produce a match in real time). Side effects are contained: each
	// call inserts one clearly-tagged warning row / fires one push,
	// both easy to clean up later. Kept as GET-only (no POST mirror)
	// so the route surface stays minimal.
	g.GET("/signal/test-alert",      handleTestAlert)
	g.GET("/signal/test-last-match", handleTestLastMatch)
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

// handleTestAlert inserts a synthetic ServiceWarning and publishes a
// signal.admin_alert event pointing at it. falcon-signal then fans out the
// alert through the full admin pipeline (email + push for every ADMIN_EMAILS
// entry), so this exercises the whole path end-to-end.
//
// The warning is clearly identifiable (warning_name=admin_test_notification,
// priority=low) and safe to delete afterwards.
func handleTestAlert(c *gin.Context) {
	ctx := c.Request.Context()

	warn := models.ServiceWarning{
		ID:          gonanoid.Must(),
		ServiceName: constants.ServiceAPI,
		WarningName: "admin_test_notification",
		Message:     "This is a test admin notification triggered manually from POST /admin/signal/test-alert. Safe to ignore.",
		Priority:    models.WarningPriorityLow,
		OccurredAt:  time.Now(),
	}
	if err := system.GetStorage().Insert(ctx, constants.MongoWarningsCollection, warn); err != nil {
		logrus.Errorf("insert test warning: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to create test warning"})
		return
	}

	evt := models.AdminAlertEvent{
		Kind: models.AdminAlertKindWarning,
		ID:   warn.ID,
	}
	if err := system.Publish(ctx, constants.SubjectSignalAdminAlert, evt); err != nil {
		logrus.Errorf("publish test admin_alert: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to publish test alert"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":     "test alert triggered",
		"warning_id": warn.ID,
	})
}

// handleTestLastMatch triggers a push notification delivery test: for each
// admin in ADMIN_EMAILS, signal fetches that admin's match_result at the
// given index (scored_at desc, same order iOS shows) and pushes it.
//
// Query param ?index=N (default 0 = most recent). Use 1 to pick the second
// most recent, etc. — useful when the latest match is already sitting in
// notification center and you want a fresh delivery to test.
func handleTestLastMatch(c *gin.Context) {
	index := 0
	if s := c.Query("index"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			index = n
		}
	}

	evt := models.AdminTestMatchEvent{Index: index}
	if err := system.Publish(c.Request.Context(), constants.SubjectSignalAdminTestMatch, evt); err != nil {
		logrus.Errorf("publish admin_test_match: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to trigger test match push"})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"status": "match push triggered for admins",
		"index":  index,
	})
}
