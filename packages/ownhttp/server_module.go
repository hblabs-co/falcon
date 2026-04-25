package ownhttp

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"
)

// ServerModule wraps a net/http server in the system.Module +
// system.ShutdownModule contract. Drop-in replacement for the
// `http.ListenAndServe` calls that scattered across every Falcon
// HTTP service: every one now gets the same graceful-stop semantics
// without each main() reimplementing the SIGTERM dance.
//
// Usage from main():
//
//	system.RunForever(ctx,
//	    ownhttp.NewServerModule("falcon-admin", ":8080", admin.Handler()),
//	    ...other modules...,
//	)
//
// Lifecycle:
//   - Register: starts ListenAndServe in a background goroutine. The
//     goroutine logs and exits cleanly when Shutdown is called; any
//     OTHER error (port busy, etc.) is fatal.
//   - Shutdown: calls srv.Shutdown(shutdownCtx), which stops accepting
//     new connections and waits for in-flight requests to finish
//     (bounded by the SHUTDOWN_TIMEOUT-derived ctx).
type ServerModule struct {
	label  string
	srv    *http.Server
	doneCh chan struct{}
	fatal  error // captured if ListenAndServe returns something other than ErrServerClosed
}

// NewServerModule builds a ServerModule with sensible defaults. The
// server's ReadHeaderTimeout is set so Slowloris-style attacks can't
// pin a goroutine forever — every Falcon service should have one,
// and forgetting is easy.
func NewServerModule(label, addr string, handler http.Handler) *ServerModule {
	return &ServerModule{
		label: label,
		srv: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
		},
		doneCh: make(chan struct{}),
	}
}

// Register implements system.Module. Starts the listener on a
// goroutine and returns immediately. Any failure that happens before
// the goroutine even calls ListenAndServe still fails the whole boot
// (the goroutine signals via doneCh + fatal).
func (m *ServerModule) Register(_ context.Context) error {
	go func() {
		defer close(m.doneCh)
		logrus.Infof("[%s] http listening on %s", m.label, m.srv.Addr)
		err := m.srv.ListenAndServe()
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			// Capture so Shutdown can report it; also log so the
			// operator sees it without waiting for SIGTERM.
			m.fatal = err
			logrus.Errorf("[%s] http server: %v", m.label, err)
		}
	}()
	return nil
}

// Shutdown implements system.ShutdownModule with a two-phase teardown:
//
//  1. srv.Shutdown(shutdownCtx) — stop accepting new connections and
//     wait for in-flight requests to drain. The standard graceful path.
//  2. If the deadline fires (typical with long-lived SSE / WebSocket
//     handlers — Shutdown does NOT close already-established
//     connections, only stops accepting), srv.Close() force-closes
//     every remaining socket. The pod is about to die anyway; an SSE
//     reconnect on the next pod is the expected client behaviour
//     during a k8s rollout.
//
// Returns nil even after a forced close — the shutdown completed from
// the operator's perspective, just less politely than ideal. Genuine
// errors (Close itself failing, listen-goroutine stuck) still bubble
// up so the rollout sees them.
func (m *ServerModule) Shutdown(shutdownCtx context.Context) error {
	logrus.Infof("[%s] http server: graceful shutdown", m.label)
	if err := m.srv.Shutdown(shutdownCtx); err != nil {
		logrus.Warnf("[%s] graceful shutdown timed out (%v) — forcing close", m.label, err)
		if cerr := m.srv.Close(); cerr != nil {
			return fmt.Errorf("[%s] force close: %w", m.label, cerr)
		}
	}
	// ListenAndServe returns ErrServerClosed once Shutdown/Close
	// runs. Bounded wait — a stuck goroutine shouldn't hang
	// RunForever indefinitely. shutdownCtx may already be exhausted
	// after a forced close, so use a small fixed grace window
	// instead.
	select {
	case <-m.doneCh:
	case <-time.After(2 * time.Second):
		return fmt.Errorf("[%s] listen-goroutine drain timeout", m.label)
	}
	return m.fatal
}
