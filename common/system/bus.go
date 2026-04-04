package system

import (
	"context"
	"encoding/json"
	"fmt"
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

// Subscribe creates a durable JetStream consumer and dispatches messages to
// handler in a background goroutine. handler receives the raw JSON bytes;
// returning an error nacks the message so it will be redelivered.
func Subscribe(ctx context.Context, stream, consumer, subject string, handler func([]byte) error) error {
	cons, err := GetBus().CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Name:          consumer,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return fmt.Errorf("create consumer %s: %w", consumer, err)
	}

	msgs, err := cons.Messages()
	if err != nil {
		return fmt.Errorf("consumer %s messages: %w", consumer, err)
	}

	go func() {
		for {
			msg, err := msgs.Next()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				logrus.Errorf("consumer %s: next: %v", consumer, err)
				continue
			}
			if err := handler(msg.Data()); err != nil {
				logrus.Errorf("consumer %s: handler: %v", consumer, err)
				_ = msg.Nak()
			} else {
				_ = msg.Ack()
			}
		}
	}()

	return nil
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
