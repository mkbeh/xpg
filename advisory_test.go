package xpg

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestAdvisoryXactLockNilTx(t *testing.T) {
	t.Parallel()

	err := AdvisoryXactLock(
		t.Context(),
		nil,
		1,
	)

	assertErrorMessage(t, err, "xpg: transaction is nil")
}

func TestAdvisoryXactLock(t *testing.T) {
	t.Parallel()

	tx := &advisoryTestTx{}

	err := AdvisoryXactLock(
		t.Context(),
		tx,
		42,
	)
	if err != nil {
		t.Fatalf("AdvisoryXactLock returned an error: %v", err)
	}

	if tx.execSQL != advisoryXactLockSQL {
		t.Fatalf(
			"SQL = %q, want %q",
			tx.execSQL,
			advisoryXactLockSQL,
		)
	}

	assertAdvisoryKey(t, tx.execArgs, 42)
}

func TestAdvisoryXactLockPreservesError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("lock failed")

	err := AdvisoryXactLock(
		t.Context(),
		&advisoryTestTx{
			execErr: expectedErr,
		},
		1,
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("original error was not preserved: %v", err)
	}
}

func TestTryAdvisoryXactLockNilTx(t *testing.T) {
	t.Parallel()

	_, err := TryAdvisoryXactLock(
		t.Context(),
		nil,
		1,
	)

	assertErrorMessage(t, err, "xpg: transaction is nil")
}

func TestTryAdvisoryXactLock(t *testing.T) {
	t.Parallel()

	for _, acquired := range []bool{false, true} {
		t.Run(
			map[bool]string{
				false: "not acquired",
				true:  "acquired",
			}[acquired],
			func(t *testing.T) {
				t.Parallel()

				tx := &advisoryTestTx{
					row: advisoryTestRow{
						acquired: acquired,
					},
				}

				got, err := TryAdvisoryXactLock(
					t.Context(),
					tx,
					42,
				)
				if err != nil {
					t.Fatalf(
						"TryAdvisoryXactLock returned an error: %v",
						err,
					)
				}

				if got != acquired {
					t.Fatalf(
						"acquired = %v, want %v",
						got,
						acquired,
					)
				}

				if tx.querySQL != tryAdvisoryXactLockSQL {
					t.Fatalf(
						"SQL = %q, want %q",
						tx.querySQL,
						tryAdvisoryXactLockSQL,
					)
				}

				assertAdvisoryKey(t, tx.queryArgs, 42)
			},
		)
	}
}

func TestTryAdvisoryXactLockPreservesError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("try lock failed")

	_, err := TryAdvisoryXactLock(
		t.Context(),
		&advisoryTestTx{
			row: advisoryTestRow{
				err: expectedErr,
			},
		},
		1,
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("original error was not preserved: %v", err)
	}
}

type advisoryTestTx struct {
	pgx.Tx

	execSQL  string
	execArgs []any
	execErr  error

	querySQL  string
	queryArgs []any
	row       pgx.Row
}

func (tx *advisoryTestTx) Exec(
	_ context.Context,
	sql string,
	args ...any,
) (pgconn.CommandTag, error) {
	tx.execSQL = sql
	tx.execArgs = args

	return pgconn.CommandTag{}, tx.execErr
}

func (tx *advisoryTestTx) QueryRow(
	_ context.Context,
	sql string,
	args ...any,
) pgx.Row {
	tx.querySQL = sql
	tx.queryArgs = args

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

func assertAdvisoryKey(
	t *testing.T,
	args []any,
	want int64,
) {
	t.Helper()

	if len(args) != 1 {
		t.Fatalf(
			"argument count = %d, want 1",
			len(args),
		)
	}

	key, ok := args[0].(int64)
	if !ok {
		t.Fatalf(
			"argument type = %T, want int64",
			args[0],
		)
	}

	if key != want {
		t.Fatalf(
			"key = %d, want %d",
			key,
			want,
		)
	}
}
