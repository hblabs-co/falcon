package ownhttp

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

// wsUpgrader is the default websocket upgrader used by ServeWS. It is
// permissive on Origin because current callers (falcon-nest portal)
// are local-dev tools where the browser's origin matches the server.
// Production callers should wrap or replace this check.
var wsUpgrader = websocket.Upgrader{
	CheckOrigin:     func(r *http.Request) bool { return true },
	ReadBufferSize:  1 << 12,
	WriteBufferSize: 1 << 12,
}

const (
	wsWriteDeadline = 10 * time.Second
	wsPingInterval  = 30 * time.Second
	wsReadDeadline  = 90 * time.Second
)

// ServeWS is the websocket counterpart to ServeSSE. It upgrades the
// request, subscribes to the hub, and writes `payloadFn()` as JSON to
// the client on every broadcast. An initial payload is sent on connect
// so the UI doesn't wait a full poll interval for its first paint.
//
// Why WS and not just SSE everywhere: SSE is ideal for dev (auto-
// reconnect, zero deps) but many cluster ingresses strip or buffer
// `text/event-stream`; WS travels through the Upgrade path and is
// better-supported end-to-end. ServeSSE stays the right choice for
// hot-reload (local-only); ServeWS is the right choice for anything
// that has to survive a production ingress.
//
// A 30s ping keeps idle ingress timeouts (nginx default 60s) from
// killing the connection on quiet channels.
func (h *Hub) ServeWS(payloadFn func() any) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := wsUpgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close()

		ch := h.Subscribe()
		defer h.Unsubscribe(ch)

		writeJSON := func(v any) error {
			_ = conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
			return conn.WriteJSON(v)
		}

		if err := writeJSON(payloadFn()); err != nil {
			return
		}

		// Read loop runs in the background to (a) notice disconnects and
		// (b) let gorilla's default pong handler reset the read deadline.
		// Without a reader we'd never see the peer hang up.
		_ = conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
		conn.SetPongHandler(func(string) error {
			return conn.SetReadDeadline(time.Now().Add(wsReadDeadline))
		})
		done := make(chan struct{})
		go func() {
			defer close(done)
			for {
				if _, _, err := conn.ReadMessage(); err != nil {
					return
				}
			}
		}()

		ping := time.NewTicker(wsPingInterval)
		defer ping.Stop()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-done:
				return
			case <-ch:
				if err := writeJSON(payloadFn()); err != nil {
					return
				}
			case <-ping.C:
				_ = conn.SetWriteDeadline(time.Now().Add(wsWriteDeadline))
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(wsWriteDeadline)); err != nil {
					return
				}
			}
		}
	}
}
