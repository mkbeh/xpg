package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mkbeh/xpg/shard/resolver"
)

const (
	userIDRangeStart uint64 = 0
	userIDBoundary   uint64 = 100
	userIDRangeEnd   uint64 = 200
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	topology, err := openTopology(ctx)
	if err != nil {
		return err
	}
	defer topology.Close()

	userResolver, err := resolver.NewRange(
		topology,
		[]resolver.Range[uint64]{
			{
				Start:   userIDRangeStart,
				End:     userIDBoundary,
				ShardID: shardAID,
			},
			{
				Start:   userIDBoundary,
				End:     userIDRangeEnd,
				ShardID: shardBID,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("create user resolver: %w", err)
	}

	if err := showRangeRouting(ctx, userResolver); err != nil {
		return err
	}

	if err := showGrouping(userResolver); err != nil {
		return err
	}

	if err := showReplicaRead(ctx, userResolver, shardBUserID); err != nil {
		return err
	}

	if err := showReferenceTables(ctx, topology); err != nil {
		return err
	}

	return nil
}
