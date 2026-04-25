package ownhttp

import (
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// LogRequest emits a standardised HTTP-request log line. Shared
// between Gin middleware and vanilla net/http wrappers so logs from
// every service are grep-able with one format:
//
//	level=info msg=request method=GET path=/healthz status=200 duration=8.44µs
//
// Callers tuck their per-framework hooks around the call (Gin's
// c.Next(), net/http's deferred write-wrapper) so this fn stays
// framework-agnostic.
func LogRequest(method, path string, status int, duration time.Duration) {
	logrus.WithFields(logrus.Fields{
		"method":   method,
		"path":     path,
		"status":   status,
		"duration": duration.String(),
	}).Info("request")
}

// LoggingMiddleware wraps a `next` handler for vanilla net/http
// services (falcon-landing, falcon-admin, dev servers). Captures
// the response status and elapsed time, then hands off to LogRequest
// so every service emits the same line shape.
func LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusCapturingWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		LogRequest(r.Method, r.URL.Path, rec.status, time.Since(start))
	})
}

// statusCapturingWriter remembers the status code so LoggingMiddleware
// can log it after next.ServeHTTP returns. Handlers that never call
// WriteHeader explicitly (the happy path) default to 200 via the
// constructor.
type statusCapturingWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusCapturingWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
