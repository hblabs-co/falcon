package system

import (
	"context"
	"fmt"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/environment"
)

// Module is implemented by every worker that wants to register itself
// into a service. Register wires up NATS consumers / starts an HTTP
// listener / launches background goroutines, and returns immediately —
// long-lived work runs in goroutines anchored to the lifecycle ctx.
type Module interface {
	Register(ctx context.Context) error
}

// ShutdownModule is the optional second half. If a module holds
// resources that need orderly teardown (open HTTP server, in-flight
// NATS handlers, websocket clients), implement Shutdown — RunForever
// detects the interface via type assertion and calls it before the
// process exits.
//
// shutdownCtx has a deadline (SHUTDOWN_TIMEOUT, default 25s — under
// k8s' 30s terminationGracePeriodSeconds). Honour it: if the deadline
// fires, return whatever you have rather than blocking forever.
type ShutdownModule interface {
	Module
	Shutdown(shutdownCtx context.Context) error
}

// RunForever is the standard service entrypoint. It:
//
//  1. Calls Register on every module in declaration order. If any
//     fails, no Shutdown is run (nothing started yet) and the error
//     is returned.
//  2. Blocks on ctx (the signal-cancellable context returned by Boot).
//  3. On SIGTERM/SIGINT, calls Shutdown on every ShutdownModule in
//     REVERSE order with a SHUTDOWN_TIMEOUT-bounded context, plus
//     drains any NATS consumers registered via system.Subscribe*.
//  4. Returns the first non-nil error from the shutdown phase, or nil
//     if everything stopped cleanly.
//
// Reverse order matters: if module B was registered after A, B's
// resources may depend on A's still being up. Tearing down in the
// reverse of construction is the safe default.
func RunForever(ctx context.Context, modules ...Module) error {
	for _, m := range modules {
		if err := m.Register(ctx); err != nil {
			return fmt.Errorf("register %T: %w", m, err)
		}
	}

	<-ctx.Done()
	// Release the OS signal handler so the next Ctrl-C kills the
	// process immediately, rather than queueing behind the orderly
	// drain we're about to run.
	if appStop != nil {
		appStop()
	}
	logrus.Info("shutdown signal received — draining")

	timeout := environment.ParseDuration("SHUTDOWN_TIMEOUT", "25s")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var firstErr error
	// Reverse order: last-registered tears down first.
	for i := len(modules) - 1; i >= 0; i-- {
		m := modules[i]
		sm, ok := m.(ShutdownModule)
		if !ok {
			continue
		}
		if err := sm.Shutdown(shutdownCtx); err != nil {
			logrus.Errorf("shutdown %T: %v", m, err)
			if firstErr == nil {
				firstErr = err
			}
		}
	}

	// Wait for in-flight NATS handlers to finish their current message
	// before tearing down the bus connection. Subscribe / SubscribeWithBackoff
	// register themselves with the drain group automatically.
	if err := drainBus(shutdownCtx); err != nil {
		logrus.Errorf("bus drain: %v", err)
		if firstErr == nil {
			firstErr = err
		}
	}

	logrus.Info("service stopped")
	return firstErr
}

