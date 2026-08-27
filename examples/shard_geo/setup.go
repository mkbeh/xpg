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
	defaultShardEUDatabaseURL = "postgres://postgres:postgres@localhost:57431/postgres?sslmode=disable"
	defaultShardUSDatabaseURL = "postgres://postgres:postgres@localhost:57432/postgres?sslmode=disable"

	shardEUID shard.ID = "shard-eu"
	shardUSID shard.ID = "shard-us"
)

func openTopology(ctx context.Context) (*shard.Topology, error) {
	shardEU, err := openShard(
		ctx,
		shardEUID,
		"eu",
		"geo.shard-eu.primary",
		environment(
			"XPG_SHARD_EU_DATABASE_URL",
			defaultShardEUDatabaseURL,
		),
	)
	if err != nil {
		return nil, fmt.Errorf("open shard-eu: %w", err)
	}

	shardUS, err := openShard(
		ctx,
		shardUSID,
		"us",
		"geo.shard-us.primary",
		environment(
			"XPG_SHARD_US_DATABASE_URL",
			defaultShardUSDatabaseURL,
		),
	)
	if err != nil {
		shardEU.Close()

		return nil, fmt.Errorf("open shard-us: %w", err)
	}

	topology, err := shard.NewTopology(
		shardEU,
		shardUS,
	)
	if err != nil {
		shardUS.Close()
		shardEU.Close()

		return nil, fmt.Errorf("create topology: %w", err)
	}

	return topology, nil
}

func openShard(
	ctx context.Context,
	id shard.ID,
	region string,
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

	shardCluster, err := cluster.New(cluster.Config{
		ID: id,
		Labels: map[string]string{
			"region": region,
		},
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
