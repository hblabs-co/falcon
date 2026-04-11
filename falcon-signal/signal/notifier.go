package signal

import (
	"context"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/signal/email"
)

// adminEmailLanguage is the locale used for admin alert emails. Admins are
// technical and consistent language is more useful than per-user resolution.
// If we ever need per-admin language, swap this for a lookup against the user
// configurations collection (same pattern as resolveUserLanguage).
const adminEmailLanguage = "en"

// AdminNotifier fans out a single ServiceError into email + APNs push for
// every entry in ADMIN_EMAILS. Email is always attempted; push is best-effort
// and only fires when the admin has a Falcon user account AND at least one
// registered iOS device token.
type AdminNotifier struct {
	apns   *apnsClient
	mail   *email.Client
	config AdminConfig
}

// NewAdminNotifier wires an AdminNotifier from the existing apns + mail
// clients and loads ADMIN_EMAILS from the environment.
func NewAdminNotifier(apns *apnsClient, mail *email.Client) *AdminNotifier {
	return &AdminNotifier{
		apns:   apns,
		mail:   mail,
		config: LoadAdminConfig(),
	}
}

// NotifyAll delivers the alert to every configured admin via email and (when
// possible) push. Failures for one admin do not block the others.
func (n *AdminNotifier) NotifyAll(ctx context.Context, errDoc *models.ServiceError) {
	if n.config.Empty() {
		logrus.Warn("admin notifier: ADMIN_EMAILS is empty — skipping alert")
		return
	}
	for _, addr := range n.config.List() {
		n.notifyOne(ctx, addr, errDoc)
	}
}

// notifyOne delivers email + push to a single admin. The two channels are
// independent — push failure (no user, no tokens, APNs rejected) does not
// affect email and vice versa.
func (n *AdminNotifier) notifyOne(ctx context.Context, adminEmail string, errDoc *models.ServiceError) {
	log := logrus.WithFields(logrus.Fields{
		"admin_email": adminEmail,
		"error_id":    errDoc.ID,
		"error_name":  errDoc.ErrorName,
	})

	if err := n.sendEmail(adminEmail, errDoc); err != nil {
		log.Errorf("admin alert email failed: %v", err)
	} else {
		log.Infof("admin alert email sent")
	}

	n.sendPush(ctx, adminEmail, errDoc, log)
}

// sendEmail renders and dispatches the admin_alert template via the existing
// mailjet client. Variables map to the placeholders defined in templates.yaml.
func (n *AdminNotifier) sendEmail(adminEmail string, errDoc *models.ServiceError) error {
	vars := map[string]string{
		"title":      adminAlertTitle(errDoc),
		"body":       errDoc.Error,
		"severity":   string(errDoc.Priority),
		"source":     errDoc.ServiceName,
		"platform":   errDoc.Platform,
		"error_name": errDoc.ErrorName,
		"error_id":   errDoc.ID,
	}
	return n.mail.Send(adminEmail, "admin_alert", adminEmailLanguage, vars)
}

// sendPush is the best-effort push path: lookup user by email, fetch ALL their
// device tokens (an admin can have several devices — phone + tablet + work
// device), send the push to each, and clean up any tokens that APNs rejected
// as stale. Same pattern as handleMatchResult.
//
// Distinguishes "admin is not a registered user" (expected — debug log, silent
// skip) from "actual DB failure" (warning) so an infrastructure outage doesn't
// hide behind the same code path as a deliberately email-only admin.
func (n *AdminNotifier) sendPush(ctx context.Context, adminEmail string, errDoc *models.ServiceError, log *logrus.Entry) {
	var user models.User
	if err := system.GetStorage().GetByField(ctx, constants.MongoUsersCollection, "email", adminEmail, &user); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			log.Error("admin push skipped — admin is not a registered Falcon user")
			return
		}
		log.Warnf("admin push skipped — user lookup failed (DB issue?): %v", err)
		return
	}

	var tokens []models.IOSDeviceToken
	if err := system.GetStorage().GetAllByField(ctx, constants.MongoIOSDeviceTokensCollection, "user_id", user.ID, &tokens); err != nil {
		log.Warnf("fetch device tokens for admin user %s: %v", user.ID, err)
		return
	}
	if len(tokens) == 0 {
		log.Warn("admin push skipped — admin has no registered iOS device tokens")
		return
	}

	var staleTokens []string
	for _, dt := range tokens {
		if err := n.apns.SendAdminAlert(ctx, dt.Token, errDoc); err != nil {
			if n.apns.IsStaleToken(err) {
				log.Warnf("stale apns token %s… — queued for removal", safePrefix(dt.Token, 8))
				staleTokens = append(staleTokens, dt.Token)
			} else {
				log.Errorf("admin push failed for device %s…: %v", safePrefix(dt.Token, 8), err)
			}
			continue
		}
		log.Infof("admin push sent to device %s…", safePrefix(dt.Token, 8))
	}

	if len(staleTokens) > 0 {
		if err := system.GetStorage().DeleteManyByFieldIn(ctx, constants.MongoIOSDeviceTokensCollection, "token", staleTokens); err != nil {
			log.Errorf("bulk delete stale admin tokens: %v", err)
		} else {
			log.Infof("removed %d stale admin token(s)", len(staleTokens))
		}
	}
}

// adminAlertTitle builds a short headline used as the email subject and the
// APNs alert title. Always includes the platform when present so triage at a
// glance is possible from the lock screen.
func adminAlertTitle(errDoc *models.ServiceError) string {
	if errDoc.Platform != "" {
		return fmt.Sprintf("[%s] %s on %s", errDoc.Priority, errDoc.ErrorName, errDoc.Platform)
	}
	return fmt.Sprintf("[%s] %s", errDoc.Priority, errDoc.ErrorName)
}

// safePrefix returns the first n runes of s, or all of s if it's shorter.
// Avoids the index-out-of-range panic that bare s[:n] would trigger when a
// stale or malformed token comes in.
func safePrefix(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
