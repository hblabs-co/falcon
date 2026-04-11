package platformkit

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type Logger interface {
	Info(args ...any)
	Infof(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
	WithFields(fields map[string]any) Logger
}

type NoopLogger struct{}

func (NoopLogger) Info(args ...any)                          {}
func (NoopLogger) Infof(format string, args ...any)          {}
func (NoopLogger) Warn(args ...any)                          {}
func (NoopLogger) Warnf(format string, args ...any)          {}
func (NoopLogger) Error(args ...any)                         {}
func (NoopLogger) Errorf(format string, args ...any)         {}
func (n NoopLogger) WithFields(fields map[string]any) Logger { return n }

// basicLogger is satisfied by *logrus.Entry and similar loggers.
type basicLogger interface {
	Info(args ...any)
	Infof(format string, args ...any)
	Warn(args ...any)
	Warnf(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
}

// loggerAdapter wraps a basicLogger to implement Logger.
type loggerAdapter struct {
	inner any
}

func (a *loggerAdapter) Info(args ...any) { a.inner.(basicLogger).Info(args...) }
func (a *loggerAdapter) Infof(format string, args ...any) {
	a.inner.(basicLogger).Infof(format, args...)
}
func (a *loggerAdapter) Warn(args ...any) { a.inner.(basicLogger).Warn(args...) }
func (a *loggerAdapter) Warnf(format string, args ...any) {
	a.inner.(basicLogger).Warnf(format, args...)
}
func (a *loggerAdapter) Error(args ...any) { a.inner.(basicLogger).Error(args...) }
func (a *loggerAdapter) Errorf(format string, args ...any) {
	a.inner.(basicLogger).Errorf(format, args...)
}

func (a *loggerAdapter) WithFields(fields map[string]any) Logger {
	v := reflect.ValueOf(a.inner)
	m := v.MethodByName("WithFields")
	if !m.IsValid() {
		return a
	}
	result := m.Call([]reflect.Value{reflect.ValueOf(fields)})
	if len(result) == 0 {
		return a
	}
	return &loggerAdapter{inner: result[0].Interface()}
}

func ResolveLogger(logger any) Logger {
	if l, ok := logger.(Logger); ok {
		return l
	}
	if _, ok := logger.(basicLogger); ok {
		return &loggerAdapter{inner: logger}
	}
	return NoopLogger{}
}

// SaveFn is a callback injected by the service to persist a scraped project.
// The project parameter must implement interfaces.Project; the service handles the assertion.
// The existing parameter, when non-nil, is the previous PersistedProject (opaque to the runner)
// — typically obtained from FilterFn — so the service can preserve identity and ordering fields
// across updates without re-querying storage.
type SaveFn func(ctx context.Context, project any, existing any) error

// FilterFn checks which candidates should be skipped and returns the existing records
// for the ones that should be processed.
//
// Inputs:
//   - platform: the source identifier (e.g. "redglobal.de")
//   - updatedAt: map of candidate platform ID → its last-updated time from the listing.
//
// Returns:
//   - skip: set of platform IDs that should NOT be processed (already up-to-date, or
//     have pending errors that the retry worker handles).
//   - existing: opaque references to previously persisted records, keyed by platform ID.
//     The runner stores these and passes them back to SaveFn so the service can preserve
//     identity and display ordering across re-scrapes without an extra DB round-trip.
type FilterFn func(
	ctx context.Context,
	platform string,
	updatedAt map[string]time.Time,
) (skip map[string]bool, existing map[string]any, err error)

func ResolveConsumerName(source string) string {
	consumer := fmt.Sprintf("scout-%s", strings.ReplaceAll(source, ".", "-"))
	return consumer
}
