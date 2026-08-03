package xpg

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
)

type testTx struct {
	pgx.Tx
}

func TestPoolInTxNilFunc(t *testing.T) {
	t.Parallel()

	err := (&Pool{}).InTx(
		context.Background(),
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
		context.Background(),
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
		context.Background(),
		&testTx{},
		nil,
	)

	assertErrorMessage(
		t,
		err,
		"xpg: savepoint function is nil",
	)
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
		t.Fatalf("unexpected error: got %q, want %q", err, want)
	}
}
