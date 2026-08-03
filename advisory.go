package xpg

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

const (
	advisoryXactLockSQL    = "SELECT pg_advisory_xact_lock($1)"
	tryAdvisoryXactLockSQL = "SELECT pg_try_advisory_xact_lock($1)"
)

// AdvisoryXactLock acquires an exclusive transaction-level advisory lock.
//
// The call waits until the lock is available or ctx is canceled. PostgreSQL
// releases the lock automatically when tx is committed or rolled back.
func AdvisoryXactLock(ctx context.Context, tx pgx.Tx, key int64) error {
	if tx == nil {
		return errors.New("xpg: transaction is nil")
	}

	if _, err := tx.Exec(ctx, advisoryXactLockSQL, key); err != nil {
		return fmt.Errorf("xpg: acquire transaction advisory lock: %w", err)
	}

	return nil
}

// TryAdvisoryXactLock attempts to acquire an exclusive transaction-level
// advisory lock without waiting.
//
// PostgreSQL releases an acquired lock automatically when tx is committed or
// rolled back.
func TryAdvisoryXactLock(ctx context.Context, tx pgx.Tx, key int64) (bool, error) {
	if tx == nil {
		return false, errors.New("xpg: transaction is nil")
	}

	var acquired bool

	if err := tx.QueryRow(ctx, tryAdvisoryXactLockSQL, key).Scan(&acquired); err != nil {
		return false, fmt.Errorf("xpg: try transaction advisory lock: %w", err)
	}

	return acquired, nil
}
