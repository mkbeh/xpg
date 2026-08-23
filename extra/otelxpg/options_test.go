package otelxpg

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestMetricsRegistration(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(
		WithMeterProvider(noop.NewMeterProvider()),
	)

	pool := newTestPool(t, metrics)
	pool.Close()
	pool.Close()
}

func TestWithMeterProviderNilUsesGlobalProvider(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(
		WithMeterProvider(nil),
	)

	pool := newTestPool(t, metrics)
	pool.Close()
}

func newTestPool(t *testing.T, metrics xpg.Metrics) *xpg.Pool {
	t.Helper()

	poolConfig, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatalf("parse pool config: %v", err)
	}

	pool, err := xpg.New(
		context.Background(),
		poolConfig,
		xpg.WithName("test-pool"),
		xpg.WithMetrics(metrics),
	)
	if err != nil {
		t.Fatalf("create pool: %v", err)
	}

	return pool
}
