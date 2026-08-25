package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg/cluster"
)

type nodeInfo struct {
	Name string
	Role string
}

type rowQuerier interface {
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

func showRouting(ctx context.Context, dbCluster *cluster.Cluster) error {
	primary, err := dbCluster.ReadPool(ctx, cluster.ReadPrimary)
	if err != nil {
		return fmt.Errorf("resolve primary read: %w", err)
	}

	node, err := loadNode(ctx, primary)
	if err != nil {
		return fmt.Errorf("read primary node: %w", err)
	}

	fmt.Println("primary read:")
	fmt.Printf(
		"- pool=%s node=%s role=%s\n",
		primary.Name(),
		node.Name,
		node.Role,
	)

	fmt.Println("replica reads:")

	for range dbCluster.ReplicaCount() {
		replica, err := dbCluster.ReadPool(
			ctx,
			cluster.ReadReplicaRequired,
		)
		if err != nil {
			return fmt.Errorf("resolve replica read: %w", err)
		}

		node, err := loadNode(ctx, replica)
		if err != nil {
			return fmt.Errorf("read replica node: %w", err)
		}

		fmt.Printf(
			"- pool=%s node=%s role=%s\n",
			replica.Name(),
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

func loadNode(ctx context.Context, db rowQuerier) (nodeInfo, error) {
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
