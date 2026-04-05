package signal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Service consumes match.result events and delivers iOS push notifications.
type Service struct {
	apns *apnsClient
}

func NewService() (*Service, error) {
	apns, err := newAPNSClient()
	if err != nil {
		return nil, fmt.Errorf("apns client: %w", err)
	}
	return &Service{apns: apns}, nil
}

// Run subscribes to match.result and starts the HTTP server. Blocks until exit.
func (s *Service) Run() error {
	if err := system.Subscribe(
		system.Ctx(),
		constants.StreamMatches,
		"signal",
		constants.SubjectMatchResult,
		s.handleMatchResult,
	); err != nil {
		return fmt.Errorf("subscribe match.result: %w", err)
	}
	logrus.Infof("subscribed to %s", constants.SubjectMatchResult)

	port := helpers.ReadEnvOptional("PORT", "8083")

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(ginLogger())
	r.SetTrustedProxies(nil)

	routes(r)
	return r.Run(":" + port)
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
		log.Warnf("could not fetch device tokens for user %s: %v", event.UserID, err)
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
				log.Warnf("stale apns token for device %s… — queued for removal", dt.Token[:8])
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
