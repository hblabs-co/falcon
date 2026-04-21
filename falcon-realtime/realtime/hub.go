package realtime

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// Each replica runs its own Hub. A client connects to one replica (via load
// balancer) and only that replica knows about the connection. When NATS
// delivers a message targeted at a user, only the replica(s) holding a live
// connection for them forward it — others no-op. This avoids any cross-replica
// coordination at the cost of NATS fan-out (cheap: one subject, many subs).
type Hub struct {
	mu      sync.RWMutex
	byUser  map[string]map[*Client]struct{} // user_id → connections
	onClose func(*Client)                   // invoked AFTER a client is fully removed
}

func NewHub() *Hub {
	return &Hub{byUser: make(map[string]map[*Client]struct{})}
}

func (h *Hub) OnClose(fn func(*Client)) { h.onClose = fn }

// Register adds the client to the hub. Safe to call multiple times for the
// same client (idempotent — prevents leaks if a reconnect races with the
// close cleanup). Registering with an empty user_id still works: the
// connection won't receive user-targeted broadcasts but is tracked for
// concurrent-connection stats.
func (h *Hub) Register(c *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	set, ok := h.byUser[c.UserID]
	if !ok {
		set = make(map[*Client]struct{})
		h.byUser[c.UserID] = set
	}
	set[c] = struct{}{}
}

func (h *Hub) Unregister(c *Client) {
	h.mu.Lock()
	set, ok := h.byUser[c.UserID]
	if ok {
		delete(set, c)
		if len(set) == 0 {
			delete(h.byUser, c.UserID)
		}
	}
	h.mu.Unlock()
	if h.onClose != nil {
		h.onClose(c)
	}
}

// Rebind moves the client between user buckets without closing the socket.
// Used when the client sends a "user_bind"/"user_unbind" frame after login/
// logout — avoids the expensive disconnect+reconnect churn and keeps exactly
// one device_online event per actual connection lifecycle. The client's
// UserID field is updated so subsequent BroadcastToUser calls target the new
// bucket.
func (h *Hub) Rebind(c *Client, newUserID string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if c.UserID == newUserID {
		return
	}
	if old, ok := h.byUser[c.UserID]; ok {
		delete(old, c)
		if len(old) == 0 {
			delete(h.byUser, c.UserID)
		}
	}
	c.UserID = newUserID
	set, ok := h.byUser[newUserID]
	if !ok {
		set = make(map[*Client]struct{})
		h.byUser[newUserID] = set
	}
	set[c] = struct{}{}
}

// BroadcastToUser sends a JSON frame to every active connection of the user.
// Returns the number of clients the message was delivered to (0 = user not
// online on this replica, nothing to do).
func (h *Hub) BroadcastToUser(userID string, envelope any) int {
	if userID == "" {
		return 0
	}
	data, err := json.Marshal(envelope)
	if err != nil {
		logrus.Errorf("hub: marshal envelope: %v", err)
		return 0
	}

	h.mu.RLock()
	set := h.byUser[userID]
	targets := make([]*Client, 0, len(set))
	for c := range set {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		c.send(data)
	}
	return len(targets)
}

// Stats returns a snapshot used by /healthz and the
// concurrent-connections observability path.
func (h *Hub) Stats() HubStats {
	h.mu.RLock()
	defer h.mu.RUnlock()

	stats := HubStats{
		UniqueUsers: len(h.byUser),
		Collected:   time.Now(),
	}
	for _, set := range h.byUser {
		stats.Connections += len(set)
	}
	return stats
}

type HubStats struct {
	UniqueUsers int
	Connections int
	Collected   time.Time
}
