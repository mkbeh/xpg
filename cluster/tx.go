package cluster

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
)

// ReadTxOptions configures a read-only transaction.
//
// AccessMode, BeginQuery, and CommitQuery are intentionally controlled by the
// cluster.
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
		return errors.New("xpg/cluster: cluster is nil")
	}

	if c.primary == nil {
		return ErrNoPrimary
	}

	return c.primary.InTx(ctx, options, fn)
}

// InReadTx selects a pool according to policy and executes fn in a read-only
// transaction on that pool.
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
