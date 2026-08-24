package slogxpg

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/tracelog"
)

type adapter struct {
	logger *slog.Logger
}

var _ tracelog.Logger = (*adapter)(nil)

// New adapts logger to pgx tracelog.Logger.
//
// New returns nil when logger is nil.
func New(logger *slog.Logger) tracelog.Logger {
	if logger == nil {
		return nil
	}

	return &adapter{
		logger: logger,
	}
}

func (a *adapter) Log(
	ctx context.Context,
	level tracelog.LogLevel,
	msg string,
	data map[string]any,
) {
	attrs := make([]slog.Attr, 0, len(data))

	for key, value := range data {
		attrs = append(attrs, slog.Any(key, value))
	}

	a.logger.LogAttrs(
		ctx,
		slogLevel(level),
		msg, //nolint:sloglint // pgx provides the log message dynamically.
		attrs...,
	)
}

func slogLevel(level tracelog.LogLevel) slog.Level {
	switch level {
	case tracelog.LogLevelTrace:
		return slog.LevelDebug - 1
	case tracelog.LogLevelDebug:
		return slog.LevelDebug
	case tracelog.LogLevelInfo:
		return slog.LevelInfo
	case tracelog.LogLevelWarn:
		return slog.LevelWarn
	case tracelog.LogLevelError:
		return slog.LevelError
	default:
		return slog.LevelError
	}
}
