package main

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"hblabs.co/falcon/packages/ownhttp"
)

// Status is a one-word health verdict shown next to each card.
type Status string

const (
	StatusOnline  Status = "online"
	StatusOffline Status = "offline"
	StatusUnknown Status = "unknown"
)

// statusStore caches the most recent status per component name. The
// background poller writes; the dashboard handler reads. Mutex-guarded
// because both happen on different goroutines.
type statusStore struct {
	mu  sync.RWMutex
	now map[string]Status
}

func newStatusStore() *statusStore {
	return &statusStore{now: map[string]Status{}}
}

func (s *statusStore) set(name string, st Status) {
	s.mu.Lock()
	s.now[name] = st
	s.mu.Unlock()
}

func (s *statusStore) get(name string) Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if v, ok := s.now[name]; ok {
		return v
	}
	return StatusUnknown
}

// snapshot returns a copy of the whole status map. Used by the WS
// handler as its payload function so every connected client gets the
// full picture on each broadcast (simpler than diffing).
func (s *statusStore) snapshot() map[string]Status {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]Status, len(s.now))
	for k, v := range s.now {
		out[k] = v
	}
	return out
}

// statusPoller probes every component on a fixed interval and writes
// the verdict to `store`. Runs forever in a goroutine; cancelling
// `ctx` stops it.
//
// Probe rules per component (see component fields):
//
//   - StatusURL set → HTTP GET, 2xx-3xx is online.
//   - StatusHost set → TCP dial, 1s timeout. Used for plain TCP
//     daemons (Mongo, Ollama on raw HTTP that doesn't expose 200 on
//     /).
//   - Neither set → leaves the cached value at "unknown" forever.
//     Documented in the dashboard so the user knows it's a TODO,
//     not a bug.
func runStatusPoller(ctx context.Context, store *statusStore, hub *ownhttp.Hub, interval time.Duration, components func() []component) {
	tick := func() {
		// Re-read the component set on every tick — config.yaml can
		// change at runtime (URLs renamed, services added/removed),
		// and the watcher swaps the underlying slice under our feet.
		all := components()
		client := &http.Client{Timeout: 2 * time.Second}
		var wg sync.WaitGroup
		for _, c := range all {
			c := c
			if c.EffectiveStatusURL() == "" && c.StatusHost == "" {
				store.set(c.Name, StatusUnknown)
				continue
			}
			wg.Add(1)
			go func() {
				defer wg.Done()
				store.set(c.Name, probe(client, c))
			}()
		}
		wg.Wait()
		// Broadcast after the full tick (not per-component) so clients see
		// one coherent snapshot instead of partial updates flickering in.
		if hub != nil {
			hub.Broadcast()
		}
	}

	tick() // prime so the first dashboard render isn't all "unknown"

	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			tick()
		}
	}
}

func probe(client *http.Client, c component) Status {
	if url := c.EffectiveStatusURL(); url != "" {
		// HEAD first — cheaper, plenty of services reply 200 to HEAD.
		// Fall back to GET when HEAD comes back 405 / network blip,
		// since some healthchecks (Qdrant, MinIO) only honor GET.
		if ok := httpProbe(client, "HEAD", url); ok {
			return StatusOnline
		}
		if ok := httpProbe(client, "GET", url); ok {
			return StatusOnline
		}
		return StatusOffline
	}
	if c.StatusHost != "" {
		conn, err := net.DialTimeout("tcp", c.StatusHost, 1*time.Second)
		if err != nil {
			return StatusOffline
		}
		_ = conn.Close()
		return StatusOnline
	}
	return StatusUnknown
}

func httpProbe(client *http.Client, method, url string) bool {
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return false
	}
	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 400
}
