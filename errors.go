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
	ErrForeignKeyViolation
	ErrSerializable
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
		if !errors.As(err, &pgErr) {
			return NewPgError(ErrOther, err)
		}
		return NewPgError(pgCodeToError(pgErr.Code), err)
	}
}

var pgCodeMap = map[string]PgErrorCode{
	pgerrcode.UniqueViolation:      ErrUniqViolation,
	pgerrcode.ForeignKeyViolation:  ErrForeignKeyViolation,
	pgerrcode.SerializationFailure: ErrSerializable,
}

func pgCodeToError(code string) PgErrorCode {
	if c, ok := pgCodeMap[code]; ok {
		return c
	}
	return ErrOther
}
