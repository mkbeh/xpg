package postgres

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

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
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&application_name=%s&%s",
		c.User,
		c.Password,
		c.ClusterHost,
		c.getPort(),
		c.DB,
		c.appName,
		c.getArgs(),
	)
}

func (c *Config) getMigrateDSN() string {
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable&application_name=%s&%s",
		c.User,
		c.Password,
		c.ClusterHost,
		c.getMigratePort(),
		c.DB,
		c.appName,
		c.MigrateArgs,
	)
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
