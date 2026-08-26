package shard

import (
	"errors"
	"fmt"

	"github.com/mkbeh/xpg/cluster"
)

var (
	// ErrNoShard indicates that a resolver could not map a key to any shard.
	ErrNoShard = errors.New("xpg/shard: no shard resolved")

	// ErrUnknownShard indicates that routing configuration or custom routing
	// logic referenced a shard that does not exist in the topology.
	ErrUnknownShard = errors.New("xpg/shard: unknown shard")

	// ErrShardMismatch indicates that keys expected to be colocated resolved to
	// different shards.
	ErrShardMismatch = errors.New("xpg/shard: keys resolve to different shards")
)

// UnknownShardError identifies a shard that does not exist in a topology.
type UnknownShardError struct {
	ShardID cluster.ID
}

func (e *UnknownShardError) Error() string {
	return fmt.Sprintf("xpg/shard: unknown shard %q", e.ShardID)
}

func (e *UnknownShardError) Unwrap() error {
	return ErrUnknownShard
}

// MismatchError describes the first key that resolved to a different shard
// than the first key.
type MismatchError struct {
	Expected cluster.ID
	Actual   cluster.ID
	Index    int
}

func (e *MismatchError) Error() string {
	return fmt.Sprintf(
		"xpg/shard: key %d resolved to shard %q instead of %q",
		e.Index,
		e.Actual,
		e.Expected,
	)
}

func (e *MismatchError) Unwrap() error {
	return ErrShardMismatch
}
