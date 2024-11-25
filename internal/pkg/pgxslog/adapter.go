package pgxslog

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5/tracelog"
)

type Logger struct {
	l *slog.Logger
}

func NewLogger(l *slog.Logger) *Logger {
	return &Logger{l}
}

func (l *Logger) Log(ctx context.Context, level tracelog.LogLevel, msg string, data map[string]interface{}) {
	attrs := make([]slog.Attr, 0, len(data))
	for k, v := range data {
		attrs = append(attrs, slog.Any(k, v))
	}

	var lvl slog.Level
	switch level {
	case tracelog.LogLevelTrace:
		lvl = slog.LevelDebug - 1
		attrs = append(attrs, slog.Any("PGX_LOG_LEVEL", level))
	case tracelog.LogLevelDebug:
		lvl = slog.LevelDebug
	case tracelog.LogLevelInfo:
		lvl = slog.LevelInfo
	case tracelog.LogLevelWarn:
		lvl = slog.LevelWarn
	case tracelog.LogLevelError:
		lvl = slog.LevelError
	default:
		lvl = slog.LevelError
		attrs = append(attrs, slog.Any("INVALID_PGX_LOG_LEVEL", level))
	}
	//nolint:sloglint // there is no other option to pass the message
	l.l.LogAttrs(ctx, lvl, msg, attrs...)
}

// all key constants must be defined with the "Key" suffix.

// Constants and constructors to standardize the keys used in logs.

const (
	// errorKey - to pass error to log.
	errorKey string = "error"
	// componentKey - to identify the component that is logging (e.g.: "kafka-consumer")
	componentKey = "component"
)

// Error for passing error to log.
func Error(err error) slog.Attr { return slog.Any(errorKey, err) }

// Component to identify the component that is logging (e.g.: "kafka-consumer").
// here any distinct level of your application abstraction can be used.
// it's not mandatory associated with kind of an external connection,
// e.g.: "location-event-processor" is also applicable for the field.
func Component(component string) slog.Attr { return slog.String(componentKey, component) }
