package company_logo

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

func (s *service) subscribe(ctx context.Context) error {
	stream := constants.StreamStorage
	subject := constants.SubjectStorageCompanyLogoRequested
	consumer := "falcon-storage-company-logo"

	err := system.Subscribe(ctx, stream, consumer, subject, func(data []byte) error {
		var evt models.CompanyLogoDownloadRequestedEvent
		if err := json.Unmarshal(data, &evt); err != nil {
			return err
		}
		return s.handle(context.Background(), evt)
	})
	if err == nil {
		logrus.Infof("[company_logo] subscribed %s → %s", consumer, subject)
	}
	return err
}
