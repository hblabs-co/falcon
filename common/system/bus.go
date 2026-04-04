package system

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	natsio "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/helpers"
)

var (
	busOnce sync.Once
	bus     jetstream.JetStream
)

// GetBus returns the process-wide JetStream handle.
func GetBus() jetstream.JetStream {
	return bus
}

// Publish marshals payload to JSON and publishes it to subject.
func Publish(ctx context.Context, subject string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = GetBus().Publish(ctx, subject, data)
	return err
}

// InitBus connects to NATS and ensures the given JetStream streams exist.
// Reads NATS_URL from the environment — fatals if not set.
// Callers pass their own stream configs so each service controls its own streams.
func InitBus(streams []jetstream.StreamConfig) {
	busOnce.Do(func() {
		url, err := helpers.ReadEnv("NATS_URL")
		if err != nil {
			logrus.Fatalf("bus init: %v", err)
		}

		nc, err := natsio.Connect(url)
		if err != nil {
			logrus.Fatalf("bus init: connect to %s: %v", url, err)
		}

		js, err := jetstream.New(nc)
		if err != nil {
			logrus.Fatalf("bus init: jetstream: %v", err)
		}

		for _, cfg := range streams {
			if _, err := js.CreateOrUpdateStream(Ctx(), cfg); err != nil {
				logrus.Fatalf("bus init: ensure stream %s: %v", cfg.Name, err)
			}
		}

		bus = js
		logrus.Infof("NATS JetStream connected — %d streams ready", len(streams))
	})
}

// NewBusConfig builds a single-stream JetStream config.
// Services pass the result directly to InitBus — no need to import the jetstream package.
func NewBusConfig(streamName string, subjects ...string) []jetstream.StreamConfig {
	return []jetstream.StreamConfig{
		{
			Name:     streamName,
			Subjects: subjects,
			MaxAge:   24 * time.Hour,
		},
	}
}
