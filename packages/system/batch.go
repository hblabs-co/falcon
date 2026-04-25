package system

import (
	"context"
	"time"
)

// BatchConfig holds the timing parameters for batch processing.
type BatchConfig struct {
	// Size is the number of items processed before a longer pause (batchDelay).
	Size int
	// ItemDelay is the pause between individual items within a batch.
	ItemDelay time.Duration
	// BatchDelay is the pause between batches.
	BatchDelay time.Duration
}

// BatchProcess calls fn for each item sequentially, pausing ItemDelay between
// items and BatchDelay after every Size items to simulate human browsing pace.
// Stops early and returns if ctx is cancelled.
func BatchProcess[T any](ctx context.Context, items []*T, cfg BatchConfig, fn func(context.Context, *T)) {
	for i, item := range items {
		if ctx.Err() != nil {
			return
		}
		fn(ctx, item)
		if i == len(items)-1 {
			break
		}
		if (i+1)%cfg.Size == 0 {
			time.Sleep(cfg.BatchDelay)
		} else {
			time.Sleep(cfg.ItemDelay)
		}
	}
}
