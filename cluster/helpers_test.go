package cluster

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
)

const testDatabaseURL = "postgres://postgres:postgres@127.0.0.1:1/postgres?sslmode=disable" //nolint:gosec // Test-only DSN with non-production credentials.

func newTestPool(t *testing.T, name string, labels map[string]string) *xpg.Pool {
	t.Helper()

	config, err := pgxpool.ParseConfig(testDatabaseURL)
	if err != nil {
		t.Fatalf("pgxpool.ParseConfig() error = %v", err)
	}

	config.MinConns = 0
	config.MaxConns = 1

	options := []xpg.Option{
		xpg.WithName(name),
	}

	if len(labels) != 0 {
		options = append(options, xpg.WithLabels(labels))
	}

	pool, err := xpg.New(t.Context(), config, options...)
	if err != nil {
		t.Fatalf("xpg.New() error = %v", err)
	}
	t.Cleanup(pool.Close)

	return pool
}

func newTestCluster(t *testing.T, config Config) *Cluster {
	t.Helper()

	cluster, err := New(config)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(cluster.Close)

	return cluster
}
