package platformkit

import (
	"context"
	"sort"
	"strconv"
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

// PeriodicWorker runs fn immediately, then re-runs it every interval until
// ctx is cancelled. Designed for background refresh tasks (contact
// directories, metadata caches, robots.txt re-reads, etc.). Blocks — the
// caller should launch it in a goroutine.
//
//	go platformkit.PeriodicWorker(ctx, 24*time.Hour, func(ctx context.Context) {
//	    dir, err := fetchContacts(ctx)
//	    ...
//	})
func StartWorker(ctx context.Context, interval time.Duration, fn func(context.Context)) {
	fn(ctx)
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

func Order[T ReversibleItem](s *[]T, shouldReverse bool) {
	if shouldReverse {
		reverse(s)
	}

	for i := range *s {
		(*s)[i].SetTotal(len(*s))
		(*s)[i].SetCurrent(i + 1)
	}
}

// OrderBy sorts s by the key returned from keyFn, then populates
// SetTotal / SetCurrent on each item.
//
// Smart comparison: when BOTH keys in a pair are pure integers
// (e.g. contractor.de's "30290"), they compare numerically so
// "9000" < "30290". Otherwise lexicographic order is used. This
// handles numeric IDs without caller-side parsing.
//
// When descending is true, the largest key is placed first
// (index 0) — useful when the largest ID is "newest" and should
// be processed first.
func OrderBy[T ReversibleItem](s *[]T, keyFn func(T) string, descending bool) {
	sort.SliceStable(*s, func(i, j int) bool {
		a := keyFn((*s)[i])
		b := keyFn((*s)[j])
		if ai, ae := strconv.Atoi(a); ae == nil {
			if bi, be := strconv.Atoi(b); be == nil {
				if descending {
					return ai > bi
				}
				return ai < bi
			}
		}
		if descending {
			return a > b
		}
		return a < b
	})
	for i := range *s {
		(*s)[i].SetTotal(len(*s))
		(*s)[i].SetCurrent(i + 1)
	}
}
