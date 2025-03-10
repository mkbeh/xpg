# Postgres Library

This library provides an API for working with Postgres, using [pgx](github.com/jackc/pgx) and
integration with OpenTelemetry for tracing and metrics.

## Features

- Connection pool
- Master and replica separate connections
- Query Builder such as [squirrel](github.com/Masterminds/squirrel)
- Built-in migrations using [golang-migrate](github.com/golang-migrate/migrate)
- Transactions
- Observability

## Getting started

Here's a basic overview of using (more examples can be found [here](https://github.com/mkbeh/xpg/tree/main/examples)):

```go
package main

import (
	"context"
	"fmt"
	"github.com/mkbeh/xpg"
	"log"
)

func main() {
	cfg := &postgres.Config{
		ClusterHost:        "127.0.0.1",
		ClusterPort:        "5432",
		ClusterReplicaPort: "5432",
		User:               "user",
		Password:           "pass",
		DB:                 "postgres",
		MigrateEnabled:     true,
	}

	writer, err := postgres.NewWriter(
		postgres.WithConfig(cfg),
		postgres.WithClientID("test-client"),
	)
	if err != nil {
		log.Fatalln("failed init master pool", err)
	}
	defer writer.Close()

	var greeting string
	err = writer.QueryRow(context.Background(), "select 'Hello, world!'").Scan(&greeting)
	if err != nil {
		log.Fatalln("QueryRow failed", err)
	}

	fmt.Println(greeting)
}

```

## Migrations

Full example can be found [here](https://github.com/mkbeh/xpg/tree/main/examples).

Create file `embed.go` in your migrations directory:
```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

Pass `embed.FS` with option `WithMigrations`

```go
writer, _ := postgres.NewWriter(
    ...
    postgres.WithMigrations(migrations.FS),
)
```

## Configuration

DSN is formed from the configuration parameters for connection in URL format.

**Default args:**

* `sslmode=disable`
* `application_name=<client_id>` (or random UUID)

Additional args that can be added:

* `MASTER_ARGS = standard_conforming_strings=on&search_path=<your-schema-name>`
* `REPLICA_ARGS = standard_conforming_strings=on&search_path=<your-schema-name>`

Available client options:

| ENV                                 | Required | Description                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
|-------------------------------------|----------|-----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| POSTGRES_SHARD_ID                   | -        | Shard ID. **Default**: 0.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                 |
| POSTGRES_CLUSTER_HOST               | true     | Database host.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| POSTGRES_CLUSTER_PORT               | true     | Database master port.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| POSTGRES_CLUSTER_REPLICA_PORT       | true     | Database replica port.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| POSTGRES_USER                       | true     | Database user.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| POSTGRES_PASSWORD                   | true     | Database password.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| POSTGRES_DB                         | true     | Database name.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| POSTGRES_MIN_RW_CONN                | -        | Is the minimum size of the pool. After connection closes, the pool might dip below MinConns. A low number of MinConns might mean the pool is empty after MaxConnLifetime until the health check has a chance to create new connections. **Default**: 1.                                                                                                                                                                                                                                                                                                   |
| POSTGRES_MIN_RO_CONN                | -        | e.g. **POSTGRES_MIN_RW_CONN**.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| POSTGRES_MAX_RW_CONN                | -        | Is the maximum size of the pool. The default is the greater of 4 or runtime.NumCPU().                                                                                                                                                                                                                                                                                                                                                                                                                                                                     |
| POSTGRES_MAX_RO_CONN                | -        | e.g. **POSTGRES_MAX_RW_CONN**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| POSTGRES_MAX_CONN_LIFETIME          | -        | Is the duration since creation after which a connection will be automatically closed. **Default**: 1m.                                                                                                                                                                                                                                                                                                                                                                                                                                                    |
| POSTGRES_MAX_CONN_IDLE_TIME         | -        | Is the duration after which an idle connection will be automatically closed by the health check. **Default**: 30s.                                                                                                                                                                                                                                                                                                                                                                                                                                        |
| POSTGRES_QUERY_EXEC_MODE            | -        | Controls the default mode for executing queries. By default pgx uses the extended protocol and automatically prepares and caches prepared statements. However, this may be incompatible with proxies such as PGBouncer. In this case it may be preferable to use QueryExecModeExec or QueryExecModeSimpleProtocol. The same functionality can be controlled on a per query basis by passing a QueryExecMode as the first query argument. **Values**: cache_statement, cache_describe, describe_exec, exec, simple_protocol. **Default**: cache_statement. |
| POSTGRES_STATEMENT_CACHE_CAPACITY   | -        | Is maximum size of the statement cache used when executing a query with "cache_statement" query exec mode. **Default**: 128.                                                                                                                                                                                                                                                                                                                                                                                                                              |
| POSTGRES_DESCRIPTION_CACHE_CAPACITY | -        | Is the maximum size of the description cache used when executing a query with "cache_describe" query exec mode. **Default**: 512.                                                                                                                                                                                                                                                                                                                                                                                                                         |
| POSTGRES_MASTER_ARGS                | -        | Additional arguments for connection string.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| POSTGRES_REPLICA_ARGS               | -        | e.g. **POSTGRES_REPLICA_ARGS**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            |
| POSTGRES_MIGRATE_ENABLED            | -        | Enable migrations if passed. **Default**: false.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                          |
| POSTGRES_MIGRATE_ARGS               | -        | e.g. **POSTGRES_MASTER_ARGS**                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                             |
| POSTGRES_MIGRATE_PORT               | -        | Migrate port. if value is not passed by default value is used **POSTGRES_CLUSTER_PORT**.                                                                                                                                                                                                                                                                                                                                                                                                                                                                  |