package signal

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/v2/bson"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
	"hblabs.co/falcon/signal/email"
)

// Service handles push notifications, device token persistence, and transactional email.
type Service struct {
	apns     *apnsClient
	mail     *email.Client
	admin    *AdminNotifier
	alertBuf *alertBuffer
}

func newService() (*Service, error) {
	apns, err := newAPNSClient()
	if err != nil {
		return nil, fmt.Errorf("apns client: %w", err)
	}
	mail := email.NewClient()
	return &Service{
		apns:     apns,
		mail:     mail,
		admin:    NewAdminNotifier(apns, mail),
		alertBuf: newAlertBuffer(),
	}, nil
}

func (s *Service) handleMatchResult(data []byte) error {
	var event models.MatchResultEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return fmt.Errorf("unmarshal match.result: %w", err)
	}

	log := logrus.WithFields(logrus.Fields{
		"cv_id":      event.CVID,
		"project_id": event.ProjectID,
		"score":      event.Score,
	})

	ctx := context.Background()

	var tokens []models.IOSDeviceToken
	if err := system.GetStorage().GetAllByField(ctx, constants.MongoIOSDeviceTokensCollection, "user_id", event.UserID, &tokens); err != nil {
		log.Warnf("fetch device tokens for user %s: %v", event.UserID, err)
		return nil
	}
	if len(tokens) == 0 {
		log.Warnf("no device tokens for user %s — skipping push", event.UserID)
		return nil
	}

	var staleTokens []string
	for _, dt := range tokens {
		lang := s.resolveDeviceLanguage(event.UserID, "ios", dt.DeviceID)
		if err := s.apns.Send(ctx, dt.Token, &event, lang); err != nil {
			if s.apns.IsStaleToken(err) {
				log.Warnf("stale apns token %s…— queued for removal", dt.Token[:8])
				staleTokens = append(staleTokens, dt.Token)
			} else {
				log.Errorf("send push to device %s…: %v", dt.Token[:8], err)
			}
			continue
		}
		log.Infof("push sent to user %s device %s… (lang=%s)", event.UserID, dt.Token[:8], lang)
	}

	if len(staleTokens) > 0 {
		if err := system.GetStorage().DeleteManyByFieldIn(ctx, constants.MongoIOSDeviceTokensCollection, "token", staleTokens); err != nil {
			log.Errorf("bulk delete stale tokens: %v", err)
		} else {
			log.Infof("removed %d stale token(s) for user %s", len(staleTokens), event.UserID)
		}
	}

	return nil
}

func (s *Service) handleMagicLink(data []byte) error {
	var evt models.MagicLinkRequestedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal signal.magic_link: %w", err)
	}

	lang := s.resolveUserLanguage(evt.Email, evt.Platform)

	if err := s.mail.SendMagicLink(evt.Email, evt.MagicLink, lang); err != nil {
		return fmt.Errorf("send magic link to %s: %w", evt.Email, err)
	}

	logrus.Infof("[signal] magic link email sent to %s (lang=%s)", evt.Email, lang)
	return nil
}

// resolveDeviceLanguage returns the effective app_language for the given user
// on the given device. Lookup order:
//  1. Device-specific (user_id + platform + device_id + name=app_language)
//  2. User-wide default (device_id="")
//  3. Fallback "de" — the authoritative source language of MatchResultEvent.
//
// One query fetches both rows by $in{"", deviceID}; the helper then picks
// device-specific if present, else user-wide, else the default.
func (s *Service) resolveDeviceLanguage(userID, platform, deviceID string) string {
	ctx := context.Background()

	var configs []models.UserConfig
	err := system.GetStorage().GetMany(ctx, constants.MongoUsersConfigurationsCollection, bson.M{
		"user_id":   userID,
		"platform":  platform,
		"name":      constants.ConfigNameAppLanguage,
		"device_id": bson.M{"$in": []string{"", deviceID}},
	}, &configs)
	if err != nil || len(configs) == 0 {
		return "de"
	}

	var userWide string
	for _, cfg := range configs {
		if cfg.DeviceID == deviceID && deviceID != "" {
			if lang, ok := cfg.Value.(string); ok && lang != "" {
				return lang
			}
		}
		if cfg.DeviceID == "" {
			if lang, ok := cfg.Value.(string); ok && lang != "" {
				userWide = lang
			}
		}
	}
	if userWide != "" {
		return userWide
	}
	return "de"
}

// resolveUserLanguage looks up the user's app_language config for the given platform.
// Falls back to "en" if not found.
func (s *Service) resolveUserLanguage(email, platform string) string {
	ctx := context.Background()

	// Find the user by email.
	var user models.User
	if err := system.GetStorage().GetByField(ctx, constants.MongoUsersCollection, "email", email, &user); err != nil {
		return "en"
	}

	// Look up the app_language config for this platform.
	var configs []models.UserConfig
	if err := system.GetStorage().GetMany(ctx, constants.MongoUsersConfigurationsCollection, bson.M{
		"user_id":  user.ID,
		"platform": platform,
		"name":     constants.ConfigNameAppLanguage,
	}, &configs); err != nil || len(configs) == 0 {
		return "en"
	}

	if lang, ok := configs[0].Value.(string); ok && lang != "" {
		return lang
	}
	return "en"
}

// handleAdminAlert resolves an AdminAlertEvent (a tiny discriminated union of
// kind + id) by loading the full record from the appropriate collection and
// pushing it into the alert buffer. The buffer deduplicates identical alerts
// within the flush window (ADMIN_ALERT_WINDOW, default 2m) and the flush loop
// delivers them via the AdminNotifier — so 50 identical events in a burst
// become 1 notification that says "[x50]".
//
// We never push HTML or other heavy fields through NATS — the event carries
// only kind + id; the full record is loaded from MongoDB here in signal.
func (s *Service) handleAdminAlert(data []byte) error {
	var evt models.AdminAlertEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal signal.admin_alert: %w", err)
	}
	if evt.ID == "" {
		return fmt.Errorf("admin_alert event missing id")
	}

	ctx := context.Background()

	switch evt.Kind {
	case models.AdminAlertKindError:
		var errDoc models.ServiceError
		if err := system.GetStorage().GetByField(ctx, constants.MongoErrorsCollection, "id", evt.ID, &errDoc); err != nil {
			return fmt.Errorf("load service error %s: %w", evt.ID, err)
		}
		logrus.Infof("[signal] admin alert buffered for error %s (%s)", errDoc.ID, errDoc.ErrorName)
		s.alertBuf.Add(fromError(&errDoc))
		return nil

	case models.AdminAlertKindWarning:
		var warnDoc models.ServiceWarning
		if err := system.GetStorage().GetByField(ctx, constants.MongoWarningsCollection, "id", evt.ID, &warnDoc); err != nil {
			return fmt.Errorf("load service warning %s: %w", evt.ID, err)
		}
		logrus.Infof("[signal] admin alert buffered for warning %s (%s)", warnDoc.ID, warnDoc.WarningName)
		s.alertBuf.Add(fromWarning(&warnDoc))
		return nil

	default:
		return fmt.Errorf("admin_alert event has unknown kind %q", evt.Kind)
	}
}

// handleAdminTestMatch is triggered by a manual admin action. For each admin
// user (resolved from ADMIN_EMAILS → Falcon user by email) it fetches the
// match_result at event.Index (scored_at desc, same order iOS shows) and
// pushes it to their iOS devices, localized per device.
// Does NOT store anything — purely a delivery test against real data.
func (s *Service) handleAdminTestMatch(data []byte) error {
	ctx := context.Background()

	var event models.AdminTestMatchEvent
	if err := json.Unmarshal(data, &event); err != nil {
		logrus.Warnf("[signal] admin test match: unmarshal failed (using index=0): %v", err)
	}
	if event.Index < 0 {
		event.Index = 0
	}

	admins := s.admin.config.List()
	if len(admins) == 0 {
		logrus.Warn("[signal] admin test match: ADMIN_EMAILS is empty, nothing to do")
		return nil
	}

	for _, email := range admins {
		var user models.User
		if err := system.GetStorage().GetByField(ctx, constants.MongoUsersCollection, "email", email, &user); err != nil {
			logrus.Warnf("[signal] admin test match: admin %s is not a Falcon user, skipping", email)
			continue
		}

		// match_result at the given index for THIS admin.
		// FindPage uses 1-indexed pages with pageSize=1 → page=index+1.
		var results []models.MatchResultEvent
		total, err := system.GetStorage().FindPage(ctx, constants.MongoMatchResultsCollection,
			bson.M{"user_id": user.ID}, "scored_at", true, event.Index+1, 1, &results)
		if err != nil {
			logrus.Errorf("[signal] admin test match: fetch match for %s: %v", email, err)
			continue
		}
		if total == 0 || len(results) == 0 {
			logrus.Warnf("[signal] admin test match: admin %s has no match at index %d (total=%d)", email, event.Index, total)
			continue
		}
		match := results[0]

		var tokens []models.IOSDeviceToken
		if err := system.GetStorage().GetAllByField(ctx, constants.MongoIOSDeviceTokensCollection, "user_id", user.ID, &tokens); err != nil {
			logrus.Warnf("[signal] admin test match: fetch tokens for %s: %v", email, err)
			continue
		}
		if len(tokens) == 0 {
			logrus.Warnf("[signal] admin test match: admin %s has no registered iOS devices", email)
			continue
		}

		for _, dt := range tokens {
			lang := s.resolveDeviceLanguage(user.ID, "ios", dt.DeviceID)
			if err := s.apns.Send(ctx, dt.Token, &match, lang); err != nil {
				logrus.Errorf("[signal] admin test match push to %s device %s…: %v", email, dt.Token[:8], err)
				continue
			}
			logrus.Infof("[signal] admin test match sent to %s device %s… (lang=%s, project=%s)", email, dt.Token[:8], lang, match.ProjectID)
		}
	}
	return nil
}

func (s *Service) handleRegisterIOSDeviceToken(data []byte) error {
	var evt models.IOSDeviceTokenRegisterEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal device_token.register: %w", err)
	}

	now := time.Now()
	dt := models.IOSDeviceToken{
		ID:        gonanoid.Must(),
		UserID:    evt.UserID,
		DeviceID:  evt.DeviceID,
		Token:     evt.Token,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ctx := context.Background()
	if err := system.GetStorage().Set(ctx, constants.MongoIOSDeviceTokensCollection,
		map[string]any{"device_id": evt.DeviceID}, dt); err != nil {
		return fmt.Errorf("upsert device token: %w", err)
	}

	logrus.Infof("[signal] registered token %s… for user %s device %s", evt.Token[:8], evt.UserID, evt.DeviceID)
	return nil
}
