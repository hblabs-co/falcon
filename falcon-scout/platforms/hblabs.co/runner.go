package hblabsco

import (
	"context"

	"hblabs.co/falcon/scout/platformkit"
)

type Runner struct {
	logger  platformkit.Logger
	batchFn func(ctx context.Context, items []Item, process func(context.Context, Item))
}

func New() *Runner {
	r := &Runner{logger: platformkit.NoopLogger{}}
	r.batchFn = func(ctx context.Context, items []Item, process func(context.Context, Item)) {
		for _, item := range items {
			process(ctx, item)
		}
	}
	return r
}

func (r *Runner) Name() string { return Source }

func (r *Runner) SetLogger(logger any) {
	r.logger = platformkit.ResolveLogger(logger)
}

func (r *Runner) Init(ctx context.Context) error {
	return nil
}

func (r *Runner) StartConsumers(ctx context.Context) error {
	return nil
}

func (r *Runner) StartWorkers(ctx context.Context) {
}

func (r *Runner) Poll(ctx context.Context) func() {
	return func() {
		items, err := r.scrape(ctx)
		if err != nil || len(items) == 0 {
			return
		}
		r.batchFn(ctx, items, r.process)
	}
}

func (r *Runner) scrape(_ context.Context) ([]Item, error) {
	// scraping logic
	return []Item{{ID: "1", URL: "https://..."}, {ID: "2", URL: "https://..."}}, nil
}

func (r *Runner) process(ctx context.Context, item Item) {
	// process logic
	r.logger.Infof("processing %s", item.ID)
}
