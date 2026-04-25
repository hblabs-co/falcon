package fswatch

import (
	"bytes"
	"context"
	"log"
	"net/http"
	"time"
)

// reloadScript is the JS served at /__reload.js. Subscribes to the
// SSE endpoint and calls location.reload() on every message. Kept
// inline as a const so downstream tools don't need to ship an
// accompanying static file.
const reloadScript = `(function () {
  var es = new EventSource('/__reload');
  es.onmessage = function () { console.log('[hot-reload] reloading…'); location.reload(); };
  es.onerror   = function () { /* server restart — let browser auto-reconnect */ };
})();`

// ScriptTag is what HTML responses get injected with. Exposed as a
// const so callers that render templates can inline it in a
// {{.ReloadTag}} slot instead of post-processing the output.
const ScriptTag = `<script src="/__reload.js"></script>`

// Broadcaster is the contract HotReload needs from its underlying
// fan-out hub. Defined here as an interface (instead of importing a
// concrete type) so this module stays self-contained — no transitive
// dep on common/ownhttp from every binary that wants hot-reload.
//
// common/ownhttp.Hub satisfies this implicitly. Callers may swap in
// any other broadcaster (mock for tests, custom transport in prod).
type Broadcaster interface {
	// Broadcast wakes every connected subscriber. Non-blocking.
	Broadcast()
	// ServeSSE returns an http handler that streams reload events
	// to subscribers as Server-Sent Events. eventName is the value
	// written in `data:` lines (the browser ignores it for plain
	// `onmessage`, but it's useful when filtering on event types).
	ServeSSE(eventName string) http.HandlerFunc
}

// HotReload bundles everything: filesystem watcher (via the Watcher
// in this same package), the caller's broadcast hub, the SSE
// endpoint, the client-side JS, and the HTML injection helper.
//
// Typical wiring:
//
//	hub := ownhttp.NewHub()
//	hr, err := fswatch.NewHotReloader(100*time.Millisecond, hub)
//	if err != nil { log.Fatal(err) }
//	defer hr.Close()
//	hr.WatchTree("static")
//	hr.Mount(mux)              // /__reload + /__reload.js
//	hr.Run(ctx)                // start watching
//	body = hr.InjectScript(body)
//
// Not for production. Emits verbose logs, no auth, no caching — it's
// a local iteration tool.
//
// Not safe for concurrent setup (call WatchTree/Mount/OnReload before
// Run); the broadcaster and HTML injection are safe to use from any
// goroutine afterward.
type HotReload struct {
	hub      Broadcaster
	watcher  *Watcher
	onReload []func() // ran in order, before each Broadcast
}

// NewHotReloader creates a HotReload with the given debounce window
// and a caller-supplied broadcaster. A 100ms window collapses the
// 3–5 writes most editors emit per ⌘S into a single reload broadcast.
func NewHotReloader(debounceWindow time.Duration, hub Broadcaster) (*HotReload, error) {
	w, err := New(debounceWindow)
	if err != nil {
		return nil, err
	}
	return &HotReload{
		hub:     hub,
		watcher: w,
	}, nil
}

// WatchTree adds `root` plus every subdirectory to the watcher.
// Thin pass-through to Watcher.AddTree — kept on this type so
// callers don't have to thread the Watcher around for the common
// case.
func (h *HotReload) WatchTree(root string) error {
	return h.watcher.AddTree(root)
}

// OnReload registers a callback fired on every detected change,
// before the SSE broadcast. Used by callers that need to refresh
// in-process state alongside the browser reload — e.g. bumping a
// cache-busting asset version. Multiple callbacks run in registration
// order. Call before Run; not safe for concurrent registration.
func (h *HotReload) OnReload(fn func()) {
	h.onReload = append(h.onReload, fn)
}

// Run starts the event loop in a background goroutine. Cancel the
// ctx (e.g. via signal.NotifyContext) to stop it.
func (h *HotReload) Run(ctx context.Context) {
	go h.watcher.Run(ctx, func(path string) {
		log.Printf("[hot-reload] change detected → broadcasting reload (%s)", path)
		for _, fn := range h.onReload {
			fn()
		}
		h.hub.Broadcast()
	})
}

// Close releases the underlying watcher. Safe to call multiple
// times; ignores subsequent errors.
func (h *HotReload) Close() error {
	return h.watcher.Close()
}

// Broadcast triggers a reload on every connected browser, bypassing
// the watcher loop. Useful for callers that watch *other* things
// (a separate config file, a NATS event, etc.) and want to piggy-
// back on the existing SSE client without duplicating the JS.
func (h *HotReload) Broadcast() {
	h.hub.Broadcast()
}

// Mount registers /__reload (SSE stream) and /__reload.js (the JS
// client) on the given mux. Call once after setup, before Run.
func (h *HotReload) Mount(mux *http.ServeMux) {
	mux.HandleFunc("/__reload", h.hub.ServeSSE("reload"))
	mux.HandleFunc("/__reload.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write([]byte(reloadScript))
	})
}

// InjectScript splices ScriptTag right before </body>, appends to
// the end when no </body> is present. Use in a response-body
// rewriter around static file handlers so every HTML page picks up
// the reload loop without per-template edits.
func (h *HotReload) InjectScript(body []byte) []byte {
	if bytes.Contains(body, []byte("</body>")) {
		return bytes.Replace(body, []byte("</body>"),
			[]byte(ScriptTag+"\n</body>"), 1)
	}
	return append(body, []byte(ScriptTag)...)
}
