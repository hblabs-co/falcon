package system

import (
	"context"
	"fmt"
)

// Module is implemented by every falcon-storage worker.
// Register wires up NATS consumers and returns — consumers run in background goroutines.
type Module interface {
	Register(ctx context.Context) error
}

// Run initialises each module in order and returns the first error encountered.
func Run(ctx context.Context, modules ...Module) error {
	for _, m := range modules {
		if err := m.Register(ctx); err != nil {
			return fmt.Errorf("%T: %w", m, err)
		}
	}
	return nil
}
