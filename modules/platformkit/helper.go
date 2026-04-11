package platformkit

import (
	"context"
	"sync"
	"time"
)

// BatchConfig holds the timing parameters used by ThrottledBatch to pace
// processing of a slice of items, simulating human browsing speed and avoiding
// soft-blocks from the source platform.
type BatchConfig struct {
	// Size is the number of items processed before a longer pause (BatchDelay).
	Size int
	// ItemDelay is the pause between individual items within a batch.
	ItemDelay time.Duration
	// BatchDelay is the pause between batches (every Size items).
	BatchDelay time.Duration
}

// BatchFn processes a slice of items by invoking process for each one.
// Implementations decide whether to run sequentially or in parallel.
type BatchFn[T any] func(ctx context.Context, items []T, process func(context.Context, T))

// SequentialBatch is a BatchFn that processes items one at a time in order.
// This is the safe default for runners that don't need concurrency.
func SequentialBatch[T any](ctx context.Context, items []T, process func(context.Context, T)) {
	for _, item := range items {
		if ctx.Err() != nil {
			return
		}
		process(ctx, item)
	}
}

// ThrottledBatch returns a BatchFn that processes items sequentially while
// pausing ItemDelay between items and BatchDelay every Size items. Use this
// for scrapers that must respect a polite browsing pace to avoid soft-blocks.
// Returns early if ctx is cancelled.
func ThrottledBatch[T any](cfg BatchConfig) BatchFn[T] {
	return func(ctx context.Context, items []T, process func(context.Context, T)) {
		if cfg.Size < 1 {
			cfg.Size = 1
		}
		for i, item := range items {
			if ctx.Err() != nil {
				return
			}
			process(ctx, item)
			if i == len(items)-1 {
				break
			}
			if (i+1)%cfg.Size == 0 {
				sleepCtx(ctx, cfg.BatchDelay)
			} else {
				sleepCtx(ctx, cfg.ItemDelay)
			}
		}
	}
}

// sleepCtx pauses for d unless ctx is cancelled first.
func sleepCtx(ctx context.Context, d time.Duration) {
	if d <= 0 {
		return
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
	case <-t.C:
	}
}

// ConcurrentBatch returns a BatchFn that processes items with up to `workers`
// goroutines in flight. Use this when individual items are slow (network I/O)
// and the platform tolerates parallel scraping.
func ConcurrentBatch[T any](workers int) BatchFn[T] {
	if workers < 1 {
		workers = 1
	}
	return func(ctx context.Context, items []T, process func(context.Context, T)) {
		var wg sync.WaitGroup
		sem := make(chan struct{}, workers)
		for _, item := range items {
			if ctx.Err() != nil {
				break
			}
			wg.Add(1)
			sem <- struct{}{}
			go func(item T) {
				defer wg.Done()
				defer func() { <-sem }()
				process(ctx, item)
			}(item)
		}
		wg.Wait()
	}
}

func reverse[T any](s *[]T) {
	for i, j := 0, len(*s)-1; i < j; i, j = i+1, j-1 {
		(*s)[i], (*s)[j] = (*s)[j], (*s)[i]
	}
}

type ReversibleItem interface {
	SetTotal(n int)
	SetCurrent(n int)
}

func Order[T ReversibleItem](s *[]T, shouldReverse bool) {
	if shouldReverse {
		reverse(s)
	}

	for i := range *s {
		(*s)[i].SetTotal(len(*s))
		(*s)[i].SetCurrent(i + 1)
	}
}
