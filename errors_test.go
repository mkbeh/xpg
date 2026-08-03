package xpg

import (
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestSQLState(t *testing.T) {
	t.Parallel()

	err := fmt.Errorf(
		"wrapped: %w",
		&pgconn.PgError{Code: sqlStateUniqueViolation},
	)

	if state := SQLState(err); state != sqlStateUniqueViolation {
		t.Fatalf(
			"unexpected SQLSTATE: got %q, want %q",
			state,
			sqlStateUniqueViolation,
		)
	}
}

func TestIsNoRows(t *testing.T) {
	t.Parallel()

	if !IsNoRows(fmt.Errorf("wrapped: %w", pgx.ErrNoRows)) {
		t.Fatal("IsNoRows returned false")
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
		})
	}
}

func TestIsConnectionError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
	}{
		{
			name: "SQLSTATE connection exception",
			err:  &pgconn.PgError{Code: "08006"},
		},
		{
			name: "closed connection",
			err:  fmt.Errorf("wrapped: %w", pgconn.ErrConnClosed),
		},
		{
			name: "network operation",
			err: &net.OpError{
				Op:  "read",
				Net: "tcp",
				Err: errors.New("connection reset"),
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			if !IsConnectionError(test.err) {
				t.Fatal("IsConnectionError returned false")
			}
		})
	}
}

func TestIsRetryableTransaction(t *testing.T) {
	t.Parallel()

	for _, state := range []string{
		sqlStateSerializationFailure,
		sqlStateDeadlockDetected,
	} {
		err := &pgconn.PgError{Code: state}
		if !IsRetryableTransaction(err) {
			t.Fatalf(
				"IsRetryableTransaction returned false for SQLSTATE %q",
				state,
			)
		}
	}

	if IsRetryableTransaction(
		&pgconn.PgError{Code: sqlStateUniqueViolation},
	) {
		t.Fatal("IsRetryableTransaction returned true for unique violation")
	}
}
