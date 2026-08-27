package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/topology/cluster"
	"github.com/mkbeh/xpg/topology/shard"
)

const (
	defaultShardADatabaseURL = "postgres://postgres:postgres@localhost:56431/postgres?sslmode=disable"
	defaultShardBDatabaseURL = "postgres://postgres:postgres@localhost:56432/postgres?sslmode=disable"

	shardAID shard.ID = "shard-a"
	shardBID shard.ID = "shard-b"
)

func openTopology(ctx context.Context) (*shard.Topology, error) {
	clusterA, err := openCluster(
		ctx,
		shardAID,
		"shard.shard-a.primary",
		environment(
			"XPG_SHARD_A_DATABASE_URL",
			defaultShardADatabaseURL,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("open shard-a cluster: %w", err)
	}

	clusterB, err := openCluster(
		ctx,
		shardBID,
		"shard.shard-b.primary",
		environment(
			"XPG_SHARD_B_DATABASE_URL",
			defaultShardBDatabaseURL,
		),
	)
	if err != nil {
		clusterA.Close()

		return nil, fmt.Errorf("open shard-b cluster: %w", err)
	}

	// After NewTopology succeeds, the topology owns both clusters and closes
	// them through Topology.Close. On constructor failure ownership remains here.
	topology, err := shard.NewTopology(
		clusterA,
		clusterB,
	)
	if err != nil {
		clusterB.Close()
		clusterA.Close()

		return nil, fmt.Errorf("create topology: %w", err)
	}

	return topology, nil
}

func openCluster(
	ctx context.Context,
	id shard.ID,
	name string,
	databaseURL string,
) (*cluster.Cluster, error) {
	pool, err := xpg.Open(
		ctx,
		databaseURL,
		xpg.WithName(name),
		xpg.WithLabel("xpg.shard.id", string(id)),
		xpg.WithLabel("xpg.pool.role", "primary"),
	)
	if err != nil {
		return nil, fmt.Errorf("open pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping pool: %w", err)
	}

	dbCluster, err := cluster.New(cluster.Config{
		ID:      id,
		Primary: pool,
	})
	if err != nil {
		pool.Close()

		return nil, fmt.Errorf("create cluster: %w", err)
	}

	return dbCluster, nil
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
