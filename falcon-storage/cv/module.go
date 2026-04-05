package cv

import "context"

// Module wires the CV ingest pipeline into falcon-storage.
type Module struct{}

func NewModule() *Module { return &Module{} }

func (m *Module) Register(ctx context.Context) error {
	svc, err := newService(ctx)
	if err != nil {
		return err
	}
	return svc.subscribe(ctx)
}
