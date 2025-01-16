package postgres

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/tracelog"
	poolcollector "github.com/mkbeh/xpg/internal/pkg/pgxpoolcollector/v5"
	"github.com/mkbeh/xpg/internal/pkg/pgxslog"
	"github.com/mkbeh/xpg/internal/pkg/pgxtracer"
	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
)

type Pool struct {
	*pgxpool.Pool

	id            string
	cfg           *Config
	logger        *slog.Logger
	traceProvider trace.TracerProvider
	qBuilder      squirrel.StatementBuilderType
	migrations    []embed.FS
	namespace     string
	labels        prometheus.Labels
}

type options struct {
	dsn                      string
	minConns                 int32
	maxConns                 int32
	maxConnLifetime          time.Duration
	maxConnIdleTime          time.Duration
	statementCacheCapacity   int
	descriptionCacheCapacity int
	defaultQueryExecMode     pgx.QueryExecMode
	logger                   *slog.Logger
	traceProvider            trace.TracerProvider
	tracers                  []pgxtracer.QueryTracer
}

func NewWriter(opts ...Option) (*Pool, error) {
	return newPool(true, opts)
}

func NewReader(opts ...Option) (*Pool, error) {
	return newPool(false, opts)
}

func newPool(writer bool, opts []Option) (*Pool, error) {
	p := &Pool{
		cfg:      &Config{},
		logger:   slog.Default(),
		qBuilder: squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar),
	}

	for _, opt := range opts {
		opt.apply(p)
	}

	p.cfg.writer = writer
	p.cfg.appName = p.getID()

	if p.traceProvider == nil {
		p.traceProvider = otel.GetTracerProvider()
	}

	if writer {
		p.logger.With(pgxslog.Component("postgres_master"))
	} else {
		p.logger.With(pgxslog.Component("postgres_replica"))
	}

	connOpts := parseConfig(p.cfg)
	connOpts.logger = p.logger
	connOpts.traceProvider = p.traceProvider

	conn, err := connect(connOpts)
	if err != nil {
		return nil, err
	}

	p.Pool = conn
	p.exposeMetrics(writer)

	collector := poolcollector.NewStatsCollector(p.namespace, "postgres", p.labels, p.Pool)
	prometheus.MustRegister(collector)

	if p.cfg.writer && p.cfg.MigrateEnabled {
		for _, fs := range p.migrations {
			if err := applyMigrations(fs, p.cfg.getMigrateDSN(), p.logger); err != nil {
				return nil, err
			}
		}
	}

	return p, err
}

func (p *Pool) QueryBuilder() squirrel.StatementBuilderType {
	return p.qBuilder
}

func (p *Pool) Logger() *slog.Logger {
	return p.logger
}

func (p *Pool) Close() error {
	p.Pool.Close()
	return nil
}

func (p *Pool) SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults {
	if tx := extractTx(ctx); tx != nil {
		return tx.SendBatch(ctx, b)
	}
	return p.Pool.SendBatch(ctx, b)
}

func (p *Pool) Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error) {
	if tx := extractTx(ctx); tx != nil {
		return tx.Exec(ctx, sql, arguments...)
	}
	return p.Pool.Exec(ctx, sql, arguments...)
}

func (p *Pool) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	if tx := extractTx(ctx); tx != nil {
		return tx.Query(ctx, sql, args...)
	}
	return p.Pool.Query(ctx, sql, args...)
}

func (p *Pool) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	if tx := extractTx(ctx); tx != nil {
		return tx.QueryRow(ctx, sql, args...)
	}
	return p.Pool.QueryRow(ctx, sql, args...)
}

// RunInTxx alias for RunInTx.
func (p *Pool) RunInTxx(ctx context.Context, fn func(ctx context.Context) error) error {
	return p.RunInTx(ctx, fn, pgx.TxOptions{})
}

func (p *Pool) RunInTx(ctx context.Context, fn func(ctx context.Context) error, txOptions pgx.TxOptions) (err error) {
	tx, err := p.Pool.BeginTx(ctx, txOptions)
	if err != nil {
		p.Logger().ErrorContext(ctx, "failed to begin transaction", pgxslog.Error(err))
		return NewPgError(ErrBeginTransaction, err)
	}

	defer func() {
		if r := recover(); r != nil {
			p.Logger().ErrorContext(ctx, "panic recovered", slog.Any("error", r))
			err = NewPgError(ErrOther, fmt.Errorf("%v", r))
		}

		if rErr := tx.Rollback(ctx); rErr != nil {
			if !errors.Is(rErr, pgx.ErrTxClosed) {
				p.Logger().ErrorContext(ctx, "failed to rollback transaction", pgxslog.Error(rErr))
			}
		}
	}()

	if err = fn(injectTx(ctx, tx)); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		p.Logger().ErrorContext(ctx, "failed to commit transaction", pgxslog.Error(err))
		return NewPgError(ErrCommitTransaction, err)
	}

	return nil
}

func (p *Pool) AcquireTxLock(ctx context.Context, key string, durationSeconds float64) (isLocked bool, err error) {
	row := p.QueryRow(ctx, `
		SELECT CASE
           WHEN pg_try_advisory_xact_lock($1) THEN (SELECT concat(pg_sleep($2), 'false'))::bool
           ELSE true
           END AS is_locked;`,
		int64(StringAsHash64(key)),
		durationSeconds)
	err = row.Scan(&isLocked)
	return
}

func (p *Pool) getID() string {
	if p.id == "" {
		return GenerateUUID()
	}
	return p.id
}

func (p *Pool) exposeMetrics(writer bool) {
	if p.labels == nil {
		p.labels = make(prometheus.Labels)
	}

	p.labels["client_id"] = p.getID()
	p.labels["db"] = p.cfg.DB
	p.labels["shard_id"] = strconv.Itoa(p.cfg.ShardID)

	if writer {
		p.labels["client_kind"] = "master"
	} else {
		p.labels["client_kind"] = "replica"
	}
}

func connect(opts *options) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(opts.dsn)
	if err != nil {
		return nil, err
	}

	opts.tracers = append(opts.tracers,
		&tracelog.TraceLog{Logger: pgxslog.NewLogger(opts.logger), LogLevel: tracelog.LogLevelError},
		otelpgx.NewTracer(
			otelpgx.WithTrimSQLInSpanName(),
			otelpgx.WithTracerProvider(opts.traceProvider),
		),
	)

	poolCfg.MinConns = opts.minConns
	poolCfg.MaxConns = opts.maxConns
	poolCfg.MaxConnLifetime = opts.maxConnLifetime
	poolCfg.MaxConnIdleTime = opts.maxConnIdleTime

	poolCfg.ConnConfig.StatementCacheCapacity = opts.statementCacheCapacity
	poolCfg.ConnConfig.DescriptionCacheCapacity = opts.descriptionCacheCapacity
	poolCfg.ConnConfig.DefaultQueryExecMode = opts.defaultQueryExecMode
	poolCfg.ConnConfig.Tracer = pgxtracer.New(opts.tracers...)

	ctx := context.Background()

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, err
	}

	if err := pool.Ping(ctx); err != nil {
		return nil, err
	}

	return pool, nil
}
