package xpg

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Pool is a concurrency-safe PostgreSQL connection pool backed by pgxpool.
type Pool struct {
	pool *pgxpool.Pool

	name   string
	labels map[string]string

	closeOnce sync.Once
}

// Open parses a DSN and creates a Pool.
func Open(ctx context.Context, dsn string, options ...Option) (*Pool, error) {
	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("xpg: parse pool config: %w", err)
	}

	return New(ctx, config, options...)
}

// New creates a Pool from config.
//
// Config must have been created by pgxpool.ParseConfig. New passes a defensive
// copy to pgxpool, so subsequent changes to the original config do not affect
// the created Pool.
//
// As with pgxpool.Config.Copy, the referenced tls.Config remains shared and
// must not be modified after it has been used to create connections.
func New(ctx context.Context, config *pgxpool.Config, options ...Option) (*Pool, error) {
	if config == nil || config.ConnConfig == nil {
		return nil, errors.New("xpg: pool config is nil")
	}

	settings := defaultSettings()

	if err := applyOptions(settings, options...); err != nil {
		return nil, err
	}

	pgxPool, err := pgxpool.NewWithConfig(ctx, config.Copy())
	if err != nil {
		return nil, fmt.Errorf("xpg: create pool: %w", err)
	}

	return &Pool{
		pool:   pgxPool,
		name:   settings.name,
		labels: cloneLabels(settings.labels),
	}, nil
}

// Name returns the logical pool name configured with WithName.
func (p *Pool) Name() string {
	return p.name
}

// Raw returns the underlying pgxpool.Pool.
func (p *Pool) Raw() *pgxpool.Pool {
	return p.pool
}

func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Close closes the underlying pool and waits for acquired connections to be
// returned. Close is safe to call multiple times.
func (p *Pool) Close() {
	p.closeOnce.Do(p.pool.Close)
}

func (p *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, arguments...)
}

func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

func (p *Pool) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	return p.pool.SendBatch(ctx, batch)
}

func (p *Pool) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return p.pool.CopyFrom(ctx, tableName, columnNames, rowSrc)
}
