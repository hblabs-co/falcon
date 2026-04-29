package signal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/models"
	"hblabs.co/falcon/packages/system"
	"hblabs.co/falcon/signal/push"
)

// handleAdminTestPush is triggered by GET /admin/signal/test-push.
// Looks the requested template up in falcon-signal/push/templates.yaml,
// renders it for the requested language (falls back to "en"), and fans
// the resulting push out to every admin's registered iOS device tokens.
//
// The same path the operator hits will be used by future
// reminder/notification flows that need a templated push to a known
// audience — no per-template wiring required here, just add an entry
// to push/templates.yaml.
func (s *Service) handleAdminTestPush(data []byte) error {
	var event models.AdminTestPushEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal admin_test_push: %w", err)
	}
	if event.TemplateID == "" {
		return fmt.Errorf("admin_test_push: empty template_id")
	}
	if !push.Has(event.TemplateID) {
		ids := make([]string, 0, len(push.List()))
		for _, t := range push.List() {
			ids = append(ids, t.ID)
		}
		return fmt.Errorf("admin_test_push: unknown template_id %q (available: %v)", event.TemplateID, ids)
	}

	lang := event.Lang
	if lang == "" {
		lang = "en"
	}
	payload, err := push.Render(event.TemplateID, lang)
	if err != nil {
		return fmt.Errorf("admin_test_push render: %w", err)
	}

	ctx := context.Background()
	log := logrus.WithFields(logrus.Fields{"template": event.TemplateID, "lang": lang})

	if s.admin == nil || s.admin.config.Empty() {
		log.Warn("admin_test_push: ADMIN_EMAILS not set — nothing to do")
		return nil
	}

	// Fan out to every admin email. Each admin may have several device
	// tokens (phone + tablet); send to every one. Same shape as
	// AdminNotifier.sendPush but keyed off a templated payload.
	var staleTokens []string
	for _, adminEmail := range s.admin.config.List() {
		var user models.User
		if err := system.GetStorage().GetByField(ctx, constants.MongoUsersCollection, "email", adminEmail, &user); err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				log.Warnf("admin_test_push: admin %s is not a registered Falcon user — skip", adminEmail)
				continue
			}
			log.Warnf("admin_test_push: lookup admin %s failed: %v — skip", adminEmail, err)
			continue
		}

		var tokens []models.IOSDeviceToken
		if err := system.GetStorage().GetAllByField(ctx, constants.MongoIOSDeviceTokensCollection, "user_id", user.ID, &tokens); err != nil {
			log.Warnf("admin_test_push: fetch tokens for %s: %v — skip", adminEmail, err)
			continue
		}
		if len(tokens) == 0 {
			log.Warnf("admin_test_push: admin %s has no registered iOS device tokens", adminEmail)
			continue
		}

		extras := map[string]string{
			"template_id": event.TemplateID,
			"source":      "admin_test_push",
		}
		for _, dt := range tokens {
			if err := s.apns.SendTemplated(ctx, dt.Token, payload, extras); err != nil {
				if s.apns.IsStaleToken(err) {
					log.Warnf("stale apns token %s… — queued for removal", safePrefix(dt.Token, 8))
					staleTokens = append(staleTokens, dt.Token)
				} else {
					log.Errorf("admin_test_push send to %s…: %v", safePrefix(dt.Token, 8), err)
				}
				continue
			}
			log.Infof("admin_test_push sent to %s device %s…", adminEmail, safePrefix(dt.Token, 8))
		}
	}

	if len(staleTokens) > 0 {
		if err := system.GetStorage().DeleteManyByFieldIn(ctx, constants.MongoIOSDeviceTokensCollection, "token", staleTokens); err != nil {
			log.Errorf("bulk delete stale admin tokens: %v", err)
		} else {
			log.Infof("removed %d stale admin token(s)", len(staleTokens))
		}
	}
	return nil
}
