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
	"hblabs.co/falcon/common/helpers"
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

	// Total matches for this user — shown in the Live Activity header.
	// Uses the canonical visible-match filter (helpers.VisibleMatchFilter)
	// so the Lock Screen counter never drifts above what the app lists.
	// Mismatch here was the cause of "13 Treffer on Lock Screen but only
	// 8 in the app" reports: we used to Count() raw user_id, which
	// included sub-threshold scores and freelance.de listings that the
	// /matches endpoint filters out.
	totalMatches64, _ := system.GetStorage().Count(ctx, constants.MongoMatchResultsCollection,
		helpers.VisibleMatchFilter(event.UserID))
	totalMatches := int(totalMatches64)

	var staleTokens []string
	for _, dt := range tokens {
		lang := s.resolveDeviceLanguage(event.UserID, "ios", dt.DeviceID)

		// Priority: UPDATE an existing activity if we have its update token.
		// On success → next; on failure (activity ended) → fall through to
		// push-to-start which creates a fresh activity.
		if dt.LiveActivityUpdateToken != "" {
			if err := s.apns.SendMatchLiveActivityUpdate(ctx, dt.LiveActivityUpdateToken, &event, lang, totalMatches); err == nil {
				log.Infof("liveactivity UPDATE sent to user %s device %s… (lang=%s, total=%d)", event.UserID, dt.Token[:8], lang, totalMatches)
				continue
			} else {
				log.Warnf("liveactivity update failed for device %s…: %v — trying start", dt.Token[:8], err)
			}
		}

		// iOS 17.2+ with a push-to-start token: starts a new activity.
		if dt.LiveActivityToken != "" {
			if err := s.apns.SendMatchLiveActivityStart(ctx, dt.LiveActivityToken, &event, lang, totalMatches); err != nil {
				log.Warnf("liveactivity start failed for device %s…: %v — falling back to regular push", dt.Token[:8], err)
			} else {
				log.Infof("liveactivity START+push sent to user %s device %s… (lang=%s, total=%d)", event.UserID, dt.Token[:8], lang, totalMatches)
				continue
			}
		}

		if err := s.apns.Send(ctx, dt.Token, &event, lang, totalMatches); err != nil {
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

		// Total matches for this admin (for the Live Activity header).
		// Same canonical filter as the production path so test pushes
		// agree with the app UI too.
		totalMatches64, _ := system.GetStorage().Count(ctx, constants.MongoMatchResultsCollection,
			helpers.VisibleMatchFilter(user.ID))
		totalMatches := int(totalMatches64)

		for _, dt := range tokens {
			lang := s.resolveDeviceLanguage(user.ID, "ios", dt.DeviceID)

			// Try update → start → regular push, mirroring handleMatchResult.
			if dt.LiveActivityUpdateToken != "" {
				if err := s.apns.SendMatchLiveActivityUpdate(ctx, dt.LiveActivityUpdateToken, &match, lang, totalMatches); err == nil {
					logrus.Infof("[signal] admin test liveactivity UPDATE sent to %s device %s… (lang=%s, project=%s, total=%d)", email, dt.Token[:8], lang, match.ProjectID, totalMatches)
					continue
				} else {
					logrus.Warnf("[signal] admin test update failed for %s device %s…: %v — trying start", email, dt.Token[:8], err)
				}
			}
			if dt.LiveActivityToken != "" {
				if err := s.apns.SendMatchLiveActivityStart(ctx, dt.LiveActivityToken, &match, lang, totalMatches); err != nil {
					logrus.Warnf("[signal] admin test start failed for %s device %s…: %v — falling back", email, dt.Token[:8], err)
				} else {
					logrus.Infof("[signal] admin test liveactivity START+push sent to %s device %s… (lang=%s, project=%s, total=%d)", email, dt.Token[:8], lang, match.ProjectID, totalMatches)
					continue
				}
			}

			if err := s.apns.Send(ctx, dt.Token, &match, lang, totalMatches); err != nil {
				logrus.Errorf("[signal] admin test match push to %s device %s…: %v", email, dt.Token[:8], err)
				continue
			}
			logrus.Infof("[signal] admin test match sent to %s device %s… (lang=%s, project=%s)", email, dt.Token[:8], lang, match.ProjectID)
		}
	}
	return nil
}

// handleLiveActivityUpdateToken upserts (or clears) the per-activity update
// token on a device's IOSDeviceToken record. Signal uses this to send
// event="update" pushes and refresh the existing activity instead of creating
// a new one each time.
func (s *Service) handleLiveActivityUpdateToken(data []byte) error {
	var evt models.IOSLiveActivityUpdateTokenEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal live_activity_update_token: %w", err)
	}
	if evt.DeviceID == "" {
		return fmt.Errorf("live_activity_update_token event missing device_id")
	}

	ctx := context.Background()
	if err := system.GetStorage().Set(ctx, constants.MongoIOSDeviceTokensCollection,
		bson.M{"device_id": evt.DeviceID},
		bson.M{"live_activity_update_token": evt.Token, "updated_at": time.Now()}); err != nil {
		return fmt.Errorf("update live_activity_update_token: %w", err)
	}

	action := "set"
	if evt.Token == "" {
		action = "cleared"
	}
	logrus.Infof("[signal] live_activity_update_token %s for device %s", action, evt.DeviceID)
	return nil
}

// handleLogoutIOSDeviceToken unbinds a device from its user. The APNs
// device token itself is a device attribute (doesn't change on logout),
// so we keep it. We clear user_id + live_activity_token +
// live_activity_update_token — signal's match lookup is
// GetAllByField("user_id"), so no user_id means no pushes. Re-login
// calls /device-token which upserts by device_id and restores the
// binding. Idempotent: unmatched filter is a no-op, not an error.
func (s *Service) handleLogoutIOSDeviceToken(data []byte) error {
	var evt models.IOSDeviceTokenLogoutEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal device_token.logout: %w", err)
	}
	if evt.DeviceID == "" {
		return fmt.Errorf("device_token.logout event missing device_id")
	}

	ctx := context.Background()
	filter := bson.M{"device_id": evt.DeviceID}
	if evt.UserID != "" {
		// Scoping by user_id protects the row if somehow a different
		// user already re-registered on this device before this event
		// was processed (rare, but NATS is async).
		filter["user_id"] = evt.UserID
	}
	update := bson.M{"$set": bson.M{
		"user_id":                    "",
		"live_activity_token":        "",
		"live_activity_update_token": "",
		"updated_at":                 time.Now(),
	}}
	if _, err := system.GetStorage().BulkUpdate(ctx, constants.MongoIOSDeviceTokensCollection, filter, update); err != nil {
		return fmt.Errorf("unbind token for device %s: %w", evt.DeviceID, err)
	}

	logrus.Infof("[signal] logout: unbound device %s (user_id=%s)", evt.DeviceID, evt.UserID)
	return nil
}

func (s *Service) handleRegisterIOSDeviceToken(data []byte) error {
	var evt models.IOSDeviceTokenRegisterEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal device_token.register: %w", err)
	}

	now := time.Now()
	dt := models.IOSDeviceToken{
		ID:                gonanoid.Must(),
		UserID:            evt.UserID,
		DeviceID:          evt.DeviceID,
		Token:             evt.Token,
		LiveActivityToken: evt.LiveActivityToken,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	ctx := context.Background()
	if err := system.GetStorage().Set(ctx, constants.MongoIOSDeviceTokensCollection,
		map[string]any{"device_id": evt.DeviceID}, dt); err != nil {
		return fmt.Errorf("upsert device token: %w", err)
	}

	laSuffix := "no live_activity_token"
	if evt.LiveActivityToken != "" {
		laSuffix = fmt.Sprintf("live_activity_token=%s…", evt.LiveActivityToken[:min(16, len(evt.LiveActivityToken))])
	}
	logrus.Infof("[signal] registered token %s… for user %s device %s (%s)", evt.Token[:8], evt.UserID, evt.DeviceID, laSuffix)
	return nil
}
