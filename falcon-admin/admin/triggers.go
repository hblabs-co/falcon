package admin

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
)

// One-shot operational triggers exposed by falcon-admin: publish a
// NATS event from an HTTP call. Used by ops automation (scan-today)
// and by App Review smoke tests where someone needs to fire the
// signal pipeline without producing a real user event. Each handler
// is small and side-effect-clear so the whole surface fits on one
// page; they're mounted by Handler() inside the bearer-protected
// group, so anyone hitting them already presented ADMIN_TOKEN.

// triggerScanToday publishes a scrape.scan_today event. Scout picks
// it up and processes today's candidates. POST so a stray Safari
// paste / link prefetch can't accidentally trigger a scrape.
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

// triggerTestAlert inserts a synthetic ServiceWarning and publishes
// signal.admin_alert pointing at it. Signal then runs the full admin
// pipeline (email + push for every ADMIN_EMAILS entry), exercising
// the whole path end-to-end. The warning row is clearly tagged
// (warning_name=admin_test_notification, priority=low) and safe to
// delete afterwards.
func triggerTestAlert(c *gin.Context) {
	ctx := c.Request.Context()

	warn := models.ServiceWarning{
		ID:          gonanoid.Must(),
		ServiceName: constants.ServiceAdmin,
		WarningName: "admin_test_notification",
		Message:     "Test admin notification triggered manually from GET /signal/test-alert. Safe to ignore.",
		Priority:    models.WarningPriorityLow,
		OccurredAt:  time.Now(),
	}
	if err := system.GetStorage().Insert(ctx, constants.MongoWarningsCollection, warn); err != nil {
		logrus.Errorf("insert test warning: %v", err)
		system.RespondInternal(c, "failed to create test warning")
		return
	}

	evt := models.AdminAlertEvent{
		Kind: models.AdminAlertKindWarning,
		ID:   warn.ID,
	}
	if err := system.Publish(ctx, constants.SubjectSignalAdminAlert, evt); err != nil {
		logrus.Errorf("publish test admin_alert: %v", err)
		system.RespondInternal(c, "failed to publish test alert")
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status":     "test alert triggered",
		"warning_id": warn.ID,
	})
}

// triggerTestLastMatch publishes signal.admin_test_match. For each
// admin in ADMIN_EMAILS, signal fetches their match_result at the
// given index (scored_at desc, same order iOS shows) and pushes it.
//
// Query: ?index=N (default 0 = latest). Use 1+ when the latest match
// is already in notification center and you want a fresh delivery.
func triggerTestLastMatch(c *gin.Context) {
	index := 0
	if s := c.Query("index"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n >= 0 {
			index = n
		}
	}

	evt := models.AdminTestMatchEvent{Index: index}
	if err := system.Publish(c.Request.Context(), constants.SubjectSignalAdminTestMatch, evt); err != nil {
		logrus.Errorf("publish admin_test_match: %v", err)
		system.RespondInternal(c, "failed to trigger test match push")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"status": "match push triggered for admins",
		"index":  index,
	})
}

// triggerTestEmail exercises the templated email pipeline end-to-end.
// Signal looks template_id up in falcon-signal/email/templates.yaml,
// renders for the requested language with the supplied vars, and
// sends the result to every entry in ADMIN_EMAILS.
//
// Query:
//
//	template_id  required-with-default. Any id from email/templates.yaml
//	             — defaults to "admin_alert" so a bare URL still produces
//	             an email (the admin_alert template renders fine even
//	             with most vars empty).
//	lang         optional. en|de|es. Defaults to "en"; signal also
//	             falls back to "en" if the requested language is
//	             missing for that template.
//	var.<key>=v  any querystring param prefixed with `var.` becomes a
//	             template variable. Example: cv_reminder needs an
//	             upload_link → ?template_id=cv_reminder&var.upload_link=https://falcon.app/upload
func triggerTestEmail(c *gin.Context) {
	tpl := c.DefaultQuery("template_id", "admin_alert")
	lang := c.DefaultQuery("lang", "en")

	// Collect every ?var.<key>=value into the vars map. Lets the
	// caller pass arbitrary placeholders without us hard-coding a
	// per-template body shape.
	vars := map[string]string{}
	for k, v := range c.Request.URL.Query() {
		if !strings.HasPrefix(k, "var.") || len(v) == 0 {
			continue
		}
		vars[strings.TrimPrefix(k, "var.")] = v[0]
	}

	evt := models.AdminTestEmailEvent{TemplateID: tpl, Lang: lang, Vars: vars}
	if err := system.Publish(c.Request.Context(), constants.SubjectSignalAdminTestEmail, evt); err != nil {
		logrus.Errorf("publish admin_test_email: %v", err)
		system.RespondInternal(c, "failed to trigger test email")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"status":      "test email triggered for admins",
		"template_id": tpl,
		"lang":        lang,
		"vars":        vars,
	})
}

// triggerTestPush exercises the templated push pipeline end-to-end.
// Signal looks template_id up in falcon-signal/push/templates.yaml,
// renders for the requested language, and fans the result out to
// every admin's iOS device tokens.
//
// Query:
//
//	template_id  defaults to "admin_test_push" so a bare URL still
//	             produces a delivery. Any id from push/templates.yaml
//	             works — `cv_reminder`, future templates, etc.
//	lang         optional. en|de|es. Defaults to "en"; signal also
//	             falls back to "en" if the requested language is
//	             missing for that template.
func triggerTestPush(c *gin.Context) {
	tpl := c.DefaultQuery("template_id", "admin_test_push")
	lang := c.DefaultQuery("lang", "en")

	evt := models.AdminTestPushEvent{TemplateID: tpl, Lang: lang}
	if err := system.Publish(c.Request.Context(), constants.SubjectSignalAdminTestPush, evt); err != nil {
		logrus.Errorf("publish admin_test_push: %v", err)
		system.RespondInternal(c, "failed to trigger test push")
		return
	}
	c.JSON(http.StatusAccepted, gin.H{
		"status":      "test push triggered for admins",
		"template_id": tpl,
		"lang":        lang,
	})
}
