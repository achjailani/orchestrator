package orchestrator

import std "log"

// Logger defines a standard logging interface.
type Logger interface {
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// DefaultLogger wraps the standard Go log package.
type DefaultLogger struct{}

func (l *DefaultLogger) Infof(format string, args ...any)  { std.Printf("[INFO] "+format, args...) }
func (l *DefaultLogger) Warnf(format string, args ...any)  { std.Printf("[WARN] "+format, args...) }
func (l *DefaultLogger) Errorf(format string, args ...any) { std.Printf("[ERROR] "+format, args...) }

type zeroLogger struct{}

// ZeroLogger returns a logger that discards all logged output.
func ZeroLogger() Logger {
	return &zeroLogger{}
}

var _ Logger = (*zeroLogger)(nil)

func (*zeroLogger) Infof(format string, args ...any)  {}
func (*zeroLogger) Warnf(format string, args ...any)  {}
func (*zeroLogger) Errorf(format string, args ...any) {}
