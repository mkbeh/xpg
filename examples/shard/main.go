package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mkbeh/xpg/shard"
	"github.com/mkbeh/xpg/shard/resolver"
)

const (
	shardARangeStart uint64 = 0
	shardBoundary    uint64 = 100
	shardBRangeEnd   uint64 = 200
)

type user struct {
	ID   uint64
	Name string
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
		[]resolver.Range[uint64]{
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

	users := []user{
		{ID: 42, Name: "alice"},
		{ID: 142, Name: "bob"},
	}

	fmt.Println("range routing:")

	for _, current := range users {
		resolved, err := userResolver.Resolve(current.ID)
		if err != nil {
			return fmt.Errorf("resolve user %d: %w", current.ID, err)
		}

		primary := resolved.Primary()
		if primary == nil {
			return fmt.Errorf("shard %q has no primary", resolved.ID())
		}

		if _, err := primary.Exec(
			ctx,
			`INSERT INTO xpg_shard_example.users (id, name)
			 VALUES ($1, $2)
			 ON CONFLICT (id) DO UPDATE
			 SET name = EXCLUDED.name`,
			current.ID,
			current.Name,
		); err != nil {
			return fmt.Errorf("write user %d: %w", current.ID, err)
		}

		fmt.Printf(
			"- user_id=%d shard=%s pool=%s\n",
			current.ID,
			resolved.ID(),
			primary.Name(),
		)
	}

	groups, err := shard.GroupByShard(
		userResolver,
		[]uint64{142, 42, 143, 43},
	)
	if err != nil {
		return fmt.Errorf("group user IDs: %w", err)
	}

	fmt.Println()
	fmt.Println("grouping:")

	for _, group := range groups {
		fmt.Printf(
			"- shard=%s user_ids=%v\n",
			group.Shard.ID(),
			group.Keys,
		)
	}

	return nil
}
