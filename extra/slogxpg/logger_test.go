package slogxpg

import (
	"context"
	"log/slog"
	"testing"

	"github.com/jackc/pgx/v5/tracelog"
)

func TestNewNil(t *testing.T) {
	t.Parallel()

	if logger := New(nil); logger != nil {
		t.Fatal("expected nil logger")
	}
}

func TestLoggerLevels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		pgx     tracelog.LogLevel
		want    slog.Level
		wantPGX string
	}{
		{
			name:    "trace",
			pgx:     tracelog.LogLevelTrace,
			want:    slog.LevelDebug - 1,
			wantPGX: tracelog.LogLevelTrace.String(),
		},
		{
			name: "debug",
			pgx:  tracelog.LogLevelDebug,
			want: slog.LevelDebug,
		},
		{
			name: "info",
			pgx:  tracelog.LogLevelInfo,
			want: slog.LevelInfo,
		},
		{
			name: "warn",
			pgx:  tracelog.LogLevelWarn,
			want: slog.LevelWarn,
		},
		{
			name: "error",
			pgx:  tracelog.LogLevelError,
			want: slog.LevelError,
		},
		{
			name:    "unknown",
			pgx:     tracelog.LogLevel(255),
			want:    slog.LevelError,
			wantPGX: tracelog.LogLevel(255).String(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			handler := &captureHandler{}
			logger := New(slog.New(handler))

			logger.Log(
				context.Background(),
				test.pgx,
				"message",
				nil,
			)

			record := handler.record

			if record.Level != test.want {
				t.Fatalf("level = %v, want %v", record.Level, test.want)
			}

			attrs := recordAttrs(record)

			if test.wantPGX == "" {
				if _, ok := attrs[pgxLogLevelKey]; ok {
					t.Fatalf("unexpected %s attribute", pgxLogLevelKey)
				}

				return
			}

			if attrs[pgxLogLevelKey] != test.wantPGX {
				t.Fatalf(
					"%s = %v, want %q",
					pgxLogLevelKey,
					attrs[pgxLogLevelKey],
					test.wantPGX,
				)
			}
		})
	}
}

func TestLoggerData(t *testing.T) {
	t.Parallel()

	handler := &captureHandler{}
	logger := New(slog.New(handler))

	logger.Log(
		context.Background(),
		tracelog.LogLevelInfo,
		"query",
		map[string]any{
			"sql":  "select 1",
			"args": 1,
		},
	)

	record := handler.record

	if record.Message != "query" {
		t.Fatalf("message = %q, want %q", record.Message, "query")
	}

	attrs := recordAttrs(record)

	if attrs["sql"] != "select 1" {
		t.Fatalf("sql = %v, want %q", attrs["sql"], "select 1")
	}

	if attrs["args"] != int64(1) {
		t.Fatalf("args = %v, want %v", attrs["args"], int64(1))
	}
}

type captureHandler struct {
	record slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(_ context.Context, record slog.Record) error {
	h.record = record.Clone()

	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}

func recordAttrs(record slog.Record) map[string]any {
	attrs := make(map[string]any, record.NumAttrs())

	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()

		return true
	})

	return attrs
}
