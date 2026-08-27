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
	defaultShardADatabaseURL = "postgres://postgres:postgres@localhost:56431/postgres?sslmode=disable"
	defaultShardBDatabaseURL = "postgres://postgres:postgres@localhost:56432/postgres?sslmode=disable"

	shardAID shard.ID = "shard-a"
	shardBID shard.ID = "shard-b"
)

func openTopology(ctx context.Context) (*shard.Topology, error) {
	type clusterConfig struct {
		id          cluster.ID
		name        string
		databaseURL string
	}

	configs := []clusterConfig{
		{
			id:   shardAID,
			name: "shard.shard-a.primary",
			databaseURL: environment(
				"XPG_SHARD_A_DATABASE_URL",
				defaultShardADatabaseURL,
			),
		},
		{
			id:   shardBID,
			name: "shard.shard-b.primary",
			databaseURL: environment(
				"XPG_SHARD_B_DATABASE_URL",
				defaultShardBDatabaseURL,
			),
		},
	}

	clusters := make([]*cluster.Cluster, 0, len(configs))

	for _, config := range configs {
		dbCluster, err := openCluster(
			ctx,
			config.id,
			config.name,
			config.databaseURL,
		)
		if err != nil {
			closeClusters(clusters)

			return nil, fmt.Errorf("open %s cluster: %w", config.id, err)
		}

		clusters = append(clusters, dbCluster)
	}

	// After NewTopology succeeds, the topology owns all clusters and closes
	// them through Topology.Close. On constructor failure ownership remains here.
	topology, err := shard.NewTopology(clusters...)
	if err != nil {
		closeClusters(clusters)

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
