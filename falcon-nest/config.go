package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
	"hblabs.co/falcon/packages/fswatch"
)

// configFile is the on-disk source of truth for services, infra and
// port-forward lists. Resolved relative to the working directory so
// `go run .` from falcon-nest/ picks it up.
const configFile = "config.yaml"

// Config is the whole dashboard catalogue — the parsed shape of
// config.yaml. Sections mirror the three rendered blocks in the UI.
type Config struct {
	Services     []component   `yaml:"services"`
	Infra        []component   `yaml:"infra"`
	PortForwards []portForward `yaml:"portForwards"`
}

// configStore owns the parsed config plus a set of subscriber
// channels that get a tick every time the file is reloaded.
// Everything behind this type is thread-safe; the dashboard handler,
// the status poller and the watcher goroutine all touch it.
type configStore struct {
	mu   sync.RWMutex
	cfg  *Config
	subs []chan struct{}
}

// newConfigStore loads the file once and returns the populated store.
// A failed initial parse is fatal — starting with an empty catalogue
// would silently render a blank portal, which is worse than crashing.
func newConfigStore(path string) (*configStore, error) {
	cfg, err := readConfig(path)
	if err != nil {
		return nil, fmt.Errorf("initial load %s: %w", path, err)
	}
	return &configStore{cfg: cfg}, nil
}

// snapshot returns the current parsed config. The returned pointer
// is immutable from the caller's perspective — reloads swap the
// whole struct, never mutate fields in place.
func (s *configStore) snapshot() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cfg
}

// components returns the combined services + infra slice the poller
// probes. Recomputed on every call so adds/removes in the YAML take
// effect on the next tick.
func (s *configStore) components() []component {
	cfg := s.snapshot()
	out := make([]component, 0, len(cfg.Services)+len(cfg.Infra))
	out = append(out, cfg.Services...)
	out = append(out, cfg.Infra...)
	return out
}

// subscribe returns a channel that receives a tick after every
// successful reload. Buffered-1 so a slow subscriber just drops
// duplicate ticks (still learns on the next reload).
func (s *configStore) subscribe() chan struct{} {
	ch := make(chan struct{}, 1)
	s.mu.Lock()
	s.subs = append(s.subs, ch)
	s.mu.Unlock()
	return ch
}

// notify signals every subscriber. Non-blocking — a full channel
// means the subscriber hasn't consumed the last tick yet, which is
// fine: one reload signal is as useful as many.
func (s *configStore) notify() {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, ch := range s.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// watch blocks until ctx is done. Uses fswatch under the hood, which
// owns the fsnotify boilerplate (parent-dir watch, basename filter,
// debounce). A failed parse logs the error and keeps the previous
// good config in place — editing YAML shouldn't crash the portal.
func (s *configStore) watch(ctx context.Context, path string, debounce time.Duration) error {
	w, err := fswatch.New(debounce)
	if err != nil {
		return err
	}
	defer w.Close()
	if err := w.AddFile(path); err != nil {
		return err
	}

	w.Run(ctx, func(_ string) {
		cfg, err := readConfig(path)
		if err != nil {
			log.Printf("[config] reload failed, keeping previous: %v", err)
			return
		}
		s.mu.Lock()
		s.cfg = cfg
		s.mu.Unlock()
		log.Printf("[config] reloaded: %d services, %d infra, %d port-forwards",
			len(cfg.Services), len(cfg.Infra), len(cfg.PortForwards))
		s.notify()
	})
	return nil
}

func readConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := yaml.Unmarshal(raw, &cfg); err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	return &cfg, nil
}
