package resolver

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/topology/cluster"
	"github.com/mkbeh/xpg/topology/shard"
)

const testDatabaseURL = "postgres://postgres@127.0.0.1:1/postgres?sslmode=disable"

func newTestTopology(t *testing.T, ids ...shard.ID) *shard.Topology {
	t.Helper()

	clusters := make([]*cluster.Cluster, 0, len(ids))

	closeClusters := func() {
		for index := len(clusters) - 1; index >= 0; index-- {
			clusters[index].Close()
		}
	}

	for _, id := range ids {
		poolConfig, err := pgxpool.ParseConfig(testDatabaseURL)
		if err != nil {
			closeClusters()
			t.Fatalf("pgxpool.ParseConfig() error = %v", err)
		}

		poolConfig.MinConns = 0
		poolConfig.MaxConns = 1

		pool, err := xpg.New(
			t.Context(),
			poolConfig,
			xpg.WithName("shard."+string(id)+".primary"),
		)
		if err != nil {
			closeClusters()
			t.Fatalf("xpg.New() error = %v", err)
		}

		dbCluster, err := cluster.New(cluster.Config{
			ID:      id,
			Primary: pool,
		})
		if err != nil {
			pool.Close()
			closeClusters()
			t.Fatalf("cluster.New() error = %v", err)
		}

		clusters = append(clusters, dbCluster)
	}

	topology, err := shard.NewTopology(clusters...)
	if err != nil {
		closeClusters()
		t.Fatalf("shard.NewTopology() error = %v", err)
	}

	t.Cleanup(topology.Close)

	return topology
}
