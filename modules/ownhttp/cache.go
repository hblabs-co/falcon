package ownhttp

import (
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

// AssetVersion is the cache-busting query value appended to static
// asset URLs (`/static/styles.css?v=…`). It serves two scenarios
// from one type:
//
//   - Production-only services (e.g. falcon-landing): construct
//     once at startup, never bump. The token rotates on every
//     redeploy because each new process initialises a fresh value.
//   - Dev tools that hot-reload (e.g. falcon-nest): construct at
//     startup, call Bump every time on-disk assets change so the
//     next page render emits a new token and the browser bypasses
//     its `immutable` cache.
//
// Atomic so the goroutine that bumps and the HTTP handler that
// reads don't need to coordinate via a mutex.
type AssetVersion struct {
	v atomic.Int64
}

// NewAssetVersion returns a token initialised to the current Unix
// time in seconds. Callers that want a deterministic token (e.g.
// pinned to BUILD_TIME) can call Bump after construction.
func NewAssetVersion() *AssetVersion {
	a := &AssetVersion{}
	a.v.Store(time.Now().Unix())
	return a
}

// Bump advances the token to "now". Subsequent calls to Current
// return the new value; in-flight HTTP responses are unaffected.
func (a *AssetVersion) Bump() {
	a.v.Store(time.Now().Unix())
}

// Current returns the token as a string suitable for a query-param
// value. Format is opaque: callers should not parse it.
func (a *AssetVersion) Current() string {
	return strconv.FormatInt(a.v.Load(), 10)
}

// CacheImmutable wraps a handler so its responses tell the browser
// to cache aggressively for a week and treat content as immutable.
// Safe only when the URL itself encodes a version (e.g. ?v=…) so
// that updated content is served from a different URL.
func CacheImmutable(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=604800, immutable")
		next.ServeHTTP(w, r)
	})
}

// CacheMaxAge wraps a handler with a public max-age cache header of
// the given duration. Use for resources that *can* be stale briefly
// (sitemaps, robots.txt, lightly-templated HTML) but don't deserve
// a full week of cache.
func CacheMaxAge(d time.Duration) func(http.Handler) http.Handler {
	header := "public, max-age=" + strconv.Itoa(int(d.Seconds()))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Cache-Control", header)
			next.ServeHTTP(w, r)
		})
	}
}

// NoCache wraps a handler so its responses tell the browser never
// to cache. Use for dynamic HTML in dev tools where every render
// must reflect the latest server state (config changes, new
// `?v=…` tokens, etc).
func NoCache(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")
		next.ServeHTTP(w, r)
	})
}
