package main

import (
	"context"
	"testing"

	"hblabs.co/falcon/scout/platformkit"
)

// mockPlatform implements Platform for testing.
type mockPlatform struct {
	name            string
	initCalled      bool
	initErr         error
	subscribeCalled bool
	subscribeErr    error
	workersCalled   bool
	pollCalled      bool
}

func (m *mockPlatform) Name() string                                { return m.name }
func (m *mockPlatform) BaseURL() string                             { return "" }
func (m *mockPlatform) CompanyID() string                           { return "" }
func (m *mockPlatform) SetLogger(_ any)                             {}
func (m *mockPlatform) SetSaveHandler(_ platformkit.SaveFn)         {}
func (m *mockPlatform) SetFilterHandler(_ platformkit.FilterFn)     {}
func (m *mockPlatform) SetWarnHandler(_ platformkit.WarnFn)         {}
func (m *mockPlatform) SetErrHandler(_ platformkit.ErrFn)           {}
func (m *mockPlatform) SetBatchConfig(_ platformkit.BatchConfig)    {}
func (m *mockPlatform) Retry(_ context.Context, _ any, _ any) error { return nil }
func (m *mockPlatform) Init(_ context.Context) error {
	m.initCalled = true
	return m.initErr
}
func (m *mockPlatform) StartConsumers(_ context.Context) error {
	m.subscribeCalled = true
	return m.subscribeErr
}
func (m *mockPlatform) StartWorkers(_ context.Context) { m.workersCalled = true }
func (m *mockPlatform) Poll(_ context.Context) func() {
	m.pollCalled = true
	return func() {}
}

func TestNewService(t *testing.T) {
	svc := NewService()
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
	if len(svc.platforms) != 0 {
		t.Fatalf("expected 0 platforms, got %d", len(svc.platforms))
	}
}

func TestRegisterPlatform(t *testing.T) {
	svc := NewService()

	p1 := &mockPlatform{name: "platform-a"}
	p2 := &mockPlatform{name: "platform-b"}

	result := svc.RegisterPlatform(p1).RegisterPlatform(p2)

	if result != svc {
		t.Fatal("RegisterPlatform should return the same service for chaining")
	}
	if len(svc.platforms) != 2 {
		t.Fatalf("expected 2 platforms, got %d", len(svc.platforms))
	}
}

func TestRegisterPlatformNil(t *testing.T) {
	svc := NewService()
	svc.RegisterPlatform(nil)

	if len(svc.platforms) != 0 {
		t.Fatalf("expected 0 platforms after nil register, got %d", len(svc.platforms))
	}
}

func TestRegisterPlatformDuplicate(t *testing.T) {
	svc := NewService()

	p1 := &mockPlatform{name: "same-name"}
	p2 := &mockPlatform{name: "same-name"}

	svc.RegisterPlatform(p1).RegisterPlatform(p2)

	if len(svc.platforms) != 1 {
		t.Fatalf("expected 1 platform after duplicate, got %d", len(svc.platforms))
	}
}

func TestShouldRun(t *testing.T) {
	svc := NewService()
	svc.AllowedPlatformsMap = map[string]bool{
		"freelance.de": true,
		"solcom.de":    true,
	}

	tests := []struct {
		name   string
		expect bool
	}{
		{"freelance.de", true},
		{"solcom.de", true},
		{"unknown.de", false},
		{"", false},
		{"  ", false},
	}

	for _, tt := range tests {
		if got := svc.shouldRun(tt.name); got != tt.expect {
			t.Errorf("shouldRun(%q) = %v, want %v", tt.name, got, tt.expect)
		}
	}
}

func TestReadAllowedPlatforms(t *testing.T) {
	t.Setenv("PLATFORMS", "freelance.de, solcom.de , peak-one.de")

	svc := NewService()
	svc.readAllowedPlatforms()

	expected := []string{"freelance.de", "solcom.de", "peak-one.de"}
	for _, name := range expected {
		if !svc.AllowedPlatformsMap[name] {
			t.Errorf("expected %q in allowed platforms", name)
		}
	}

	if len(svc.AllowedPlatformsMap) != 3 {
		t.Errorf("expected 3 allowed platforms, got %d", len(svc.AllowedPlatformsMap))
	}
}

func TestReadAllowedPlatformsDefault(t *testing.T) {
	// When PLATFORMS is empty, ReadOptional falls back to "hblabs.co".
	t.Setenv("PLATFORMS", "")

	svc := NewService()
	svc.readAllowedPlatforms()

	if !svc.AllowedPlatformsMap["hblabs.co"] {
		t.Error("expected default 'hblabs.co' in allowed platforms")
	}
}
