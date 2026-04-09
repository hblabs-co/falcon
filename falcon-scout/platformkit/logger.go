package platformkit

type Logger interface {
	Info(args ...any)
	Infof(format string, args ...any)
	Error(args ...any)
	Errorf(format string, args ...any)
}

type NoopLogger struct{}

func (NoopLogger) Info(args ...any)                  {}
func (NoopLogger) Infof(format string, args ...any)  {}
func (NoopLogger) Error(args ...any)                 {}
func (NoopLogger) Errorf(format string, args ...any) {}

func ResolveLogger(logger any) Logger {
	if l, ok := logger.(Logger); ok {
		return l
	}
	return NoopLogger{}
}
