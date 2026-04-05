package logo

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Subscribe registers a durable NATS consumer for storage.logo.requested.
func (s *Service) Subscribe() error {
	stream := constants.StreamStorage
	subject := constants.SubjectStorageCompanyLogoRequested
	consumerName := "falcon-storage-logo"

	err := system.Subscribe(
		system.Ctx(),
		stream,
		consumerName,
		subject,
		func(data []byte) error {
			var evt models.CompanyLogoDownloadRequestedEvent
			if err := json.Unmarshal(data, &evt); err != nil {
				return err
			}
			return s.handleDownloadLogo(context.Background(), evt)
		},
	)

	if err == nil {
		logrus.Infof("consumer %s subscribed to %s.%s", consumerName, stream, subject)
	}

	return err
}
