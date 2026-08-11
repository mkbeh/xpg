package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg/cluster"
	"github.com/mkbeh/xpg/shard"
)

const referenceTableConcurrency = 2

type nodeInfo struct {
	name string
	role string
}

type rowQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func showReplicaRead(
	ctx context.Context,
	userResolver shard.Resolver[uint64],
	key uint64,
) error {
	resolved, err := userResolver.Resolve(key)
	if err != nil {
		return fmt.Errorf("resolve replica key %d: %w", key, err)
	}

	pool, err := resolved.ReadPool(
		ctx,
		cluster.ReadReplicaRequired,
	)
	if err != nil {
		return fmt.Errorf("resolve shard replica: %w", err)
	}

	node, err := loadNode(ctx, pool)
	if err != nil {
		return fmt.Errorf("read replica node: %w", err)
	}

	var (
		transactionNode nodeInfo
		readOnly        string
	)
	if err := resolved.InReadTx(
		ctx,
		cluster.ReadReplicaRequired,
		cluster.ReadTxOptions{},
		func(ctx context.Context, tx pgx.Tx) error {
			var err error

			transactionNode, err = loadNode(ctx, tx)
			if err != nil {
				return err
			}

			return tx.QueryRow(
				ctx,
				"SHOW transaction_read_only",
			).Scan(&readOnly)
		},
	); err != nil {
		return fmt.Errorf("run shard read transaction: %w", err)
	}

	fmt.Println()
	fmt.Println("replica routing and read-only transaction:")
	fmt.Printf(
		"- user_id=%d shard=%s read_pool=%s read_node=%s tx_node=%s role=%s read_only=%s\n",
		key,
		resolved.ID(),
		pool.Name(),
		node.name,
		transactionNode.name,
		node.role,
		readOnly,
	)

	return nil
}

func showReferenceTables(
	ctx context.Context,
	topology *shard.Topology,
) error {
	counts := make(map[shard.ID]int, topology.Len())
	var countsMu sync.Mutex

	// Fan out with bounded concurrency while preserving registration-order
	// results for deterministic reporting.
	results, err := topology.ForEachShard(
		ctx,
		referenceTableConcurrency,
		func(ctx context.Context, resolved shard.Shard) error {
			primary := resolved.Primary()
			if primary == nil {
				return fmt.Errorf("shard %q has no primary", resolved.ID())
			}

			var count int
			if err := primary.QueryRow(
				ctx,
				`SELECT count(*)
				 FROM xpg_shard_example.countries`,
			).Scan(&count); err != nil {
				return err
			}

			countsMu.Lock()
			counts[resolved.ID()] = count
			countsMu.Unlock()

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("schedule reference-table reads: %w", err)
	}

	if err := results.Err(); err != nil {
		return fmt.Errorf("read reference tables: %w", err)
	}

	fmt.Println()
	fmt.Println("reference table copies:")
	for index := 0; index < topology.Len(); index++ {
		resolved := topology.At(index)
		fmt.Printf(
			"- shard=%s countries=%d\n",
			resolved.ID(),
			counts[resolved.ID()],
		)
	}

	return nil
}

func loadNode(
	ctx context.Context,
	db rowQuerier,
) (nodeInfo, error) {
	var node nodeInfo

	err := db.QueryRow(
		ctx,
		`SELECT node_name, node_role
		 FROM xpg_shard_example.node_info`,
	).Scan(&node.name, &node.role)
	if err != nil {
		return nodeInfo{}, err
	}

	return node, nil
}
