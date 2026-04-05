package company_logo

import "context"

// Module wires the company-logo pipeline into falcon-storage.
type Module struct{}

func NewModule() *Module { return &Module{} }

func (m *Module) Register(ctx context.Context) error {
	svc, err := newService()
	if err != nil {
		return err
	}
	return svc.subscribe(ctx)
}
