package shard

import (
	"errors"
	"fmt"
)

var (
	// ErrNoShard is returned when a resolver cannot map a key to any shard.
	ErrNoShard = errors.New("xpg/shard: no shard resolved")

	// ErrUnknownShard is returned when routing references a shard that does not
	// exist in the topology.
	ErrUnknownShard = errors.New("xpg/shard: unknown shard")

	// ErrShardMismatch is returned when keys expected to be colocated resolve to
	// different shards.
	ErrShardMismatch = errors.New("xpg/shard: keys resolve to different shards")
)

// UnknownShardError identifies a shard referenced by routing that does not
// exist in the topology.
type UnknownShardError struct {
	ShardID ID
}

func (e *UnknownShardError) Error() string {
	return fmt.Sprintf("xpg/shard: unknown shard %q", e.ShardID)
}

func (e *UnknownShardError) Unwrap() error {
	return ErrUnknownShard
}

// MismatchError describes the first key whose resolved shard differs from the
// shard of the first key.
type MismatchError struct {
	Expected ID
	Actual   ID
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
