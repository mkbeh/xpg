// Package slogxpg adapts log/slog loggers to pgx tracelog.Logger.
package slogxpg

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/tracelog"
)

const pgxLogLevelKey = "pgx_log_level"

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

func (l *adapter) Log(
	ctx context.Context,
	level tracelog.LogLevel,
	msg string,
	data map[string]any,
) {
	attrs := make([]slog.Attr, 0, len(data)+1)

	for key, value := range data {
		attrs = append(attrs, slog.Any(key, value))
	}

	var logLevel slog.Level

	switch level {
	case tracelog.LogLevelTrace:
		logLevel = slog.LevelDebug - 1
		attrs = append(
			attrs,
			slog.String(pgxLogLevelKey, level.String()),
		)
	case tracelog.LogLevelDebug:
		logLevel = slog.LevelDebug
	case tracelog.LogLevelInfo:
		logLevel = slog.LevelInfo
	case tracelog.LogLevelWarn:
		logLevel = slog.LevelWarn
	case tracelog.LogLevelError:
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelError
		attrs = append(
			attrs,
			slog.String(pgxLogLevelKey, level.String()),
		)
	}

	l.logger.LogAttrs(
		ctx,
		logLevel,
		msg,
		attrs...,
	)
}
