package xpgotel

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
	xpgcache "github.com/mkbeh/xpg/cache"
	"go.opentelemetry.io/otel/metric/noop"
)

func TestMetricsRegistration(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(
		WithMeterProvider(noop.NewMeterProvider()),
	)

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
	pool.Close()
	pool.Close()

	cache, err := xpgcache.New[int](
		xpgcache.Config{
			Name:    "test-cache",
			TTL:     time.Minute,
			Metrics: metrics,
		},
	)
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}
	cache.Close()
	cache.Close()
}

func TestWithMeterProviderNilUsesGlobalProvider(t *testing.T) {
	t.Parallel()

	metrics := NewMetrics(
		WithMeterProvider(nil),
	)

	cache, err := xpgcache.New[int](
		xpgcache.Config{
			Name:    "test-cache",
			TTL:     time.Minute,
			Metrics: metrics,
		},
	)
	if err != nil {
		t.Fatalf("create cache: %v", err)
	}

	cache.Close()
}
