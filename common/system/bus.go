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
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
)

var (
	busOnce sync.Once
	natConn *natsio.Conn
	bus     jetstream.JetStream
)

// GetBus returns the process-wide JetStream handle.
func GetBus() jetstream.JetStream { return bus }

// GetConn returns the underlying NATS connection.
// Needed for core NATS request/reply and non-JetStream subscriptions.
func GetConn() *natsio.Conn { return natConn }

// Publish marshals payload to JSON and publishes it to subject via JetStream.
func Publish(ctx context.Context, subject string, payload any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = GetBus().Publish(ctx, subject, data)
	return err
}

// Request sends payload to subject using NATS core request/reply and
// JSON-unmarshals the response into result (if non-nil).
// Use this for synchronous RPC calls where the responder is not a JetStream consumer.
func Request(ctx context.Context, subject string, payload any, result any) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg, err := natConn.RequestWithContext(ctx, subject, data)
	if err != nil {
		return err
	}
	if result != nil {
		return json.Unmarshal(msg.Data, result)
	}
	return nil
}

// SubscribeCore registers a NATS core (non-JetStream) subscription.
// handler receives the request data and returns a response payload (JSON-marshalled)
// and an error. If the incoming message has a reply-to subject and handler returns
// no error, the response is sent there automatically.
// Returns an error indicating that the subscription itself could not be created.
func SubscribeCore(subject string, handler func(data []byte) (any, error)) error {
	_, err := natConn.Subscribe(subject, func(msg *natsio.Msg) {
		result, err := handler(msg.Data)
		if err != nil {
			logrus.Errorf("core subscriber %s: %v", subject, err)
			return
		}
		if msg.Reply == "" || result == nil {
			return
		}
		resp, err := json.Marshal(result)
		if err != nil {
			logrus.Errorf("core subscriber %s: marshal reply: %v", subject, err)
			return
		}
		if err := natConn.Publish(msg.Reply, resp); err != nil {
			logrus.Errorf("core subscriber %s: send reply: %v", subject, err)
		}
	})
	return err
}

// InitBus connects to NATS and ensures the given JetStream streams exist.
// Reads NATS_URL from the environment — fatals if not set.
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

		natConn = nc
		bus = js
		logrus.Infof("NATS JetStream connected — %d streams ready", len(streams))
	})
}

// Subscribe creates a durable JetStream consumer and dispatches messages to
// handler in a background goroutine. Returning an error nacks the message.
func Subscribe(ctx context.Context, stream, consumer, subject string, handler func([]byte) error) error {
	cons, err := GetBus().CreateOrUpdateConsumer(ctx, stream, jetstream.ConsumerConfig{
		Name:          consumer,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		DeliverPolicy: jetstream.DeliverNewPolicy,
		AckWait:       10 * time.Minute,
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
func NewBusConfig(streamName string, subjects ...string) []jetstream.StreamConfig {
	return []jetstream.StreamConfig{
		{
			Name:     streamName,
			Subjects: subjects,
			MaxAge:   24 * time.Hour,
		},
	}
}

func MergeBusConfigs(configs ...[]jetstream.StreamConfig) []jetstream.StreamConfig {
	var result []jetstream.StreamConfig
	for _, cfg := range configs {
		result = append(result, cfg...)
	}
	return result
}

// Canonical stream configs — always declare the FULL subject list.
// CreateOrUpdateStream replaces the config, so partial declarations from one
// service would silently remove subjects declared by another. Using these
// ensures every service agrees on the complete definition regardless of start order.

func StreamProjects() []jetstream.StreamConfig {
	return NewBusConfig(
		constants.StreamProjects,
		constants.SubjectProjectCreated,
		constants.SubjectProjectUpdated,
		constants.SubjectProjectNormalized,
	)
}

func StreamMatches() []jetstream.StreamConfig {
	return NewBusConfig(
		constants.StreamMatches,
		constants.SubjectMatchPending,
		constants.SubjectMatchResult,
	)
}

func StreamScrape() []jetstream.StreamConfig {
	return NewBusConfig(
		constants.StreamScrape,
		constants.SubjectScrapeRequested+".>",
		constants.SubjectScrapeFailed,
	)
}

func StreamStorage() []jetstream.StreamConfig {
	return NewBusConfig(
		constants.StreamStorage,
		constants.SubjectStorageCompanyLogoRequested,
		constants.SubjectStorageCompanyLogoDownloaded,
		constants.SubjectCVIndexRequested,
		constants.SubjectCVIndexed,
	)
}

func StreamSignal() []jetstream.StreamConfig {
	return NewBusConfig(
		constants.StreamSignal,
		constants.SubjectSignalDeviceTokenRegister,
	)
}
