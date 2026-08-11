package main

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg/shard"
)

const (
	shardAUserID       uint64 = 42
	shardASecondUserID uint64 = 43
	shardBUserID       uint64 = 142
	shardBSecondUserID uint64 = 143
)

func showRangeRouting(
	ctx context.Context,
	userResolver shard.Resolver[uint64],
) error {
	fmt.Println("range routing and primary transactions:")

	for _, userID := range []uint64{
		shardAUserID,
		shardBUserID,
	} {
		resolved, err := userResolver.Resolve(userID)
		if err != nil {
			return fmt.Errorf("resolve user %d: %w", userID, err)
		}

		primary := resolved.Primary()
		if primary == nil {
			return fmt.Errorf("shard %q has no primary", resolved.ID())
		}

		name := fmt.Sprintf("user-%d", userID)

		if err := resolved.InPrimaryTx(
			ctx,
			pgx.TxOptions{},
			func(ctx context.Context, tx pgx.Tx) error {
				_, err := tx.Exec(
					ctx,
					`INSERT INTO xpg_shard_example.users (id, name)
					 VALUES ($1, $2)
					 ON CONFLICT (id) DO UPDATE
					 SET name = EXCLUDED.name`,
					userID,
					name,
				)

				return err
			},
		); err != nil {
			return fmt.Errorf("write user %d: %w", userID, err)
		}

		fmt.Printf(
			"- user_id=%d shard=%s primary_pool=%s\n",
			userID,
			resolved.ID(),
			primary.Name(),
		)
	}

	return nil
}

func showGrouping(
	userResolver shard.Resolver[uint64],
) error {
	// The first occurrence belongs to shard-b, so GroupByShard returns
	// shard-b before shard-a.
	keys := []uint64{
		shardBUserID,
		shardAUserID,
		shardBSecondUserID,
		shardASecondUserID,
	}

	groups, err := shard.GroupByShard(
		userResolver,
		keys,
	)
	if err != nil {
		return fmt.Errorf("group users: %w", err)
	}

	colocated, err := shard.SameShard(
		userResolver,
		shardAUserID,
		shardASecondUserID,
	)
	if err != nil {
		return fmt.Errorf("verify colocated users: %w", err)
	}

	_, mismatchErr := shard.SameShard(
		userResolver,
		shardAUserID,
		shardBUserID,
	)
	if !errors.Is(mismatchErr, shard.ErrShardMismatch) {
		return fmt.Errorf(
			"verify cross-shard users: expected ErrShardMismatch, got %v",
			mismatchErr,
		)
	}

	fmt.Println()
	fmt.Println("grouping and colocation:")

	for _, group := range groups {
		fmt.Printf(
			"- shard=%s user_ids=%v\n",
			group.Shard.ID(),
			group.Keys,
		)
	}

	fmt.Printf(
		"- colocated shard=%s\n",
		colocated.ID(),
	)
	fmt.Println(
		"- cross-shard SameShard returns ErrShardMismatch=true",
	)

	return nil
}
