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
	pool    *pgxpool.Pool
	metrics MetricsRegistration

	name   string
	labels map[string]string

	closeOnce sync.Once
}

// Open parses a PostgreSQL connection string and creates a Pool.
func Open(ctx context.Context, connString string, options ...Option) (*Pool, error) {
	config, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("xpg: parse pool config: %w", err)
	}

	return New(ctx, config, options...)
}

// New creates a Pool from config.
//
// Config must have been created by pgxpool.ParseConfig. New passes a defensive
// copy to pgxpool, so subsequent changes to config do not affect the Pool.
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

	poolConfig := config.Copy()
	connConfig := poolConfig.ConnConfig

	if tracer := settings.buildTracer(); tracer != nil {
		connConfig.Tracer = tracer
	}

	pgxPool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("xpg: create pool: %w", err)
	}

	pool := &Pool{
		pool: pgxPool,
		name: settings.poolName(
			connConfig.Host,
			connConfig.Port,
			connConfig.Database,
		),
		labels: cloneLabels(settings.labels),
	}

	if err := pool.registerMetrics(settings.metrics); err != nil {
		pool.Close()

		return nil, fmt.Errorf("xpg: register pool metrics: %w", err)
	}

	return pool, nil
}

// Name returns the logical pool name.
//
// If WithName is not configured, the name is derived from the connection host,
// port, and database.
func (p *Pool) Name() string {
	return p.name
}

// Labels returns a copy of the pool labels.
func (p *Pool) Labels() map[string]string {
	return cloneLabels(p.labels)
}

// Raw returns the underlying pgxpool.Pool.
//
// The returned pool is owned by Pool and must not be closed directly.
func (p *Pool) Raw() *pgxpool.Pool {
	return p.pool
}

// Ping verifies connectivity to PostgreSQL.
func (p *Pool) Ping(ctx context.Context) error {
	return p.pool.Ping(ctx)
}

// Close closes the pool and waits for acquired connections to be returned.
// Close is safe to call multiple times.
func (p *Pool) Close() {
	p.closeOnce.Do(func() {
		if p.metrics != nil {
			p.metrics.Close()
		}

		p.pool.Close()
	})
}

// Exec executes SQL against the pool.
func (p *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	return p.pool.Exec(ctx, sql, arguments...)
}

// Query executes SQL and returns the resulting rows.
func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	return p.pool.Query(ctx, sql, args...)
}

// QueryRow executes SQL that is expected to return at most one row.
func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return p.pool.QueryRow(ctx, sql, args...)
}

// SendBatch sends a batch of queries through the pool.
func (p *Pool) SendBatch(ctx context.Context, batch *pgx.Batch) pgx.BatchResults {
	return p.pool.SendBatch(ctx, batch)
}

// CopyFrom copies rows into the specified table.
func (p *Pool) CopyFrom(ctx context.Context, tableName pgx.Identifier, columnNames []string, rowSrc pgx.CopyFromSource) (int64, error) {
	return p.pool.CopyFrom(ctx, tableName, columnNames, rowSrc)
}

func (p *Pool) registerMetrics(metrics Metrics) error {
	if metrics == nil {
		return nil
	}

	registration, err := metrics.Register(p)
	if err != nil {
		return err
	}

	p.metrics = registration

	return nil
}
