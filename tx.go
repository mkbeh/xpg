package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
)

type ctxTx struct {
	tx pgx.Tx
}

type txKey struct{}

var (
	txMarkerKey = &txKey{}
	nullTx      = &ctxTx{}
)

func injectTx(ctx context.Context, tx pgx.Tx) context.Context {
	t := &ctxTx{
		tx: tx,
	}
	return context.WithValue(ctx, txMarkerKey, t)
}

func extractTx(ctx context.Context) pgx.Tx {
	t, ok := ctx.Value(txMarkerKey).(*ctxTx)
	if !ok || t == nil {
		return nullTx.tx
	}
	return t.tx
}
