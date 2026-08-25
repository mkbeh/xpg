package xpg

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSQLState(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want string
	}{
		{
			name: "PostgreSQL error",
			err: fmt.Errorf(
				"wrapped: %w",
				&pgconn.PgError{Code: sqlStateUniqueViolation},
			),
			want: sqlStateUniqueViolation,
		},
		{
			name: "generic error",
			err:  errors.New("generic"),
			want: "",
		},
		{
			name: "nil",
			err:  nil,
			want: "",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := SQLState(test.err); got != test.want {
				t.Fatalf(
					"SQLState() = %q, want %q",
					got,
					test.want,
				)
			}
		})
	}
}

func TestIsNoRows(t *testing.T) {
	t.Parallel()

	if !IsNoRows(fmt.Errorf("wrapped: %w", pgx.ErrNoRows)) {
		t.Fatal("IsNoRows returned false for pgx.ErrNoRows")
	}

	if IsNoRows(errors.New("generic")) {
		t.Fatal("IsNoRows returned true for generic error")
	}
}

func TestErrorClassifiers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		state      string
		classifier func(error) bool
	}{
		{
			name:       "unique violation",
			state:      sqlStateUniqueViolation,
			classifier: IsUniqueViolation,
		},
		{
			name:       "foreign key violation",
			state:      sqlStateForeignKeyViolation,
			classifier: IsForeignKeyViolation,
		},
		{
			name:       "not null violation",
			state:      sqlStateNotNullViolation,
			classifier: IsNotNullViolation,
		},
		{
			name:       "check violation",
			state:      sqlStateCheckViolation,
			classifier: IsCheckViolation,
		},
		{
			name:       "serialization failure",
			state:      sqlStateSerializationFailure,
			classifier: IsSerializationFailure,
		},
		{
			name:       "deadlock",
			state:      sqlStateDeadlockDetected,
			classifier: IsDeadlock,
		},
		{
			name:       "lock not available",
			state:      sqlStateLockNotAvailable,
			classifier: IsLockNotAvailable,
		},
		{
			name:       "query canceled",
			state:      sqlStateQueryCanceled,
			classifier: IsQueryCanceled,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := fmt.Errorf(
				"wrapped: %w",
				&pgconn.PgError{Code: test.state},
			)

			if !test.classifier(err) {
				t.Fatalf(
					"classifier returned false for SQLSTATE %q",
					test.state,
				)
			}

			if test.classifier(
				&pgconn.PgError{Code: "00000"},
			) {
				t.Fatalf(
					"classifier returned true for unrelated SQLSTATE",
				)
			}
		})
	}
}

func TestIsQueryCanceledDoesNotClassifyContextCancellation(t *testing.T) {
	t.Parallel()

	if IsQueryCanceled(context.Canceled) {
		t.Fatal("IsQueryCanceled returned true for context.Canceled")
	}

	if IsQueryCanceled(context.DeadlineExceeded) {
		t.Fatal("IsQueryCanceled returned true for context.DeadlineExceeded")
	}
}

func TestIsConnectionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "SQLSTATE connection exception",
			err:  &pgconn.PgError{Code: "08006"},
			want: true,
		},
		{
			name: "closed connection",
			err:  fmt.Errorf("wrapped: %w", pgconn.ErrConnClosed),
			want: true,
		},
		{
			name: "EOF",
			err:  io.EOF,
			want: true,
		},
		{
			name: "unexpected EOF",
			err:  io.ErrUnexpectedEOF,
			want: true,
		},
		{
			name: "network operation",
			err: &net.OpError{
				Op:  "read",
				Net: "tcp",
				Err: errors.New("connection reset"),
			},
			want: true,
		},
		{
			name: "generic error",
			err:  errors.New("generic"),
			want: false,
		},
		{
			name: "nil",
			err:  nil,
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if got := IsConnectionError(test.err); got != test.want {
				t.Fatalf(
					"IsConnectionError() = %v, want %v",
					got,
					test.want,
				)
			}
		})
	}
}

func TestIsRetryableTransaction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		state string
		want  bool
	}{
		{
			name:  "serialization failure",
			state: sqlStateSerializationFailure,
			want:  true,
		},
		{
			name:  "deadlock",
			state: sqlStateDeadlockDetected,
			want:  true,
		},
		{
			name:  "unique violation",
			state: sqlStateUniqueViolation,
			want:  false,
		},
		{
			name:  "connection exception",
			state: "08006",
			want:  false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := &pgconn.PgError{Code: test.state}

			if got := IsRetryableTransaction(err); got != test.want {
				t.Fatalf(
					"IsRetryableTransaction() = %v, want %v",
					got,
					test.want,
				)
			}
		})
	}
}
