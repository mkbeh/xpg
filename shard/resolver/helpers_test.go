package resolver

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/cluster"
	"github.com/mkbeh/xpg/shard"
)

const testDatabaseURL = "postgres://postgres@127.0.0.1:1/postgres?sslmode=disable"

func newTestTopology(t *testing.T, ids ...shard.ID) *shard.Topology {
	t.Helper()

	configs := make([]shard.Config, len(ids))

	for index, id := range ids {
		poolConfig, err := pgxpool.ParseConfig(testDatabaseURL)
		if err != nil {
			t.Fatalf("pgxpool.ParseConfig() error = %v", err)
		}

		poolConfig.MinConns = 0
		poolConfig.MaxConns = 1

		pool, err := xpg.New(
			context.Background(),
			poolConfig,
			xpg.WithName("shard."+string(id)+".primary"),
		)
		if err != nil {
			t.Fatalf("xpg.New() error = %v", err)
		}

		shardCluster, err := cluster.New(cluster.Config{
			ID:      id,
			Primary: pool,
		})
		if err != nil {
			pool.Close()
			t.Fatalf("cluster.New() error = %v", err)
		}

		t.Cleanup(shardCluster.Close)

		configs[index] = shard.Config{Cluster: shardCluster}
	}

	topology, err := shard.NewTopology(configs)
	if err != nil {
		t.Fatalf("shard.NewTopology() error = %v", err)
	}

	t.Cleanup(topology.Close)

	return topology
}
