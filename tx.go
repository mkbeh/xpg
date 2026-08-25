package xpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// InTx executes fn in a transaction configured by txOptions.
//
// If fn returns nil, the transaction is committed; otherwise it is rolled back.
// If fn panics, rollback is attempted before the panic is propagated. The
// callback must not call Commit or Rollback; InTx owns transaction
// finalization.
//
// The callback receives ctx unchanged. Context cancellation does not
// automatically finalize the transaction while fn is running; fn should
// observe ctx and return promptly.
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

// InSavepoint executes fn within a PostgreSQL savepoint.
//
// If fn returns nil, the savepoint is released; otherwise it is rolled back.
// If fn panics, rollback is attempted before the panic is propagated. The
// callback must not call Commit or Rollback; InSavepoint owns savepoint
// finalization.
//
// The callback receives ctx unchanged and should observe its cancellation.
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
