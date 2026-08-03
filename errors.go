package xpg

import (
	"errors"
	"io"
	"net"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

const (
	sqlStateUniqueViolation      = "23505"
	sqlStateForeignKeyViolation  = "23503"
	sqlStateNotNullViolation     = "23502"
	sqlStateCheckViolation       = "23514"
	sqlStateSerializationFailure = "40001"
	sqlStateDeadlockDetected     = "40P01"
	sqlStateLockNotAvailable     = "55P03"
	sqlStateQueryCanceled        = "57014"
)

// SQLState returns the PostgreSQL SQLSTATE code carried by err.
// It returns an empty string when the error tree does not contain a
// pgconn.PgError.
func SQLState(err error) string {
	pgErr, ok := errors.AsType[*pgconn.PgError](err)
	if !ok || pgErr == nil {
		return ""
	}

	return pgErr.Code
}

// IsNoRows reports whether err indicates that a query returned no rows.
func IsNoRows(err error) bool {
	return errors.Is(err, pgx.ErrNoRows)
}

// IsUniqueViolation reports whether err is a PostgreSQL unique_violation.
func IsUniqueViolation(err error) bool {
	return SQLState(err) == sqlStateUniqueViolation
}

// IsForeignKeyViolation reports whether err is a PostgreSQL
// foreign_key_violation.
func IsForeignKeyViolation(err error) bool {
	return SQLState(err) == sqlStateForeignKeyViolation
}

// IsNotNullViolation reports whether err is a PostgreSQL not_null_violation.
func IsNotNullViolation(err error) bool {
	return SQLState(err) == sqlStateNotNullViolation
}

// IsCheckViolation reports whether err is a PostgreSQL check_violation.
func IsCheckViolation(err error) bool {
	return SQLState(err) == sqlStateCheckViolation
}

// IsSerializationFailure reports whether err is a PostgreSQL
// serialization_failure.
func IsSerializationFailure(err error) bool {
	return SQLState(err) == sqlStateSerializationFailure
}

// IsDeadlock reports whether err is a PostgreSQL deadlock_detected error.
func IsDeadlock(err error) bool {
	return SQLState(err) == sqlStateDeadlockDetected
}

// IsLockNotAvailable reports whether err is a PostgreSQL lock_not_available
// error.
func IsLockNotAvailable(err error) bool {
	return SQLState(err) == sqlStateLockNotAvailable
}

// IsQueryCanceled reports whether PostgreSQL canceled the query.
//
// Client-side context cancellation remains available through errors.Is with
// context.Canceled or context.DeadlineExceeded.
func IsQueryCanceled(err error) bool {
	return SQLState(err) == sqlStateQueryCanceled
}

// IsConnectionError reports whether err represents a PostgreSQL connection
// failure known to pgx or the Go networking stack.
func IsConnectionError(err error) bool {
	if err == nil {
		return false
	}

	if errors.Is(err, pgconn.ErrConnClosed) ||
		errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) {
		return true
	}

	if _, ok := errors.AsType[*pgconn.ConnectError](err); ok {
		return true
	}

	if _, ok := errors.AsType[*net.OpError](err); ok {
		return true
	}

	state := SQLState(err)

	return len(state) >= 2 && state[:2] == "08"
}

// IsRetryableTransaction reports whether PostgreSQL aborted the transaction
// because of a serialization failure or a deadlock.
//
// The entire transaction callback must still be safe to replay. Connection
// failures are deliberately not classified as transaction-retryable.
func IsRetryableTransaction(err error) bool {
	return IsSerializationFailure(err) || IsDeadlock(err)
}
