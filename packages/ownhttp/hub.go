package ownhttp

import (
	"fmt"
	"net/http"
	"sync"
)

// Hub is a generic fan-out broadcaster for SSE-style endpoints.
// Each subscriber gets a buffered channel that receives a tick
// whenever Broadcast is called. The hub is connection-agnostic:
// callers wire it into an HTTP handler (see ServeSSE) or any other
// transport that can push to subscribers.
//
// Used by dev servers (hot-reload), but kept generic so anything
// that needs "tell every connected client something happened"
// can reuse it without re-implementing the buffered-channel +
// mutex dance.
type Hub struct {
	mu      sync.RWMutex
	clients map[chan struct{}]struct{}
}

// NewHub returns a Hub with no subscribers.
func NewHub() *Hub {
	return &Hub{clients: map[chan struct{}]struct{}{}}
}

// Subscribe registers a new subscriber and returns its channel. The
// caller must Unsubscribe when done (typically via defer in the SSE
// handler) to avoid leaking a closed connection's slot.
func (h *Hub) Subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	h.mu.Lock()
	h.clients[ch] = struct{}{}
	h.mu.Unlock()
	return ch
}

// Unsubscribe removes the subscriber and closes its channel.
func (h *Hub) Unsubscribe(ch chan struct{}) {
	h.mu.Lock()
	delete(h.clients, ch)
	h.mu.Unlock()
	close(ch)
}

// Broadcast sends a tick to every subscriber. Non-blocking — if a
// channel's buffer is full we drop the tick (the subscriber is
// already about to wake from a previous broadcast, the dup is
// harmless).
func (h *Hub) Broadcast() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for ch := range h.clients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// ServeSSE is a ready-made http.HandlerFunc that subscribes the
// caller to the hub and streams `data: <eventName>` lines as
// Server-Sent Events. Use it directly with `mux.HandleFunc("/path",
// hub.ServeSSE("reload"))` for the typical hot-reload case.
//
// Connections live until the request context is cancelled (browser
// closes the tab, page reloads, etc.) — at which point Unsubscribe
// runs via defer and frees the slot.
func (h *Hub) ServeSSE(eventName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming unsupported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Access-Control-Allow-Origin", "*")

		ch := h.Subscribe()
		defer h.Unsubscribe(ch)

		// Initial comment so EventSource fires onopen on the client.
		fmt.Fprintf(w, ": connected\n\n")
		flusher.Flush()

		ctx := r.Context()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ch:
				fmt.Fprintf(w, "data: %s\n\n", eventName)
				flusher.Flush()
			}
		}
	}
}
