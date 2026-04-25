package realtime

import (
	"context"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/constants"
	"hblabs.co/falcon/packages/environment"
	"hblabs.co/falcon/packages/system"
)

// Module wires realtime into falcon-realtime. Each replica gets its own
// consumer name suffix via REALTIME_REPLICA_ID so NATS fans out match
// and project events to every replica (not just one) — required so a
// user connected to replica B still receives a match for their user_id.
type Module struct{}

func NewModule() *Module { return &Module{} }

func (m *Module) Register(ctx context.Context) error {
	svc := newService()
	if err := svc.startHTTP(ctx); err != nil {
		return err
	}

	// Per-replica consumer names ensure every replica independently receives
	// every message. Without this, JetStream would load-balance across
	// replicas and only one instance would get each event — broken for
	// realtime fan-out since a user may be connected to any replica.
	replicaID := environment.ReadOptional("REALTIME_REPLICA_ID", "default")

	if err := system.Subscribe(
		ctx,
		constants.StreamProjects,
		"falcon-realtime-project-normalized-"+replicaID,
		constants.SubjectProjectNormalized,
		svc.handleProjectNormalized,
	); err != nil {
		return err
	}
	logrus.Infof("[realtime] subscribed → %s (replica=%s)", constants.SubjectProjectNormalized, replicaID)

	if err := system.Subscribe(
		ctx,
		constants.StreamMatches,
		"falcon-realtime-match-result-"+replicaID,
		constants.SubjectMatchResult,
		svc.handleMatchResult,
	); err != nil {
		return err
	}
	logrus.Infof("[realtime] subscribed → %s (replica=%s)", constants.SubjectMatchResult, replicaID)

	if err := system.Subscribe(
		ctx,
		constants.StreamMatches,
		"falcon-realtime-match-flipped-"+replicaID,
		constants.SubjectMatchFlipped,
		svc.handleMatchFlipped,
	); err != nil {
		return err
	}
	logrus.Infof("[realtime] subscribed → %s (replica=%s)", constants.SubjectMatchFlipped, replicaID)

	return nil
}
