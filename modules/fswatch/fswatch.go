// Package fswatch wraps fsnotify with the boilerplate every dev-
// reload / config-watch use case needs: recursive directory adds,
// single-file watches that survive editor save-by-rename, debounce
// coalescing of bursty editor writes, and a single onChange callback.
//
// Typical wiring:
//
//	w, err := fswatch.New(100 * time.Millisecond)
//	if err != nil { log.Fatal(err) }
//	defer w.Close()
//	w.AddTree("static")          // watch directory + subdirs
//	w.AddFile("config.yaml")     // watch a specific file
//	w.Run(ctx, func(path string) {
//	    // fired once per debounced burst, with the most-recent path
//	})
//
// Not safe for concurrent setup: call AddTree/AddFile before Run.
// After Run starts, the watcher is fixed for its lifetime — adding
// paths after the loop is running is a future-iteration concern.
package fswatch

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher coalesces fsnotify events through a debounce window and
// invokes the user's callback once per burst. See package doc for
// usage.
type Watcher struct {
	inner    *fsnotify.Watcher
	debounce time.Duration

	// treeDirs are directories added via AddTree — every event in
	// them passes through. fileDirs hold dir → set-of-basenames
	// added via AddFile, so we drop unrelated events on shared dirs
	// (config.yaml in repo root must not fire on every main.go save).
	// Read-only after Run starts; protected by mu only during setup.
	mu       sync.Mutex
	treeDirs map[string]struct{}
	fileDirs map[string]map[string]struct{}
}

// New creates a Watcher with the given debounce window. A 100ms
// window collapses the 3–5 writes most editors emit per ⌘S into a
// single onChange call.
func New(debounce time.Duration) (*Watcher, error) {
	inner, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return &Watcher{
		inner:    inner,
		debounce: debounce,
		treeDirs: map[string]struct{}{},
		fileDirs: map[string]map[string]struct{}{},
	}, nil
}

// AddTree adds `root` plus every subdirectory to the watcher.
// fsnotify isn't recursive on macOS/Linux, so walking once at setup
// is the standard workaround. New subdirs created after Run starts
// won't be auto-watched; re-create the Watcher to pick them up.
func (w *Watcher) AddTree(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if err := w.inner.Add(path); err != nil {
			return err
		}
		w.mu.Lock()
		w.treeDirs[path] = struct{}{}
		w.mu.Unlock()
		return nil
	})
}

// AddFile registers a single file to watch. fsnotify is pointed at
// the parent directory (not the file itself) because many editors
// save via rename, which invalidates a direct-file watch. Other
// files in that directory are filtered out by basename.
func (w *Watcher) AddFile(path string) error {
	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	base := filepath.Base(path)
	if err := w.inner.Add(dir); err != nil {
		return err
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.fileDirs[dir] == nil {
		w.fileDirs[dir] = map[string]struct{}{}
	}
	w.fileDirs[dir][base] = struct{}{}
	return nil
}

// Run blocks until ctx is done. Every event that passes the path
// filter starts/extends a debounce timer; when the timer fires,
// onChange is invoked with the most recent matching path.
//
// Errors from fsnotify are logged via stdlib log and don't stop the
// loop — a single bad event shouldn't tear down the watcher.
func (w *Watcher) Run(ctx context.Context, onChange func(path string)) {
	var (
		timer    *time.Timer
		mu       sync.Mutex
		lastPath string
	)
	fire := func() {
		mu.Lock()
		path := lastPath
		mu.Unlock()
		onChange(path)
	}

	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-w.inner.Events:
			if !ok {
				return
			}
			// Chmod-only events (editors touch perms on save without
			// modifying content) add noise without signal — drop them.
			if ev.Op == fsnotify.Chmod {
				continue
			}
			if !w.allow(ev.Name) {
				continue
			}
			mu.Lock()
			lastPath = ev.Name
			mu.Unlock()
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, fire)
		case err, ok := <-w.inner.Errors:
			if !ok {
				return
			}
			log.Printf("[fswatch] error: %v", err)
		}
	}
}

// allow returns true if the event path matches one of the watched
// trees or files. A tree-watched directory accepts every event;
// a file-watched directory only accepts events whose basename was
// registered via AddFile.
func (w *Watcher) allow(eventPath string) bool {
	dir := filepath.Dir(eventPath)
	w.mu.Lock()
	defer w.mu.Unlock()
	if _, tree := w.treeDirs[dir]; tree {
		return true
	}
	if files, ok := w.fileDirs[dir]; ok {
		_, allowed := files[filepath.Base(eventPath)]
		return allowed
	}
	return false
}

// Close releases the underlying fsnotify watcher. Safe to call
// multiple times.
func (w *Watcher) Close() error {
	return w.inner.Close()
}
