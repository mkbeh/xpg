// Package pgxtracer is a chaining QueryTracer for pgx.
package pgxtracer

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// QueryTracer traces Query, QueryRow, and Exec.
type QueryTracer interface {
	// TraceQueryStart is called at the beginning of Query, QueryRow, and Exec calls. The returned context is used for the
	// rest of the call and will be passed to TraceQueryEnd.
	TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context

	TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData)
}

type tracer struct {
	tracers []QueryTracer
}

func (t *tracer) TraceQueryStart(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	for i := range t.tracers {
		ctx = t.tracers[i].TraceQueryStart(ctx, conn, data)
	}
	return ctx
}

func (t *tracer) TraceQueryEnd(ctx context.Context, conn *pgx.Conn, data pgx.TraceQueryEndData) {
	for i := range t.tracers {
		t.tracers[i].TraceQueryEnd(ctx, conn, data)
	}
}

func New(tracers ...QueryTracer) QueryTracer {
	return &tracer{
		tracers: tracers,
	}
}
