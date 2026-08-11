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
	defaultShardAPrimaryDatabaseURL = "postgres://postgres:postgres@localhost:56431/postgres?sslmode=disable&target_session_attrs=read-write"
	defaultShardBPrimaryDatabaseURL = "postgres://postgres:postgres@localhost:56432/postgres?sslmode=disable&target_session_attrs=read-write"
	defaultShardBReplicaDatabaseURL = "postgres://postgres:postgres@localhost:56433/postgres?sslmode=disable&target_session_attrs=read-only"

	shardAID shard.ID = "shard-a"
	shardBID shard.ID = "shard-b"
)

type poolSpec struct {
	databaseURL string
	name        string
}

type shardSpec struct {
	id       shard.ID
	primary  poolSpec
	replicas []poolSpec
}

func openTopology(ctx context.Context) (*shard.Topology, error) {
	specs := []shardSpec{
		{
			id: shardAID,
			primary: poolSpec{
				databaseURL: environment(
					"XPG_SHARD_A_PRIMARY_DATABASE_URL",
					defaultShardAPrimaryDatabaseURL,
				),
				name: "shard.shard-a.primary",
			},
		},
		{
			id: shardBID,
			primary: poolSpec{
				databaseURL: environment(
					"XPG_SHARD_B_PRIMARY_DATABASE_URL",
					defaultShardBPrimaryDatabaseURL,
				),
				name: "shard.shard-b.primary",
			},
			replicas: []poolSpec{
				{
					databaseURL: environment(
						"XPG_SHARD_B_REPLICA_DATABASE_URL",
						defaultShardBReplicaDatabaseURL,
					),
					name: "shard.shard-b.replica",
				},
			},
		},
	}

	clusters := make([]*cluster.Cluster, 0, len(specs))
	configs := make([]shard.Config, 0, len(specs))

	for _, spec := range specs {
		dbCluster, err := openCluster(ctx, spec)
		if err != nil {
			closeClusters(clusters)

			return nil, fmt.Errorf("open %s cluster: %w", spec.id, err)
		}

		clusters = append(clusters, dbCluster)
		configs = append(configs, shard.Config{Cluster: dbCluster})
	}

	// Topology takes ownership of the clusters only after successful creation.
	topology, err := shard.NewTopology(configs)
	if err != nil {
		closeClusters(clusters)

		return nil, fmt.Errorf("create topology: %w", err)
	}

	return topology, nil
}

func openCluster(ctx context.Context, spec shardSpec) (*cluster.Cluster, error) {
	primary, err := openPool(ctx, spec.primary)
	if err != nil {
		return nil, fmt.Errorf("open primary %s: %w", spec.primary.name, err)
	}

	pools := []*xpg.Pool{primary}
	replicas := make([]*xpg.Pool, 0, len(spec.replicas))

	for _, replicaSpec := range spec.replicas {
		replica, err := openPool(ctx, replicaSpec)
		if err != nil {
			closePools(pools)

			return nil, fmt.Errorf("open replica %s: %w", replicaSpec.name, err)
		}

		pools = append(pools, replica)
		replicas = append(replicas, replica)
	}

	config := cluster.Config{
		ID:       spec.id,
		Primary:  primary,
		Replicas: replicas,
	}

	if len(replicas) > 0 {
		// Keep replica selection explicit in the runnable example.
		config.Selector = cluster.RoundRobinSelector()
	}

	dbCluster, err := cluster.New(config)
	if err != nil {
		closePools(pools)

		return nil, fmt.Errorf("create cluster: %w", err)
	}

	return dbCluster, nil
}

func openPool(ctx context.Context, spec poolSpec) (*xpg.Pool, error) {
	pool, err := xpg.Open(
		ctx,
		spec.databaseURL,
		xpg.WithName(spec.name),
	)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", spec.name, err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping %s: %w", spec.name, err)
	}

	return pool, nil
}

func closePools(pools []*xpg.Pool) {
	for index := len(pools) - 1; index >= 0; index-- {
		pools[index].Close()
	}
}

func closeClusters(clusters []*cluster.Cluster) {
	for index := len(clusters) - 1; index >= 0; index-- {
		clusters[index].Close()
	}
}

func environment(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}

	return fallback
}
