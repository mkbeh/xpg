package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/cluster"
)

const (
	defaultPrimaryDatabaseURL    = "postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable&target_session_attrs=read-write"
	defaultReplicaOneDatabaseURL = "postgres://postgres:postgres@localhost:55433/postgres?sslmode=disable&target_session_attrs=read-only"
	defaultReplicaTwoDatabaseURL = "postgres://postgres:postgres@localhost:55434/postgres?sslmode=disable&target_session_attrs=read-only"
)

type nodeInfo struct {
	Name string
	Role string
}

type queryRower interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	dbCluster, err := openCluster(ctx)
	if err != nil {
		return err
	}
	defer dbCluster.Close()

	if err := showRouting(ctx, dbCluster); err != nil {
		return err
	}

	if err := showTransactions(ctx, dbCluster); err != nil {
		return err
	}

	return nil
}

func openCluster(ctx context.Context) (*cluster.Cluster, error) {
	nodes := []struct {
		databaseURL string
		name        string
		role        string
	}{
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
				defaultReplicaOneDatabaseURL,
			),
			name: "cluster.replica-one",
			role: "replica",
		},
		{
			databaseURL: environment(
				"XPG_REPLICA_TWO_DATABASE_URL",
				defaultReplicaTwoDatabaseURL,
			),
			name: "cluster.replica-two",
			role: "replica",
		},
	}

	pools := make([]*xpg.Pool, 0, len(nodes))

	for _, node := range nodes {
		pool, err := openPool(
			ctx,
			node.databaseURL,
			node.name,
			node.role,
		)
		if err != nil {
			closePools(pools)

			return nil, fmt.Errorf("open %s pool: %w", node.name, err)
		}

		pools = append(pools, pool)
	}

	// Build a replicated cluster and explicitly use round-robin selection for
	// replica reads.
	dbCluster, err := cluster.New(
		cluster.Config{
			Primary:  pools[0],
			Replicas: pools[1:],
			Selector: cluster.RoundRobinSelector(),
		},
	)
	if err != nil {
		return nil, fmt.Errorf("create cluster: %w", err)
	}

	return dbCluster, nil
}

func openPool(
	ctx context.Context,
	databaseURL string,
	name string,
	role string,
) (*xpg.Pool, error) {
	pool, err := xpg.Open(
		ctx,
		databaseURL,
		xpg.WithName(name),
		xpg.WithLabel("role", role),
	)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()

		return nil, fmt.Errorf("ping %s: %w", name, err)
	}

	return pool, nil
}

func closePools(pools []*xpg.Pool) {
	for index := len(pools) - 1; index >= 0; index-- {
		pools[index].Close()
	}
}

func showRouting(ctx context.Context, dbCluster *cluster.Cluster) error {
	primary := dbCluster.Primary()

	node, err := loadNode(ctx, primary)
	if err != nil {
		return fmt.Errorf("read primary node: %w", err)
	}

	fmt.Println("primary:")
	fmt.Printf(
		"- pool=%s node=%s role=%s\n",
		primary.Name(),
		node.Name,
		node.Role,
	)

	fmt.Println("replica reads:")

	// Read once per registered replica to demonstrate one complete round-robin
	// cycle without hard-coding the cluster size.
	for range dbCluster.ReplicaCount() {
		pool, err := dbCluster.ReadPool(
			ctx,
			cluster.ReadReplicaRequired,
		)
		if err != nil {
			return fmt.Errorf("resolve replica read: %w", err)
		}

		node, err := loadNode(ctx, pool)
		if err != nil {
			return fmt.Errorf("read replica node: %w", err)
		}

		fmt.Printf(
			"- pool=%s node=%s role=%s\n",
			pool.Name(),
			node.Name,
			node.Role,
		)
	}

	return nil
}

func showTransactions(ctx context.Context, dbCluster *cluster.Cluster) error {
	var (
		primaryNode     nodeInfo
		primaryReadOnly string
	)

	// Primary transactions use regular pgx transaction options and may perform
	// both reads and writes.
	err := dbCluster.InPrimaryTx(
		ctx,
		pgx.TxOptions{},
		func(ctx context.Context, tx pgx.Tx) error {
			var err error

			primaryNode, err = loadNode(ctx, tx)
			if err != nil {
				return err
			}

			return tx.QueryRow(
				ctx,
				"SHOW transaction_read_only",
			).Scan(&primaryReadOnly)
		},
	)
	if err != nil {
		return fmt.Errorf("run primary transaction: %w", err)
	}

	var (
		replicaNode     nodeInfo
		replicaReadOnly string
	)

	// Read transactions resolve their pool through ReadPolicy and always start
	// PostgreSQL transactions in READ ONLY mode.
	err = dbCluster.InReadTx(
		ctx,
		cluster.ReadReplicaRequired,
		cluster.ReadTxOptions{
			IsoLevel: pgx.RepeatableRead,
		},
		func(ctx context.Context, tx pgx.Tx) error {
			var err error

			replicaNode, err = loadNode(ctx, tx)
			if err != nil {
				return err
			}

			return tx.QueryRow(
				ctx,
				"SHOW transaction_read_only",
			).Scan(&replicaReadOnly)
		},
	)
	if err != nil {
		return fmt.Errorf("run replica transaction: %w", err)
	}

	fmt.Println("transactions:")
	fmt.Printf(
		"- primary node=%s read_only=%s\n",
		primaryNode.Name,
		primaryReadOnly,
	)
	fmt.Printf(
		"- replica node=%s read_only=%s\n",
		replicaNode.Name,
		replicaReadOnly,
	)

	return nil
}

func loadNode(ctx context.Context, db queryRower) (nodeInfo, error) {
	var node nodeInfo

	err := db.QueryRow(
		ctx,
		`SELECT node_name, node_role
		 FROM xpg_cluster_example.node_info`,
	).Scan(
		&node.Name,
		&node.Role,
	)
	if err != nil {
		return nodeInfo{}, err
	}

	return node, nil
}

func environment(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
