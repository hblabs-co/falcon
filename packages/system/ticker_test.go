package system

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestPoll_CallsFnImmediately(t *testing.T) {
	var count int32
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		Poll(ctx, 10*time.Second, nil, func() {
			atomic.AddInt32(&count, 1)
		})
		close(done)
	}()

	// fn must be invoked on the first iteration before any tick fires.
	time.Sleep(100 * time.Millisecond)
	if atomic.LoadInt32(&count) < 1 {
		t.Error("fn was not called immediately after Poll started")
	}

	cancel()

	select {
	case <-done:
		// expected
	case <-time.After(2 * time.Second):
		t.Error("Poll did not stop after context cancel")
	}
}

func TestPoll_CallsFnOnEachTick(t *testing.T) {
	var count int32
	done := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		Poll(ctx, 30*time.Millisecond, nil, func() {
			atomic.AddInt32(&count, 1)
		})
		close(done)
	}()

	// Wait long enough for the initial call + at least two ticks.
	time.Sleep(120 * time.Millisecond)

	if atomic.LoadInt32(&count) < 3 {
		t.Errorf("expected fn to be called at least 3 times, got %d", atomic.LoadInt32(&count))
	}

	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Error("Poll did not stop after context cancel")
	}
}
