package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg/topology/shard"
	"github.com/mkbeh/xpg/topology/shard/resolver"
)

const (
	shardARangeStart int64 = 0
	shardBoundary    int64 = 100
	shardBRangeEnd   int64 = 200
)

type user struct {
	ID     int64
	Name   string
	Active bool
}

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
		[]resolver.Range[int64]{
			{
				Start:   shardARangeStart,
				End:     shardBoundary,
				ShardID: shardAID,
			},
			{
				Start:   shardBoundary,
				End:     shardBRangeEnd,
				ShardID: shardBID,
			},
		},
	)
	if err != nil {
		return fmt.Errorf("create user resolver: %w", err)
	}

	if err := batchActivateUsers(ctx, userResolver, []int64{42, 142, 43, 143}); err != nil {
		return err
	}

	if err := updateColocatedUsers(ctx, userResolver, 42, 43); err != nil {
		return err
	}

	if err := showShardMismatch(userResolver, 42, 142); err != nil {
		return err
	}

	if err := batchLoadUsers(ctx, userResolver, []int64{42, 142, 250, 43, 143}); err != nil {
		return err
	}

	return nil
}

func batchActivateUsers(
	ctx context.Context,
	userResolver shard.Resolver[int64],
	ids []int64,
) error {
	groups, err := shard.GroupByShard(userResolver, ids)
	if err != nil {
		return fmt.Errorf("group users for batch update: %w", err)
	}

	fmt.Println("batch update:")

	for _, group := range groups {
		primary := group.Shard.Primary()
		if primary == nil {
			return fmt.Errorf("shard %q has no primary", group.Shard.ID())
		}

		tag, err := primary.Exec(
			ctx,
			`UPDATE xpg_shard_group_example.users
			 SET active = true
			 WHERE id = ANY($1::bigint[])`,
			group.Keys,
		)
		if err != nil {
			return fmt.Errorf("batch update on shard %q: %w", group.Shard.ID(), err)
		}

		if tag.RowsAffected() != int64(len(group.Keys)) {
			return fmt.Errorf(
				"batch update on shard %q affected %d rows, want %d",
				group.Shard.ID(),
				tag.RowsAffected(),
				len(group.Keys),
			)
		}

		fmt.Printf("- shard=%s user_ids=%v\n", group.Shard.ID(), group.Keys)
	}

	return nil
}

func updateColocatedUsers(
	ctx context.Context,
	userResolver shard.Resolver[int64],
	ids ...int64,
) error {
	targetShard, err := shard.SameShard(userResolver, ids...)
	if err != nil {
		return fmt.Errorf("resolve colocated users: %w", err)
	}

	err = targetShard.InPrimaryTx(
		ctx,
		pgx.TxOptions{},
		func(ctx context.Context, tx pgx.Tx) error {
			tag, err := tx.Exec(
				ctx,
				`UPDATE xpg_shard_group_example.users
				 SET active = false
				 WHERE id = ANY($1::bigint[])`,
				ids,
			)
			if err != nil {
				return err
			}

			if tag.RowsAffected() != int64(len(ids)) {
				return fmt.Errorf("updated %d users, want %d", tag.RowsAffected(), len(ids))
			}

			return nil
		},
	)
	if err != nil {
		return fmt.Errorf("update colocated users on shard %q: %w", targetShard.ID(), err)
	}

	fmt.Println()
	fmt.Println("colocation:")
	fmt.Printf("- user_ids=%v shard=%s transaction=committed\n", ids, targetShard.ID())

	return nil
}

func showShardMismatch(
	userResolver shard.Resolver[int64],
	ids ...int64,
) error {
	_, err := shard.SameShard(userResolver, ids...)
	if !errors.Is(err, shard.ErrShardMismatch) {
		return fmt.Errorf("check shard mismatch: got %v, want shard.ErrShardMismatch", err)
	}

	var mismatch *shard.MismatchError
	if !errors.As(err, &mismatch) {
		return fmt.Errorf("check shard mismatch details: %w", err)
	}

	fmt.Printf(
		"- user_ids=%v mismatch=%s->%s\n",
		ids,
		mismatch.Expected,
		mismatch.Actual,
	)

	return nil
}

func batchLoadUsers(
	ctx context.Context,
	userResolver shard.Resolver[int64],
	ids []int64,
) error {
	partition, err := shard.PartitionByShard(userResolver, ids)
	if err != nil {
		return fmt.Errorf("partition users for batch select: %w", err)
	}

	fmt.Println()
	fmt.Println("batch select:")

	if len(partition.Unresolved) != 0 {
		fmt.Printf("- unresolved user_ids=%v\n", partition.Unresolved)
	}

	for _, group := range partition.Groups {
		primary := group.Shard.Primary()
		if primary == nil {
			return fmt.Errorf("shard %q has no primary", group.Shard.ID())
		}

		rows, err := primary.Query(
			ctx,
			`SELECT id, name, active
			 FROM xpg_shard_group_example.users
			 WHERE id = ANY($1::bigint[])
			 ORDER BY id`,
			group.Keys,
		)
		if err != nil {
			return fmt.Errorf("batch select on shard %q: %w", group.Shard.ID(), err)
		}

		users, err := pgx.CollectRows(rows, pgx.RowToStructByPos[user])
		if err != nil {
			return fmt.Errorf("collect users on shard %q: %w", group.Shard.ID(), err)
		}

		fmt.Printf("- shard=%s user_ids=%v\n", group.Shard.ID(), group.Keys)

		for _, current := range users {
			fmt.Printf(
				"  user_id=%d name=%s active=%t\n",
				current.ID,
				current.Name,
				current.Active,
			)
		}
	}

	return nil
}
