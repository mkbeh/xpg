package shard

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/topology/cluster"
)

const testDatabaseURL = "postgres://postgres@127.0.0.1:1/postgres?sslmode=disable"

func newTestCluster(t *testing.T, id ID, labels map[string]string) *cluster.Cluster {
	t.Helper()

	config, err := pgxpool.ParseConfig(testDatabaseURL)
	if err != nil {
		t.Fatalf("pgxpool.ParseConfig() error = %v", err)
	}

	config.MinConns = 0
	config.MaxConns = 1

	pool, err := xpg.New(
		t.Context(),
		config,
		xpg.WithName("shard."+string(id)+".primary"),
	)
	if err != nil {
		t.Fatalf("xpg.New() error = %v", err)
	}

	dbCluster, err := cluster.New(cluster.Config{
		ID:      id,
		Labels:  labels,
		Primary: pool,
	})
	if err != nil {
		pool.Close()
		t.Fatalf("cluster.New() error = %v", err)
	}

	t.Cleanup(dbCluster.Close)

	return dbCluster
}

func newTestTopology(t *testing.T, ids ...ID) *Topology {
	t.Helper()

	clusters := make([]*cluster.Cluster, len(ids))
	for index, id := range ids {
		clusters[index] = newTestCluster(t, id, nil)
	}

	topology, err := NewTopology(clusters...)
	if err != nil {
		t.Fatalf("NewTopology() error = %v", err)
	}

	t.Cleanup(topology.Close)

	return topology
}

type testResolverFunc[K any] func(K) (Shard, error)

func (resolve testResolverFunc[K]) Resolve(key K) (Shard, error) {
	return resolve(key)
}
