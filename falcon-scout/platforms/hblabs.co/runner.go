package hblabsco

import (
	"context"

	"hblabs.co/falcon/common/interfaces"
	"hblabs.co/falcon/common/system"
)

const Source = "hblabs.co"

type Runner struct {
	logger interfaces.Logger
}

func New() *Runner { return &Runner{} }

func (r *Runner) SetLogger(logger interfaces.Logger) {
	r.logger = logger
}

func (r *Runner) Name() string {
	return Source
}

func (r *Runner) Init(ctx context.Context) error {
	return nil
}

func (r *Runner) StartConsumers(ctx context.Context) error {
	return nil
}

func (r *Runner) StartWorkers(ctx context.Context) {
}

func (r *Runner) Poll(ctx context.Context) {
	system.Poll(system.Ctx(), system.PollInterval(), r.logger, func() {
		r.logger.Info("is alive")
	})
}
