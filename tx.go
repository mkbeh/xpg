package xpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// InTx executes fn in a transaction configured by txOptions.
//
// The transaction is committed when fn returns nil and rolled back when fn
// returns an error. If fn panics, rollback is attempted before the panic is
// propagated. The callback must not call Commit or Rollback; InTx owns
// transaction finalization.
//
// The callback receives the same context and an explicit pgx.Tx. Context
// cancellation does not automatically finalize the transaction while fn is
// running; fn should observe ctx and return promptly.
func (p *Pool) InTx(
	ctx context.Context,
	txOptions pgx.TxOptions,
	fn func(context.Context, pgx.Tx) error,
) error {
	if fn == nil {
		return errors.New("xpg: transaction function is nil")
	}

	err := pgx.BeginTxFunc(
		ctx,
		p.pool,
		txOptions,
		func(tx pgx.Tx) error {
			return fn(ctx, tx)
		},
	)
	if err != nil {
		return fmt.Errorf("xpg: transaction: %w", err)
	}

	return nil
}

// InSavepoint executes fn in a pseudo-nested transaction implemented with a
// PostgreSQL savepoint.
//
// The savepoint is released when fn returns nil and rolled back when fn returns
// an error. If fn panics, rollback is attempted before the panic is propagated.
// The callback must not call Commit or Rollback; InSavepoint owns savepoint
// finalization.
func InSavepoint(
	ctx context.Context,
	tx pgx.Tx,
	fn func(context.Context, pgx.Tx) error,
) error {
	if tx == nil {
		return errors.New("xpg: transaction is nil")
	}

	if fn == nil {
		return errors.New("xpg: savepoint function is nil")
	}

	err := pgx.BeginFunc(
		ctx,
		tx,
		func(savepoint pgx.Tx) error {
			return fn(ctx, savepoint)
		},
	)
	if err != nil {
		return fmt.Errorf("xpg: savepoint: %w", err)
	}

	return nil
}
