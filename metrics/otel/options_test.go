package xpgotel

import (
	"context"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestWithMetrics(t *testing.T) {
	t.Parallel()

	config, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}

	pool, err := xpg.New(
		context.Background(),
		config,
		xpg.WithName("test"),
		WithMetrics(
			WithMeterProvider(noop.NewMeterProvider()),
		),
	)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	pool.Close()
	pool.Close()
}

func TestWithMetricsNilProvider(t *testing.T) {
	t.Parallel()

	config, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}

	pool, err := xpg.New(
		context.Background(),
		config,
		WithMetrics(WithMeterProvider(nil)),
	)
	if pool != nil {
		pool.Close()
		t.Fatal("New returned a pool with a nil MeterProvider")
	}
	if err == nil || !strings.Contains(err.Error(), "meter provider is nil") {
		t.Fatalf("unexpected error: %v", err)
	}
}
