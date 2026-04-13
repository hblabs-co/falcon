package hblabsco

import (
	"context"

	"hblabs.co/falcon/modules/platformkit"
)

type Runner struct {
	logger  platformkit.Logger
	batchFn platformkit.BatchFn[*Item]
}

func New() *Runner {
	return &Runner{
		logger:  platformkit.NoopLogger{},
		batchFn: platformkit.SequentialBatch[*Item],
	}
}

func (r *Runner) Name() string      { return Source }
func (r *Runner) BaseURL() string   { return "" } // metadata loop disabled
func (r *Runner) CompanyID() string { return "" }

func (r *Runner) SetLogger(logger any) {
	r.logger = platformkit.ResolveLogger(logger)
}

func (r *Runner) SetSaveHandler(fn platformkit.SaveFn) {
}

func (r *Runner) SetFilterHandler(fn platformkit.FilterFn) {
}

func (r *Runner) SetWarnHandler(fn platformkit.WarnFn) {
}

func (r *Runner) SetErrHandler(fn platformkit.ErrFn) {
}

func (r *Runner) SetBatchConfig(cfg platformkit.BatchConfig) {
}

func (r *Runner) Retry(_ context.Context, _ any, _ any) error {
	return nil
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
		items, err := r.findCandidates(ctx)
		if err != nil || len(items) == 0 {
			return
		}
		r.batchFn(ctx, items, r.process)
	}
}

func (r *Runner) findCandidates(_ context.Context) ([]*Item, error) {
	// scraping logic
	return []*Item{
		{ID: "1", URL: "https://..."},
		{ID: "2", URL: "https://..."},
	}, nil
}

func (r *Runner) process(_ context.Context, item *Item) {
	// process logic
	r.logger.Infof("processing %s", item.ID)
}
