package main

import (
	"context"
	"fmt"
	"os"
	"slices"

	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/topology/cluster"
)

const (
	defaultPrimaryDatabaseURL = "postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable&target_session_attrs=read-write"
	defaultReplicaOneURL      = "postgres://postgres:postgres@localhost:55433/postgres?sslmode=disable&target_session_attrs=read-only"
	defaultReplicaTwoURL      = "postgres://postgres:postgres@localhost:55434/postgres?sslmode=disable&target_session_attrs=read-only"
)

func openCluster(ctx context.Context) (*cluster.Cluster, error) {
	type poolConfig struct {
		databaseURL string
		name        string
		role        string
	}

	configs := []poolConfig{
		{
			databaseURL: environment(
				"XPG_PRIMARY_DATABASE_URL",
				defaultPrimaryDatabaseURL,
			),
			name: "cluster.primary",
			role: "primary",
		},
		{
			databaseURL: environment(
				"XPG_REPLICA_ONE_DATABASE_URL",
				defaultReplicaOneURL,
			),
			name: "cluster.replica-one",
			role: "replica",
		},
		{
			databaseURL: environment(
				"XPG_REPLICA_TWO_DATABASE_URL",
				defaultReplicaTwoURL,
			),
			name: "cluster.replica-two",
			role: "replica",
		},
	}

	pools := make([]*xpg.Pool, 0, len(configs))

	for _, config := range configs {
		pool, err := openPool(
			ctx,
			config.databaseURL,
			config.name,
			config.role,
		)
		if err != nil {
			closePools(pools)

			return nil, fmt.Errorf("open %s pool: %w", config.name, err)
		}

		pools = append(pools, pool)
	}

	dbCluster, err := cluster.New(cluster.Config{
		ID:       "cluster-example",
		Primary:  pools[0],
		Replicas: pools[1:],
	})
	if err != nil {
		closePools(pools)

		return nil, fmt.Errorf("create cluster: %w", err)
	}

	return dbCluster, nil
}

func openPool(ctx context.Context, databaseURL, name, role string) (*xpg.Pool, error) {
	pool, err := xpg.Open(
		ctx,
		databaseURL,
		xpg.WithName(name),
		xpg.WithLabel("xpg.pool.role", role),
	)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping pool: %w", err)
	}

	return pool, nil
}

func closePools(pools []*xpg.Pool) {
	for _, pool := range slices.Backward(pools) {
		pool.Close()
	}
}

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
