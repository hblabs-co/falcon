package realtime

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/environment"
)

// Client is a single websocket connection. The struct is intentionally
// small — lifecycle (register → read loop → unregister) is driven from
// serveWS below. Every send goes through `send()` which serialises writes
// via outbox, because concurrent WriteMessage calls on a gorilla/websocket
// conn are not safe.
type Client struct {
	UserID   string
	DeviceID string
	Platform string
	IP       string

	conn   *websocket.Conn
	outbox chan []byte
	once   sync.Once
	done   chan struct{}
}

// send enqueues an already-marshalled frame. Drops silently if the outbox
// is full — a slow/stuck consumer must not backpressure the hub. The
// close(done) check prevents "send on closed channel" panics when a
// broadcast races with unregister.
func (c *Client) send(data []byte) {
	select {
	case <-c.done:
		return
	default:
	}
	select {
	case c.outbox <- data:
	default:
		logrus.Warnf("realtime: outbox full for device=%s user=%s — dropping frame", c.DeviceID, c.UserID)
	}
}

// envelope is the wire format for every server → client message.
// Keeping a fixed shape lets the client dispatch on "type" without
// having to sniff the payload.
type envelope struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

const (
	writeDeadline = 10 * time.Second
	readDeadline  = 90 * time.Second // aligns with client ping cadence (30s)
	pingInterval  = 30 * time.Second
	outboxSize    = 32
)

var upgrader = websocket.Upgrader{
	// The WS endpoint is public in the sense that we don't gate it with a
	// login session — the HMAC handshake is the only access control. That
	// does NOT mean we accept arbitrary Origin headers though: web clients
	// running in a browser must come from an approved origin. Native clients
	// typically send no Origin header, which we allow.
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true
		}
		allowed := allowedOrigins()
		if len(allowed) == 0 {
			return true
		}
		for _, a := range allowed {
			if a == origin {
				return true
			}
		}
		return false
	},
	ReadBufferSize:  1 << 12,
	WriteBufferSize: 1 << 12,
}

// allowedOrigins is cached so we don't re-read the env on every handshake.
var originCache struct {
	once sync.Once
	list []string
}

func allowedOrigins() []string {
	originCache.once.Do(func() {
		v := strings.TrimSpace(environment.ReadOptional("REALTIME_ALLOWED_ORIGINS", ""))
		if v == "" {
			return
		}
		for _, o := range strings.Split(v, ",") {
			o = strings.TrimSpace(o)
			if o != "" {
				originCache.list = append(originCache.list, o)
			}
		}
	})
	return originCache.list
}

// serveWS upgrades the HTTP request and runs the full lifecycle of a
// single client connection until it disconnects or the ctx is cancelled.
func (s *Service) serveWS(w http.ResponseWriter, r *http.Request) {
	// Headers over query string: query strings get logged by reverse
	// proxies/CDNs; custom headers usually don't. `ts` is a unix timestamp
	// (seconds); the server window is ±5min.
	ts := r.Header.Get("X-Falcon-Timestamp")
	deviceID := r.Header.Get("X-Falcon-Device-ID")
	platform := r.Header.Get("X-Falcon-Platform")
	userID := r.Header.Get("X-Falcon-User-ID") // optional — empty until user logs in
	sig := r.Header.Get("X-Falcon-Signature")

	if err := VerifyHandshake(s.secret, ts, deviceID, platform, sig); err != nil {
		logrus.Warnf("realtime: handshake rejected from %s: %v", clientIP(r), err)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		// Upgrader has already written the error response — nothing else to do.
		return
	}

	client := &Client{
		UserID:   userID,
		DeviceID: deviceID,
		Platform: platform,
		IP:       clientIP(r),
		conn:     conn,
		outbox:   make(chan []byte, outboxSize),
		done:     make(chan struct{}),
	}
	s.hub.Register(client)
	connectedAt := time.Now()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Server-authoritative connection lifecycle. Unlike client-emitted
	// app_backgrounded/app_terminated (which iOS drops in most "kill"
	// scenarios), these events ALWAYS fire — worst case the read-deadline
	// eventually closes the socket and triggers device_offline. This is
	// the reliable source of truth for "was the user online at time X?".
	s.persistEvent(ctx, incomingEvent{
		Event:    "device_online",
		UserID:   client.UserID,
		DeviceID: client.DeviceID,
		Platform: client.Platform,
		IP:       client.IP,
	})

	// Writer owns the conn for writes; reader handles pongs + inbound
	// events. Both goroutines coordinate via client.done (closed once).
	go s.writeLoop(ctx, client)
	s.readLoop(ctx, client)

	client.once.Do(func() { close(client.done) })
	s.hub.Unregister(client)
	_ = conn.Close()

	s.persistEvent(context.Background(), incomingEvent{
		Event:    "device_offline",
		UserID:   client.UserID,
		DeviceID: client.DeviceID,
		Platform: client.Platform,
		IP:       client.IP,
		Metadata: map[string]any{
			"duration_ms": time.Since(connectedAt).Milliseconds(),
		},
	})
}

// readLoop handles pong responses, ping requests (resetting the read
// deadline on either), and inbound client events. A bad frame closes
// the connection — we don't try to recover a protocol-confused client.
//
// Why handle PingHandler specifically: iOS URLSessionWebSocketTask
// doesn't reliably reply to server-sent pings, so the canonical
// "server pings client, client pongs" pattern fails and the read
// deadline expires ~every 90s, closing the connection and producing
// a steady stream of device_online events. Having the client ping
// US (which iOS does correctly) and resetting the deadline here
// avoids that churn.
func (s *Service) readLoop(ctx context.Context, c *Client) {
	_ = c.conn.SetReadDeadline(time.Now().Add(readDeadline))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(readDeadline))
	})
	c.conn.SetPingHandler(func(msg string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(readDeadline))
		// Mirror gorilla's default pong-reply behaviour so the client
		// knows we're alive. Error ignored on timeout / already-closed
		// — either way the read loop will exit on the next iteration.
		_ = c.conn.WriteControl(websocket.PongMessage, []byte(msg), time.Now().Add(writeDeadline))
		return nil
	})

	for {
		if ctx.Err() != nil {
			return
		}
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			if !websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway) {
				logrus.Debugf("realtime: read from device=%s: %v", c.DeviceID, err)
			}
			return
		}

		var msg struct {
			Event    string         `json:"event"`
			Metadata map[string]any `json:"metadata,omitempty"`
			// Optional overrides — clients may send these on session_started.
			OS         string `json:"os,omitempty"`
			OSVersion  string `json:"os_version,omitempty"`
			AppVersion string `json:"app_version,omitempty"`
			// Mutable user binding: set on user_bind (login), cleared on
			// user_unbind (logout). Takes effect on this connection for
			// subsequent broadcasts. For other events, it's ignored
			// (the current c.UserID from the hub wins).
			UserID string `json:"user_id,omitempty"`
		}
		if err := json.Unmarshal(data, &msg); err != nil {
			logrus.Warnf("realtime: bad frame from device=%s: %v", c.DeviceID, err)
			return
		}

		// user_bind / user_unbind: rebind the hub in place so pushes go
		// to the right bucket, without tearing down the socket. We also
		// persist user_bound / user_unbound as analytics events — they
		// capture the auth transition separately from device lifecycle.
		switch msg.Event {
		case "user_bind":
			s.hub.Rebind(c, msg.UserID)
			s.persistEvent(ctx, incomingEvent{
				Event:    "user_bound",
				UserID:   msg.UserID,
				DeviceID: c.DeviceID,
				Platform: c.Platform,
				IP:       c.IP,
			})
			continue
		case "user_unbind":
			s.hub.Rebind(c, "")
			s.persistEvent(ctx, incomingEvent{
				Event:    "user_unbound",
				UserID:   "", // socket is now unbound
				DeviceID: c.DeviceID,
				Platform: c.Platform,
				IP:       c.IP,
			})
			continue
		}

		s.persistEvent(ctx, incomingEvent{
			Event:      msg.Event,
			UserID:     c.UserID, // current bound user, not whatever the client sends
			DeviceID:   c.DeviceID,
			Platform:   c.Platform,
			IP:         c.IP,
			OS:         msg.OS,
			OSVersion:  msg.OSVersion,
			AppVersion: msg.AppVersion,
			Metadata:   msg.Metadata,
		})
	}
}

// writeLoop pumps outbound frames to the peer and keeps the connection
// alive with periodic pings. The select guarantees ordered, single-writer
// access to the underlying conn.
func (s *Service) writeLoop(ctx context.Context, c *Client) {
	ticker := time.NewTicker(pingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-c.done:
			return
		case data := <-c.outbox:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := c.conn.WriteMessage(websocket.TextMessage, data); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeDeadline))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// clientIP resolves the originating IP, honouring X-Forwarded-For / X-Real-IP
// so replicas behind a load balancer still log the real client address.
// Trusts headers blindly — fine for stats, don't use for authz.
func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First hop is the origin; subsequent are proxies.
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xr := r.Header.Get("X-Real-IP"); xr != "" {
		return xr
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
