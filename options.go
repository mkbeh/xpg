package postgres

import (
	"embed"
	"fmt"
	"log/slog"
	"runtime"
	"time"

	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/trace"
)

// An Option lets you add opts to pberrors interceptors using With* funcs.
type Option interface {
	apply(p *Pool)
}

type optionFunc func(p *Pool)

func (f optionFunc) apply(p *Pool) {
	f(p)
}

func WithLogger(l *slog.Logger) Option {
	return optionFunc(func(p *Pool) {
		if l != nil {
			p.logger = l
		}
	})
}

func WithConfig(config *Config) Option {
	return optionFunc(func(p *Pool) {
		if config != nil {
			p.cfg = config
		}
	})
}

func WithClientID(id string) Option {
	return optionFunc(func(p *Pool) {
		if id != "" {
			p.id = fmt.Sprintf("%s-%s", id, GenerateUUID())
		}
	})
}

func WithTraceProvider(provider trace.TracerProvider) Option {
	return optionFunc(func(p *Pool) {
		p.traceProvider = provider
	})
}

func WithMigrations(migrations ...embed.FS) Option {
	return optionFunc(func(p *Pool) {
		if len(migrations) > 0 {
			p.migrations = migrations
		}
	})
}

func WithMetricsNamespace(ns string) Option {
	return optionFunc(func(p *Pool) {
		if ns != "" {
			p.namespace = ns
		}
	})
}

type Config struct {
	ShardID            int    `envconfig:"POSTGRES_SHARD_ID"`
	ClusterHost        string `envconfig:"POSTGRES_CLUSTER_HOST" required:"true"`
	ClusterPort        string `envconfig:"POSTGRES_CLUSTER_PORT" required:"true"`
	ClusterReplicaPort string `envconfig:"POSTGRES_CLUSTER_REPLICA_PORT" required:"true"`
	User               string `envconfig:"POSTGRES_USER" required:"true"`
	Password           string `envconfig:"POSTGRES_PASSWORD" required:"true"`
	DB                 string `envconfig:"POSTGRES_DB" required:"true"`

	MinRWConn                int32         `envconfig:"POSTGRES_MIN_RW_CONN"`
	MinROConn                int32         `envconfig:"POSTGRES_MIN_RO_CONN"`
	MaxRWConn                int32         `envconfig:"POSTGRES_MAX_RW_CONN"`
	MaxROConn                int32         `envconfig:"POSTGRES_MAX_RO_CONN"`
	MaxConnLifetime          time.Duration `envconfig:"POSTGRES_MAX_CONN_LIFETIME"`
	MaxConnIdleTime          time.Duration `envconfig:"POSTGRES_MAX_CONN_IDLE_TIME"`
	QueryExecMode            string        `envconfig:"POSTGRES_QUERY_EXEC_MODE"`
	StatementCacheCapacity   int           `envconfig:"POSTGRES_STATEMENT_CACHE_CAPACITY"`
	DescriptionCacheCapacity int           `envconfig:"POSTGRES_DESCRIPTION_CACHE_CAPACITY"`

	MasterArgs  string `envconfig:"POSTGRES_MASTER_ARGS"`
	ReplicaArgs string `envconfig:"POSTGRES_REPLICA_ARGS"`

	MigrateEnabled bool   `envconfig:"POSTGRES_MIGRATE_ENABLED"`
	MigrateArgs    string `envconfig:"POSTGRES_MIGRATE_ARGS"`
	MigratePort    string `envconfig:"POSTGRES_MIGRATE_PORT"`

	writer  bool
	appName string
}

// DSN postgres://username:password@host:port/db?sslmode=disable&<args_string>
func (c *Config) getDSN() string {
	return formatDSN(c.User, c.Password, c.ClusterHost, c.getPort(), c.DB, c.appName, c.getArgs())
}

func (c *Config) getMigrateDSN() string {
	return formatDSN(c.User, c.Password, c.ClusterHost, c.getMigratePort(), c.DB, c.appName, c.MigrateArgs)
}

func (c *Config) getPort() string {
	if c.writer {
		return c.ClusterPort
	}
	return c.ClusterReplicaPort
}

func (c *Config) getMigratePort() string {
	if c.MigratePort != "" {
		return c.MigratePort
	}
	return c.ClusterPort
}

func (c *Config) getArgs() string {
	if c.writer {
		return c.MasterArgs
	}
	return c.ReplicaArgs
}

func (c *Config) getMinConns() int32 {
	if c.writer {
		return c.MinRWConn
	}
	return c.MinROConn
}

func (c *Config) getMaxConns() int32 {
	if c.writer {
		return c.MaxRWConn
	}
	return c.MaxROConn
}

func formatDSN(user, pass, host, port, db, appName, args string) string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&application_name=%s&%s",
		user, pass, host, port, db, appName, args,
	)
}

const (
	QueryExecModeCacheStatement = "cache_statement"
	QueryExecModeCacheDescribe  = "cache_describe"
	QueryExecModeDescribeExec   = "describe_exec"
	QueryExecModeExec           = "exec"
	QueryExecModeSimpleProtocol = "simple_protocol"
)

func getQueryExecMode(mode string) pgx.QueryExecMode {
	switch mode {
	case QueryExecModeCacheStatement:
		return pgx.QueryExecModeCacheStatement

	case QueryExecModeCacheDescribe:
		return pgx.QueryExecModeCacheDescribe

	case QueryExecModeDescribeExec:
		return pgx.QueryExecModeDescribeExec

	case QueryExecModeExec:
		return pgx.QueryExecModeExec

	case QueryExecModeSimpleProtocol:
		return pgx.QueryExecModeSimpleProtocol

	default:
		return pgx.QueryExecModeCacheStatement
	}
}

func parseConfig(cfg *Config) *options {
	o := &options{
		dsn:                      cfg.getDSN(),
		minConns:                 1,
		maxConns:                 4,
		maxConnLifetime:          time.Minute * 1,
		maxConnIdleTime:          time.Second * 30,
		defaultQueryExecMode:     getQueryExecMode(cfg.QueryExecMode),
		statementCacheCapacity:   128,
		descriptionCacheCapacity: 512,
	}

	if cfg.getMinConns() > 0 {
		o.minConns = cfg.getMinConns()
	}

	if cfg.getMaxConns() > 0 {
		o.maxConns = cfg.getMaxConns()
		if numCPU := int32(runtime.NumCPU()); numCPU > cfg.getMaxConns() {
			o.maxConns = numCPU
		}
	}

	if cfg.MaxConnLifetime > 0 {
		o.maxConnLifetime = cfg.MaxConnLifetime
	}

	if cfg.MaxConnIdleTime > 0 {
		o.maxConnIdleTime = cfg.MaxConnIdleTime
	}

	if cfg.StatementCacheCapacity > 0 {
		o.statementCacheCapacity = cfg.StatementCacheCapacity
	}

	if cfg.DescriptionCacheCapacity > 0 {
		o.descriptionCacheCapacity = cfg.DescriptionCacheCapacity
	}

	return o
}
