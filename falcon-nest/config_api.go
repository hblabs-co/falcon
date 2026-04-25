package main

import (
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// 1 MiB cap — config.yaml is currently ~5 KiB; this leaves head-room
// for adding sections without ever risking an OOM from a malformed
// upload (e.g. a runaway editor pasting a binary file).
const configMaxBytes = 1 << 20

// registerConfigAPI wires GET/PUT /api/config. PUT is gated to
// localhost so a stray network exposure can't be turned into
// arbitrary file writes — Nest has no auth story.
func registerConfigAPI(mux *http.ServeMux, path string) {
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			serveGet(w, path)
		case http.MethodPut:
			if !isLocalRequest(r) {
				http.Error(w, "config edit is restricted to localhost", http.StatusForbidden)
				return
			}
			servePut(w, r, path)
		default:
			w.Header().Set("Allow", "GET, PUT")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
}

func serveGet(w http.ResponseWriter, path string) {
	raw, etag, err := readWithETag(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/x-yaml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("ETag", etag)
	_, _ = w.Write(raw)
}

func servePut(w http.ResponseWriter, r *http.Request, path string) {
	body, err := io.ReadAll(io.LimitReader(r.Body, configMaxBytes+1))
	if err != nil {
		http.Error(w, "read body: "+err.Error(), http.StatusBadRequest)
		return
	}
	if len(body) > configMaxBytes {
		http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
		return
	}
	// Validate before disk: an unparseable file would just trigger
	// "reload failed, keeping previous" in the watcher forever.
	if err := yaml.Unmarshal(body, new(Config)); err != nil {
		http.Error(w, "yaml parse: "+err.Error(), http.StatusBadRequest)
		return
	}
	// Optimistic concurrency — refuse if disk moved under the editor.
	if want := r.Header.Get("If-Match"); want != "" {
		if have, err := etagOf(path); err == nil && have != want {
			http.Error(w, "config changed on disk since GET", http.StatusPreconditionFailed)
			return
		}
	}
	if err := atomicWrite(path, body); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	etag, err := etagOf(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("ETag", etag)
	w.WriteHeader(http.StatusNoContent)
}

// etagOf is the file's mtime in nanos — cheap, monotonic per edit,
// good enough for the editor's optimistic-concurrency check.
func etagOf(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	return strconv.FormatInt(info.ModTime().UnixNano(), 10), nil
}

func readWithETag(path string) ([]byte, string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	etag, err := etagOf(path)
	if err != nil {
		return nil, "", err
	}
	return raw, etag, nil
}

// atomicWrite writes body to a sibling tmp file and renames into
// place. Same directory matters: rename across filesystems isn't
// atomic. fsync before rename so a crash mid-write doesn't leave
// a half-flushed file on disk.
func atomicWrite(path string, body []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	cleanup := func() { _ = os.Remove(tmp.Name()) }
	if _, err := tmp.Write(body); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		cleanup()
		return err
	}
	return nil
}

// isLocalRequest returns true when the connection comes from this
// machine. Loopback covers 127.0.0.0/8 and ::1.
func isLocalRequest(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
