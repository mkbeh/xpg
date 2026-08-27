package main

import (
	"context"
	"fmt"
	"os"
	"slices"

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
	type shardConfig struct {
		id          cluster.ID
		region      string
		name        string
		databaseURL string
	}

	configs := []shardConfig{
		{
			id:     shardEUID,
			region: "eu",
			name:   "geo.shard-eu.primary",
			databaseURL: environment(
				"XPG_SHARD_EU_DATABASE_URL",
				defaultShardEUDatabaseURL,
			),
		},
		{
			id:     shardUSID,
			region: "us",
			name:   "geo.shard-us.primary",
			databaseURL: environment(
				"XPG_SHARD_US_DATABASE_URL",
				defaultShardUSDatabaseURL,
			),
		},
	}

	clusters := make([]*cluster.Cluster, 0, len(configs))

	for _, config := range configs {
		dbCluster, err := openShard(
			ctx,
			config.id,
			config.region,
			config.name,
			config.databaseURL,
		)
		if err != nil {
			closeClusters(clusters)

			return nil, fmt.Errorf("open %s shard: %w", config.id, err)
		}

		clusters = append(clusters, dbCluster)
	}

	topology, err := shard.NewTopology(clusters...)
	if err != nil {
		closeClusters(clusters)

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

func closeClusters(clusters []*cluster.Cluster) {
	for _, cluster := range slices.Backward(clusters) {
		cluster.Close()
	}
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
