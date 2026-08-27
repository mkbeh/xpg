package main

import (
	"context"
	"fmt"
	"os"

	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/topology/cluster"
)

const (
	defaultPrimaryDatabaseURL = "postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable&target_session_attrs=read-write"
	defaultReplicaOneURL      = "postgres://postgres:postgres@localhost:55433/postgres?sslmode=disable&target_session_attrs=read-only"
	defaultReplicaTwoURL      = "postgres://postgres:postgres@localhost:55434/postgres?sslmode=disable&target_session_attrs=read-only"
)

func openCluster(ctx context.Context) (*cluster.Cluster, error) {
	primary, err := openPool(
		ctx,
		environment("XPG_PRIMARY_DATABASE_URL", defaultPrimaryDatabaseURL),
		"cluster.primary",
		"primary",
	)
	if err != nil {
		return nil, fmt.Errorf("open primary pool: %w", err)
	}

	replicaOne, err := openPool(
		ctx,
		environment("XPG_REPLICA_ONE_DATABASE_URL", defaultReplicaOneURL),
		"cluster.replica-one",
		"replica",
	)
	if err != nil {
		primary.Close()

		return nil, fmt.Errorf("open replica-one pool: %w", err)
	}

	replicaTwo, err := openPool(
		ctx,
		environment("XPG_REPLICA_TWO_DATABASE_URL", defaultReplicaTwoURL),
		"cluster.replica-two",
		"replica",
	)
	if err != nil {
		replicaOne.Close()
		primary.Close()

		return nil, fmt.Errorf("open replica-two pool: %w", err)
	}

	dbCluster, err := cluster.New(cluster.Config{
		ID:       "cluster-example",
		Primary:  primary,
		Replicas: []*xpg.Pool{replicaOne, replicaTwo},
	})
	if err != nil {
		replicaTwo.Close()
		replicaOne.Close()
		primary.Close()

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

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
