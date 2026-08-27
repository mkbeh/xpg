package cluster

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ReadTxOptions configures a read-only transaction.
//
// AccessMode is always pgx.ReadOnly. BeginQuery and CommitQuery are not exposed
// so callers cannot override the read-only transaction semantics.
type ReadTxOptions struct {
	IsoLevel       pgx.TxIsoLevel
	DeferrableMode pgx.TxDeferrableMode
}

// InPrimaryTx executes fn in a transaction on the primary pool.
func (c *Cluster) InPrimaryTx(
	ctx context.Context,
	options pgx.TxOptions,
	fn func(context.Context, pgx.Tx) error,
) error {
	if c == nil {
		return errors.New("xpg/topology/cluster: cluster is nil")
	}

	pool, err := c.resolvePrimary()
	if err != nil {
		return err
	}

	return pool.InTx(ctx, options, fn)
}

// InReadTx selects a pool according to policy and executes fn in a read-only
// transaction.
//
// The transaction remains read-only when policy resolves to the primary.
func (c *Cluster) InReadTx(
	ctx context.Context,
	policy ReadPolicy,
	options ReadTxOptions,
	fn func(context.Context, pgx.Tx) error,
) error {
	pool, err := c.ReadPool(ctx, policy)
	if err != nil {
		return err
	}

	return pool.InTx(
		ctx,
		pgx.TxOptions{
			IsoLevel:       options.IsoLevel,
			AccessMode:     pgx.ReadOnly,
			DeferrableMode: options.DeferrableMode,
		},
		fn,
	)
}
