package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/common/constants"
	"hblabs.co/falcon/common/helpers"
	"hblabs.co/falcon/common/models"
	"hblabs.co/falcon/common/system"
)

// Service owns the hub, persists client events, and forwards NATS pushes
// to connected clients. Kept deliberately thin — main.go / handler.go wire
// everything in; Service itself doesn't poll or own its lifecycle.
type Service struct {
	hub    *Hub
	secret string
	server *http.Server
}

func newService() *Service {
	return &Service{
		hub:    NewHub(),
		secret: helpers.ReadEnvOptional("REALTIME_SHARED_SECRET", ""),
	}
}

// startHTTP boots the WS endpoint and a tiny /healthz. Returns once the
// listener is up; serve runs in a background goroutine.
func (s *Service) startHTTP(ctx context.Context) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/ws", s.serveWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		stats := s.hub.Stats()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":          true,
			"users":       stats.UniqueUsers,
			"connections": stats.Connections,
		})
	})

	port := helpers.ReadEnvOptional("REALTIME_PORT", "8090")
	s.server = &http.Server{
		Addr:              ":" + port,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = s.server.Shutdown(shutdownCtx)
	}()

	go func() {
		logrus.Infof("[realtime] WS listening on :%s/ws", port)
		if err := s.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logrus.Errorf("[realtime] http server: %v", err)
		}
	}()
	return nil
}

// incomingEvent is the internal shape before it becomes a RealtimeEvent
// document. Kept separate so the wire format can evolve without forcing
// model changes.
type incomingEvent struct {
	Event      string
	UserID     string
	DeviceID   string
	Platform   string
	IP         string
	OS         string
	OSVersion  string
	AppVersion string
	Metadata   map[string]any
}

// persistEvent writes one doc to realtime_stats and echoes it onto
// NATS for any downstream consumer that wants a copy. Publish failures
// log and swallow — stats are best-effort; failing to notify a downstream
// should not lose the already-persisted record.
func (s *Service) persistEvent(ctx context.Context, evt incomingEvent) {
	if evt.Event == "" || evt.DeviceID == "" {
		return
	}
	doc := models.RealtimeEvent{
		ID:         gonanoid.Must(),
		Event:      evt.Event,
		UserID:     evt.UserID,
		DeviceID:   evt.DeviceID,
		Platform:   evt.Platform,
		OS:         evt.OS,
		OSVersion:  evt.OSVersion,
		AppVersion: evt.AppVersion,
		IP:         evt.IP,
		Metadata:   evt.Metadata,
		CreatedAt:  time.Now(),
	}
	if err := system.GetStorage().Insert(ctx, constants.MongoRealtimeStatsCollection, doc); err != nil {
		logrus.Warnf("[realtime] persist %s: %v", evt.Event, err)
	}
	if err := system.Publish(ctx, constants.SubjectRealtimeEvent, doc); err != nil {
		logrus.Debugf("[realtime] publish %s: %v", evt.Event, err)
	}
}

// handleProjectNormalized broadcasts a "new project" push to every
// connected client on this replica. Project normalisation isn't bound
// to a specific user, so this is a fan-out to all — the client decides
// whether the project is relevant (e.g. matches user's profile).
func (s *Service) handleProjectNormalized(data []byte) error {
	var evt models.ProjectNormalizedEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal project.normalized: %w", err)
	}
	env := envelope{
		Type:    "project.normalized",
		Payload: evt,
	}
	s.broadcastAll(env)
	return nil
}

// handleMatchResult forwards the match push to the specific user's
// connections on this replica. Other replicas that don't hold a live
// connection for this user simply deliver to 0 clients — cheap no-op.
func (s *Service) handleMatchResult(data []byte) error {
	var evt models.MatchResultEvent
	if err := json.Unmarshal(data, &evt); err != nil {
		return fmt.Errorf("unmarshal match.result: %w", err)
	}
	s.hub.BroadcastToUser(evt.UserID, envelope{
		Type:    "match.result",
		Payload: evt,
	})
	return nil
}

// broadcastAll sends the frame to every connection, regardless of user.
// Used for events that concern all users (e.g. new normalized project).
func (s *Service) broadcastAll(env envelope) {
	data, err := json.Marshal(env)
	if err != nil {
		logrus.Errorf("realtime: marshal broadcast: %v", err)
		return
	}
	s.hub.mu.RLock()
	defer s.hub.mu.RUnlock()
	for _, set := range s.hub.byUser {
		for c := range set {
			c.send(data)
		}
	}
}
