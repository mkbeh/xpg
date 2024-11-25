package postgres

import (
	"context"
	"errors"
	"net"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type PgErrorCode int

const (
	ErrContextDeadline PgErrorCode = iota
	ErrNoRows
	ErrUniqViolation
	ErrOther
	ErrBeginTransaction
	ErrCommitTransaction
	ErrNoConnection
)

type PgError struct {
	code PgErrorCode
	msg  string
}

func (e PgError) Error() string {
	return e.msg
}

func (e PgError) Code() PgErrorCode {
	return e.code
}

func NewPgError(code PgErrorCode, err error) *PgError {
	return &PgError{code, err.Error()}
}

func ConvertError(err error) *PgError {
	if err == nil {
		return nil
	}

	if pgErr, ok := err.(*PgError); ok {
		return pgErr
	}

	if ne, ok := err.(net.Error); ok {
		return NewPgError(ErrNoConnection, ne)
	}

	switch {
	case errors.Is(err, context.Canceled), pgconn.Timeout(err):
		return NewPgError(ErrContextDeadline, err)
	case errors.Is(err, pgx.ErrNoRows):
		return NewPgError(ErrNoRows, err)
	default:
		var pgErr *pgconn.PgError
		var code string

		if errors.As(err, &pgErr) {
			code = pgErr.Code
		}

		switch code {
		case "":
			return NewPgError(ErrOther, err)
		case pgerrcode.UniqueViolation:
			return NewPgError(ErrUniqViolation, err)
		default:
			return NewPgError(ErrOther, err)
		}
	}
}
