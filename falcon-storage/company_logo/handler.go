package company_logo

import (
	"context"
	"encoding/json"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// logoBackoff defines the retry schedule for logo downloads.
// Delivery 1: immediate
// Delivery 2: 5s later
// Delivery 3: 1m later
// Delivery 4: 5m later
// Delivery 5: 20m later
// Delivery 6: 1h later  ← last attempt
var logoBackoff = []time.Duration{
	5 * time.Second,
	1 * time.Minute,
	5 * time.Minute,
	20 * time.Minute,
	1 * time.Hour,
}

func (s *service) subscribe(ctx context.Context) error {
	stream := constants.StreamStorage
	subject := constants.SubjectStorageCompanyLogoRequested
	consumer := "falcon-storage-company-logo"

	err := system.SubscribeWithBackoff(ctx, stream, consumer, subject, logoBackoff,
		func(data []byte, attempt system.RetryAttempt) error {
			var evt models.CompanyLogoDownloadRequestedEvent
			if err := json.Unmarshal(data, &evt); err != nil {
				// Malformed event — ack immediately, never retry.
				logrus.Errorf("[company_logo] unmarshal event: %v (dropping)", err)
				return nil
			}
			return s.handle(context.Background(), evt, attempt)
		},
	)
	if err == nil {
		logrus.Infof("[company_logo] subscribed %s → %s (max %d attempts)", consumer, subject, len(logoBackoff)+1)
	}
	return err
}
