package shard

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/topology/cluster"
)

// ID identifies one logical shard.
//
// ID is an alias of cluster.ID because a shard inherits the stable identity of
// its underlying cluster.
type ID = cluster.ID

// Shard is an immutable, restricted view of one Cluster registered in a
// Topology.
//
// Shard exposes shard-local data access without exposing cluster lifecycle or
// replica-set management. A Shard does not own the underlying Cluster and is
// valid only for the lifetime of its owning Topology; it must not be used after
// Topology.Close.
type Shard struct {
	cluster *cluster.Cluster
}

// ID returns the stable logical shard ID inherited from the underlying Cluster.
func (s Shard) ID() ID {
	if s.cluster == nil {
		return ""
	}

	return s.cluster.ID()
}

// Label returns one shard label without allocating a copy of all labels.
func (s Shard) Label(key string) (string, bool) {
	if s.cluster == nil {
		return "", false
	}

	return s.cluster.Label(key)
}

// Labels returns a defensive copy of shard labels.
func (s Shard) Labels() map[string]string {
	if s.cluster == nil {
		return nil
	}

	return s.cluster.Labels()
}

// Primary returns the shard primary pool, or nil when the underlying cluster
// has no primary configured. The returned pool is borrowed and must not be
// closed separately.
func (s Shard) Primary() *xpg.Pool {
	if s.cluster == nil {
		return nil
	}

	return s.cluster.Primary()
}

// ReadPool returns a borrowed pool for a read operation according to policy.
func (s Shard) ReadPool(
	ctx context.Context,
	policy cluster.ReadPolicy,
) (*xpg.Pool, error) {
	if s.cluster == nil {
		return nil, ErrNoShard
	}

	return s.cluster.ReadPool(ctx, policy)
}

// InPrimaryTx executes fn in a transaction on the shard primary.
func (s Shard) InPrimaryTx(
	ctx context.Context,
	options pgx.TxOptions,
	fn func(context.Context, pgx.Tx) error,
) error {
	if s.cluster == nil {
		return ErrNoShard
	}

	return s.cluster.InPrimaryTx(ctx, options, fn)
}

// InReadTx executes fn in a read-only transaction resolved within this shard.
func (s Shard) InReadTx(
	ctx context.Context,
	policy cluster.ReadPolicy,
	options cluster.ReadTxOptions,
	fn func(context.Context, pgx.Tx) error,
) error {
	if s.cluster == nil {
		return ErrNoShard
	}

	return s.cluster.InReadTx(ctx, policy, options, fn)
}
