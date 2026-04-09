// platformkit/processor.go
package platformkit

import "context"

type Processor[T any] struct {
	Process func(ctx context.Context, item T)
}

type BatchFn func(ctx context.Context, items []any, process func(ctx context.Context, item any))
