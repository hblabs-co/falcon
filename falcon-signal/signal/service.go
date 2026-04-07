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
	apns *apnsClient
	mail *email.Client
}

func newService() (*Service, error) {
	apns, err := newAPNSClient()
	if err != nil {
		return nil, fmt.Errorf("apns client: %w", err)
	}
	return &Service{apns: apns, mail: email.NewClient()}, nil
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

	var tokens []models.DeviceToken
	if err := system.GetStorage().GetAllByField(ctx, constants.MongoDeviceTokensCollection, "user_id", event.UserID, &tokens); err != nil {
		log.Warnf("fetch device tokens for user %s: %v", event.UserID, err)
		return nil
	}
	if len(tokens) == 0 {
		log.Warnf("no device tokens for user %s — skipping push", event.UserID)
		return nil
	}

	var staleTokens []string
	for _, dt := range tokens {
		if err := s.apns.Send(ctx, dt.Token, &event); err != nil {
			if s.apns.IsStaleToken(err) {
				log.Warnf("stale apns token %s…— queued for removal", dt.Token[:8])
				staleTokens = append(staleTokens, dt.Token)
			} else {
				log.Errorf("send push to device %s…: %v", dt.Token[:8], err)
			}
			continue
		}
		log.Infof("push sent to user %s device %s…", event.UserID, dt.Token[:8])
	}

	if len(staleTokens) > 0 {
		if err := system.GetStorage().DeleteManyByFieldIn(ctx, constants.MongoDeviceTokensCollection, "token", staleTokens); err != nil {
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

func (s *Service) handleRegisterToken(data []byte) error {
	var evt models.DeviceTokenRegisterEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal device_token.register: %w", err)
	}

	now := time.Now()
	dt := models.DeviceToken{
		ID:        gonanoid.Must(),
		UserID:    evt.UserID,
		Token:     evt.Token,
		CreatedAt: now,
		UpdatedAt: now,
	}

	ctx := context.Background()
	if err := system.GetStorage().Set(
		ctx,
		constants.MongoDeviceTokensCollection,
		map[string]any{"token": evt.Token},
		dt,
	); err != nil {
		return fmt.Errorf("upsert device token: %w", err)
	}

	logrus.Infof("[signal] registered token %s… for user %s", evt.Token[:8], evt.UserID)
	return nil
}
