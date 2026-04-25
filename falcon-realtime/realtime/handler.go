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
//
// Implements both system.Module (Register) and system.ShutdownModule
// (Shutdown) — the WS server gets drained gracefully on SIGTERM,
// finishing the close handshake with each connected client instead of
// dropping the TCP socket cold.
type Module struct {
	svc *Service
}

func NewModule() *Module { return &Module{} }

func (m *Module) Register(ctx context.Context) error {
	m.svc = newService()
	if err := m.svc.startHTTP(ctx); err != nil {
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
		m.svc.handleProjectNormalized,
	); err != nil {
		return err
	}
	logrus.Infof("[realtime] subscribed → %s (replica=%s)", constants.SubjectProjectNormalized, replicaID)

	if err := system.Subscribe(
		ctx,
		constants.StreamMatches,
		"falcon-realtime-match-result-"+replicaID,
		constants.SubjectMatchResult,
		m.svc.handleMatchResult,
	); err != nil {
		return err
	}
	logrus.Infof("[realtime] subscribed → %s (replica=%s)", constants.SubjectMatchResult, replicaID)

	if err := system.Subscribe(
		ctx,
		constants.StreamMatches,
		"falcon-realtime-match-flipped-"+replicaID,
		constants.SubjectMatchFlipped,
		m.svc.handleMatchFlipped,
	); err != nil {
		return err
	}
	logrus.Infof("[realtime] subscribed → %s (replica=%s)", constants.SubjectMatchFlipped, replicaID)

	return nil
}

// Shutdown drains the WS server: stops accepting new connections,
// waits for in-flight HTTP handlers (including the WS upgrade) to
// complete, bounded by shutdownCtx. The Service's startHTTP also
// already wires a ctx-cancel goroutine for the same Server, so this
// is a defensive second handle — calling Shutdown twice is safe.
func (m *Module) Shutdown(shutdownCtx context.Context) error {
	if m.svc == nil || m.svc.server == nil {
		return nil
	}
	logrus.Info("[realtime] shutting down WS server")
	return m.svc.server.Shutdown(shutdownCtx)
}
