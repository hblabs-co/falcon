package cv

import (
	"context"
	"encoding/json"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

func (s *service) subscribe(ctx context.Context) error {
	// cv.prepare.requested uses NATS core request/reply — synchronous RPC.
	if err := system.SubscribeCore(
		constants.SubjectCVPrepareRequested,
		func(data []byte) (any, error) {
			var evt models.CVPrepareRequestedEvent
			if err := json.Unmarshal(data, &evt); err != nil {
				return nil, err
			}
			result, err := s.prepare(context.Background(), evt)
			return result, err
		},
	); err != nil {
		return err
	}
	logrus.Infof("[cv] subscribed (core) → %s", constants.SubjectCVPrepareRequested)

	// cv.index.requested uses JetStream — durable, retryable.
	if err := system.Subscribe(
		ctx,
		constants.StreamStorage,
		"falcon-storage-cv-index",
		constants.SubjectCVIndexRequested,
		func(data []byte) error {
			var evt models.CVIndexRequestedEvent
			if err := json.Unmarshal(data, &evt); err != nil {
				return err
			}
			return s.index(context.Background(), evt)
		},
	); err != nil {
		return err
	}
	logrus.Infof("[cv] subscribed (JetStream) → %s", constants.SubjectCVIndexRequested)

	return nil
}
