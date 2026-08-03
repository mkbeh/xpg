package xpg

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type advisoryTestTx struct {
	pgx.Tx

	execErr error
	row     pgx.Row
}

func (tx *advisoryTestTx) Exec(context.Context, string, ...any) (pgconn.CommandTag, error) {
	return pgconn.CommandTag{}, tx.execErr
}

func (tx *advisoryTestTx) QueryRow(context.Context, string, ...any) pgx.Row {
	return tx.row
}

type advisoryTestRow struct {
	acquired bool
	err      error
}

func (row advisoryTestRow) Scan(dest ...any) error {
	if row.err != nil {
		return row.err
	}

	acquired, ok := dest[0].(*bool)
	if !ok {
		return errors.New("unexpected destination type")
	}

	*acquired = row.acquired

	return nil
}

func TestAdvisoryXactLockNilTx(t *testing.T) {
	t.Parallel()

	err := AdvisoryXactLock(
		context.Background(),
		nil,
		1,
	)

	assertErrorMessage(t, err, "xpg: transaction is nil")
}

func TestAdvisoryXactLockPreservesError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("lock failed")

	err := AdvisoryXactLock(
		context.Background(),
		&advisoryTestTx{execErr: expectedErr},
		1,
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("original error was not preserved: %v", err)
	}
}

func TestTryAdvisoryXactLockNilTx(t *testing.T) {
	t.Parallel()

	_, err := TryAdvisoryXactLock(
		context.Background(),
		nil,
		1,
	)

	assertErrorMessage(t, err, "xpg: transaction is nil")
}

func TestTryAdvisoryXactLock(t *testing.T) {
	t.Parallel()

	acquired, err := TryAdvisoryXactLock(
		context.Background(),
		&advisoryTestTx{
			row: advisoryTestRow{acquired: true},
		},
		1,
	)
	if err != nil {
		t.Fatalf("TryAdvisoryXactLock returned an error: %v", err)
	}

	if !acquired {
		t.Fatal("TryAdvisoryXactLock returned false")
	}
}

func TestTryAdvisoryXactLockPreservesError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("try lock failed")

	_, err := TryAdvisoryXactLock(
		context.Background(),
		&advisoryTestTx{
			row: advisoryTestRow{err: expectedErr},
		},
		1,
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("original error was not preserved: %v", err)
	}
}
