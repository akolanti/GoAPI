package logger_i

import (
	"context"
	"log/slog"
	"os"
	"runtime"

	"github.com/akolanti/GoAPI/internal/config"
)

type Logger struct {
	inner *slog.Logger
}

func Init() {
	options := &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}

	var handler slog.Handler
	if config.IS_PROD {
		options.Level = config.LOG_LEVEL_PROD
		handler = slog.NewJSONHandler(os.Stdout, options)

	} else {
		handler = slog.NewTextHandler(os.Stdout, options)

	}
	newLogger := slog.New(handler)
	slog.SetDefault(newLogger)
}

func NewLogger(section string) *Logger {
	return &Logger{
		inner: slog.Default().With("component", section),
	}
}

func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

func (l *Logger) Error(msg string, args ...any) {
	l.logWithSource(slog.LevelError, msg, args...)
}

func (l *Logger) Warn(msg string, args ...any) {
	l.logWithSource(slog.LevelWarn, msg, args...)
}

func (l *Logger) Debug(msg string, args ...any) {
	l.logWithSource(slog.LevelDebug, msg, args...)
}

func (l *Logger) logWithSource(level slog.Level, msg string, args ...any) {
	if !l.inner.Enabled(context.Background(), level) {
		return
	}
	var pcs [1]uintptr
	// Skip 3 levels: runtime.Callers, logWithSource, and Err/Dbg wrapper - this looks at GO's stack trace
	runtime.Callers(3, pcs[:])
	l.inner.Log(context.Background(), level, msg, args...)
}

func (l *Logger) With(args ...any) *Logger {
	return &Logger{
		inner: l.inner.With(args...),
	}
}
