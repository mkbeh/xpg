package xpg

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPoolInTxNilFunc(t *testing.T) {
	t.Parallel()

	err := (&Pool{}).InTx(
		t.Context(),
		pgx.TxOptions{},
		nil,
	)

	assertErrorMessage(
		t,
		err,
		"xpg: transaction function is nil",
	)
}

func TestInSavepointNilTx(t *testing.T) {
	t.Parallel()

	called := false

	err := InSavepoint(
		t.Context(),
		nil,
		func(context.Context, pgx.Tx) error {
			called = true

			return nil
		},
	)

	assertErrorMessage(
		t,
		err,
		"xpg: transaction is nil",
	)

	if called {
		t.Fatal("savepoint function was called with a nil transaction")
	}
}

func TestInSavepointNilFunc(t *testing.T) {
	t.Parallel()

	err := InSavepoint(
		t.Context(),
		&savepointParentTx{},
		nil,
	)

	assertErrorMessage(
		t,
		err,
		"xpg: savepoint function is nil",
	)
}

func TestInSavepoint(t *testing.T) {
	t.Parallel()

	savepoint := &savepointTestTx{}
	parent := &savepointParentTx{
		savepoint: savepoint,
	}

	called := false

	err := InSavepoint(
		t.Context(),
		parent,
		func(_ context.Context, tx pgx.Tx) error {
			called = true

			if tx != savepoint {
				t.Fatal("unexpected savepoint transaction")
			}

			return nil
		},
	)
	if err != nil {
		t.Fatalf("InSavepoint returned an error: %v", err)
	}

	if !called {
		t.Fatal("savepoint function was not called")
	}

	if savepoint.commitCalls != 1 {
		t.Fatalf(
			"commit calls = %d, want 1",
			savepoint.commitCalls,
		)
	}
}

func TestInSavepointPreservesCallbackError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("callback failed")
	savepoint := &savepointTestTx{}
	parent := &savepointParentTx{
		savepoint: savepoint,
	}

	err := InSavepoint(
		t.Context(),
		parent,
		func(context.Context, pgx.Tx) error {
			return expectedErr
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("original error was not preserved: %v", err)
	}

	if savepoint.commitCalls != 0 {
		t.Fatalf(
			"commit calls = %d, want 0",
			savepoint.commitCalls,
		)
	}

	if savepoint.rollbackCalls == 0 {
		t.Fatal("savepoint was not rolled back")
	}
}

func TestInSavepointPreservesBeginError(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("begin failed")
	parent := &savepointParentTx{
		beginErr: expectedErr,
	}

	called := false

	err := InSavepoint(
		t.Context(),
		parent,
		func(context.Context, pgx.Tx) error {
			called = true

			return nil
		},
	)
	if !errors.Is(err, expectedErr) {
		t.Fatalf("original error was not preserved: %v", err)
	}

	if called {
		t.Fatal("savepoint function was called after begin failure")
	}
}

type savepointParentTx struct {
	pgx.Tx

	savepoint pgx.Tx
	beginErr  error
}

func (tx *savepointParentTx) Begin(context.Context) (pgx.Tx, error) {
	if tx.beginErr != nil {
		return nil, tx.beginErr
	}

	return tx.savepoint, nil
}

type savepointTestTx struct {
	pgx.Tx

	closed        bool
	commitCalls   int
	rollbackCalls int
}

func (tx *savepointTestTx) Commit(context.Context) error {
	tx.commitCalls++

	if tx.closed {
		return pgx.ErrTxClosed
	}

	tx.closed = true

	return nil
}

func (tx *savepointTestTx) Rollback(context.Context) error {
	tx.rollbackCalls++

	if tx.closed {
		return pgx.ErrTxClosed
	}

	tx.closed = true

	return nil
}

func assertErrorMessage(
	t *testing.T,
	err error,
	want string,
) {
	t.Helper()

	if err == nil {
		t.Fatalf("expected error %q, got nil", want)
	}

	if err.Error() != want {
		t.Fatalf(
			"error = %q, want %q",
			err,
			want,
		)
	}
}
