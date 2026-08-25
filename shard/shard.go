package shard

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg"
	"github.com/mkbeh/xpg/cluster"
)

// ID identifies one logical shard.
type ID = cluster.ID

// Shard is a borrowed handle to one cluster registered in a Topology.
//
// Shard exposes shard-local operations without exposing cluster lifecycle or
// replica-set management.
type Shard struct {
	cluster *cluster.Cluster
}

// ID returns the logical shard ID.
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

// Primary returns the shard primary pool.
//
// Primary returns nil when no primary is configured. The returned pool is
// borrowed and must not be closed separately.
func (s Shard) Primary() *xpg.Pool {
	if s.cluster == nil {
		return nil
	}

	return s.cluster.Primary()
}

// ReadPool returns a borrowed pool according to policy.
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

// InReadTx executes fn in a read-only transaction on a pool selected according
// to policy within the shard.
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
