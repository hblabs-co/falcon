package system

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/sirupsen/logrus"
	"hblabs.co/falcon/packages/interfaces"
)

// StartWorker launches a background goroutine that calls fn every interval.
// Unlike Poll, it does not block the caller and does not run fn immediately.
func StartWorker(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				fn(ctx)
			}
		}
	}()
}

// Poll calls fn in a goroutine immediately, then again every interval until
// ctx is cancelled. If the previous invocation of fn is still running when
// the next tick fires, that tick is skipped and a warning is logged.
// This means fn never runs concurrently with itself.
func Poll(ctx context.Context, interval time.Duration, logger interfaces.Logger, fn func()) {
	var running atomic.Bool

	finalLogger := logrus.WithFields(logrus.Fields{})
	if logger != nil {
		finalLogger = logger
	}

	run := func() {
		if !running.CompareAndSwap(false, true) {
			finalLogger.Warn("previous tick still running, skipping this one")
			return
		}
		go func() {
			defer running.Store(false)
			fn()
		}()
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	run()

	for {
		select {
		case <-ticker.C:
			run()
		case <-ctx.Done():
			finalLogger.Printf("stopping")
			return
		}
	}
}
