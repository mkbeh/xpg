# xpg

PostgreSQL client for Go built on top of [pgx](https://github.com/jackc/pgx), with connection pooling, separate
master/replica pools, built-in migrations, query builder, and OpenTelemetry observability out of the box.

## Features

- Separate connection pools for master (writer) and replica (reader)
- Query builder via [squirrel](https://github.com/Masterminds/squirrel)
- Built-in migrations via [golang-migrate](https://github.com/golang-migrate/migrate)
- Transaction helpers with panic recovery
- OpenTelemetry tracing and Prometheus metrics

## Installation

```bash
go get github.com/mkbeh/xpg
```

## Getting started

```go
package main

import (
	"context"
	"fmt"
	"log"

	"github.com/mkbeh/xpg"
)

func main() {
	cfg := &postgres.Config{
		ClusterHost:        "127.0.0.1",
		ClusterPort:        "5432",
		ClusterReplicaPort: "5432",
		User:               "user",
		Password:           "pass",
		DB:                 "postgres",
	}

	writer, err := postgres.NewWriter(
		postgres.WithConfig(cfg),
		postgres.WithClientID("my-service"),
	)
	if err != nil {
		log.Fatalln("failed to init writer pool:", err)
	}
	defer writer.Close()

	reader, err := postgres.NewReader(postgres.WithConfig(cfg))
	if err != nil {
		log.Fatalln("failed to init reader pool:", err)
	}
	defer reader.Close()

	var greeting string
	if err = writer.QueryRow(context.Background(), "select 'Hello, world!'").Scan(&greeting); err != nil {
		log.Fatalln("QueryRow failed:", err)
	}
	fmt.Println(greeting)
}
```

More examples: [examples/](https://github.com/mkbeh/xpg/tree/main/examples)

## Transactions

```go
err := writer.RunInTxx(ctx, func (ctx context.Context) error {
_, err := writer.Exec(ctx, "insert into orders (id) values ($1)", id)
return err
})
```

`RunInTxx` uses default transaction options. For custom isolation level or access mode, use `RunInTx`:

```go
err := writer.RunInTx(ctx, func (ctx context.Context) error {
// ...
}, pgx.TxOptions{IsoLevel: pgx.Serializable})
```

Rollback is automatic. Commit happens only if the function returns nil. Panics are recovered and returned as errors.

## Migrations

Create `embed.go` in your migrations directory:

```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```

Pass it via `WithMigrations`. Migrations run automatically on `NewWriter` when `MigrateEnabled` is true:

```go
writer, err := postgres.NewWriter(
postgres.WithConfig(cfg),
postgres.WithMigrations(migrations.FS),
)
```

Migration files follow standard golang-migrate naming: `000001_create_users.up.sql` / `000001_create_users.down.sql`.

## Configuration

The DSN is constructed from `Config` fields in the format:

```
postgres://user:pass@host:port/db?sslmode=disable&application_name=<id>&<args>
```

### Config struct

```go
cfg := &postgres.Config{
ClusterHost:        "127.0.0.1", // required
ClusterPort:        "5432",      // required, master port
ClusterReplicaPort: "5433", // required, replica port
User:               "user", // required
Password:           "pass",        // required
DB:                 "mydb",        // required

MaxRWConn:       16,
MaxROConn:       16,
MaxConnLifetime: 5 * time.Minute,
MaxConnIdleTime: 30 * time.Second,

MigrateEnabled: true,
}
```

### Environment variables

| Variable                              | Required | Default                 | Description                                            |
|---------------------------------------|----------|-------------------------|--------------------------------------------------------|
| `POSTGRES_CLUSTER_HOST`               | ✓        | —                       | Database host                                          |
| `POSTGRES_CLUSTER_PORT`               | ✓        | —                       | Master port                                            |
| `POSTGRES_CLUSTER_REPLICA_PORT`       | ✓        | —                       | Replica port                                           |
| `POSTGRES_USER`                       | ✓        | —                       | Database user                                          |
| `POSTGRES_PASSWORD`                   | ✓        | —                       | Database password                                      |
| `POSTGRES_DB`                         | ✓        | —                       | Database name                                          |
| `POSTGRES_SHARD_ID`                   |          | `0`                     | Shard ID (exposed in metrics)                          |
| `POSTGRES_MIN_RW_CONN`                |          | `1`                     | Min connections in writer pool                         |
| `POSTGRES_MIN_RO_CONN`                |          | `1`                     | Min connections in reader pool                         |
| `POSTGRES_MAX_RW_CONN`                |          | `max(4, NumCPU)`        | Max connections in writer pool                         |
| `POSTGRES_MAX_RO_CONN`                |          | `max(4, NumCPU)`        | Max connections in reader pool                         |
| `POSTGRES_MAX_CONN_LIFETIME`          |          | `1m`                    | Max connection lifetime                                |
| `POSTGRES_MAX_CONN_IDLE_TIME`         |          | `30s`                   | Max idle connection lifetime                           |
| `POSTGRES_QUERY_EXEC_MODE`            |          | `cache_statement`       | Query execution mode (see below)                       |
| `POSTGRES_STATEMENT_CACHE_CAPACITY`   |          | `128`                   | Statement cache size                                   |
| `POSTGRES_DESCRIPTION_CACHE_CAPACITY` |          | `512`                   | Description cache size                                 |
| `POSTGRES_MASTER_ARGS`                |          | —                       | Extra DSN args for master, e.g. `search_path=myschema` |
| `POSTGRES_REPLICA_ARGS`               |          | —                       | Extra DSN args for replica                             |
| `POSTGRES_MIGRATE_ENABLED`            |          | `false`                 | Run migrations on startup                              |
| `POSTGRES_MIGRATE_PORT`               |          | `POSTGRES_CLUSTER_PORT` | Port used for migrations                               |
| `POSTGRES_MIGRATE_ARGS`               |          | —                       | Extra DSN args for migration connection                |

### Query execution modes

| Value             | Protocol | Round trips             | Description                                                                                                                                                                        |
|-------------------|----------|-------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `cache_statement` | Extended | 1 (after cache warm-up) | Automatically prepares and caches statements. **Default.** May fail on first execution after schema changes.                                                                       |
| `cache_describe`  | Extended | 1 (after cache warm-up) | Caches argument and result type descriptions instead of full statements. Same schema-change caveat.                                                                                |
| `describe_exec`   | Extended | 2                       | Fetches description on every execution, then executes. No caching — safe with concurrent schema changes. May break connection poolers that switch connections between round trips. |
| `exec`            | Extended | 1                       | No prepare, no describe. Infers PostgreSQL types from Go types using text format. Register custom types with `pgtype.Map.RegisterDefaultPgType`.                                   |
| `simple_protocol` | Simple   | 1                       | Client-side parameter interpolation. Compatible with PgBouncer and proxies that don't support the extended protocol. Prefer `exec` when possible.                                  |

> For PgBouncer in transaction pooling mode use `simple_protocol`. For session pooling mode `cache_statement` works
> fine.

## Client options

| Option                     | Description                                                             |
|----------------------------|-------------------------------------------------------------------------|
| `WithConfig(cfg)`          | Set connection config                                                   |
| `WithClientID(id)`         | Set a human-readable client identifier (appended to `application_name`) |
| `WithLogger(l)`            | Set a custom `slog.Logger`                                              |
| `WithTraceProvider(tp)`    | Set a custom OpenTelemetry `TracerProvider`                             |
| `WithMigrations(fs...)`    | Provide one or more `embed.FS` with migration files                     |
| `WithMetricsNamespace(ns)` | Set Prometheus metrics namespace                                        |

## License

[MIT](LICENSE)