package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/cluster"
	"github.com/mkbeh/xpg/shard"
)

const (
	defaultShardADatabaseURL = "postgres://postgres:postgres@localhost:56431/postgres?sslmode=disable"
	defaultShardBDatabaseURL = "postgres://postgres:postgres@localhost:56432/postgres?sslmode=disable"

	shardAID shard.ID = "shard-a"
	shardBID shard.ID = "shard-b"
)

func openTopology(ctx context.Context) (*shard.Topology, error) {
	shardA, err := openShard(
		ctx,
		shardAID,
		"shard.shard-a.primary",
		environment(
			"XPG_SHARD_A_DATABASE_URL",
			defaultShardADatabaseURL,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("open shard-a: %w", err)
	}

	shardB, err := openShard(
		ctx,
		shardBID,
		"shard.shard-b.primary",
		environment(
			"XPG_SHARD_B_DATABASE_URL",
			defaultShardBDatabaseURL,
		),
	)
	if err != nil {
		shardA.Close()

		return nil, fmt.Errorf("open shard-b: %w", err)
	}

	topology, err := shard.NewTopology([]shard.Config{
		{Cluster: shardA},
		{Cluster: shardB},
	})
	if err != nil {
		shardB.Close()
		shardA.Close()

		return nil, fmt.Errorf("create topology: %w", err)
	}

	return topology, nil
}

func openShard(ctx context.Context, id shard.ID, name, databaseURL string) (*cluster.Cluster, error) {
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

	shardCluster, err := cluster.New(cluster.Config{
		ID:      id,
		Primary: pool,
	})
	if err != nil {
		pool.Close()

		return nil, fmt.Errorf("create cluster: %w", err)
	}

	return shardCluster, nil
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
