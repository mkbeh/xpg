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

func TestSlogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		pgx  tracelog.LogLevel
		want slog.Level
	}{
		{
			name: "trace",
			pgx:  tracelog.LogLevelTrace,
			want: slog.LevelDebug - 1,
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
			name: "unknown",
			pgx:  tracelog.LogLevel(255),
			want: slog.LevelError,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := slogLevel(test.pgx); got != test.want {
				t.Fatalf("slogLevel(%v) = %v, want %v", test.pgx, got, test.want)
			}
		})
	}
}

func TestLoggerLog(t *testing.T) {
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

	if record.Level != slog.LevelInfo {
		t.Fatalf("level = %v, want %v", record.Level, slog.LevelInfo)
	}

	attrs := recordAttrs(&record)

	if attrs["sql"] != "select 1" {
		t.Fatalf("sql = %v, want %q", attrs["sql"], "select 1")
	}

	if attrs["args"] != int64(1) {
		t.Fatalf("args = %v, want %v", attrs["args"], int64(1))
	}
}

func TestLoggerUnknownLevel(t *testing.T) {
	t.Parallel()

	handler := &captureHandler{}
	logger := New(slog.New(handler))

	logger.Log(
		context.Background(),
		tracelog.LogLevel(255),
		"unknown",
		nil,
	)

	record := handler.record

	if record.Message != "unknown" {
		t.Fatalf("message = %q, want %q", record.Message, "unknown")
	}

	if record.Level != slog.LevelError {
		t.Fatalf("level = %v, want %v", record.Level, slog.LevelError)
	}
}

type captureHandler struct {
	record slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *captureHandler) Handle(
	_ context.Context,
	record slog.Record, //nolint:gocritic // slog.Handler requires slog.Record by value.
) error {
	h.record = record.Clone()

	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *captureHandler) WithGroup(string) slog.Handler {
	return h
}

func recordAttrs(record *slog.Record) map[string]any {
	attrs := make(map[string]any, record.NumAttrs())

	record.Attrs(func(attr slog.Attr) bool {
		attrs[attr.Key] = attr.Value.Any()

		return true
	})

	return attrs
}
