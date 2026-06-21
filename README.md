<div align="center">

# xpg

**Lightweight PostgreSQL wrapper for Go, built on top of [pgx](https://github.com/jackc/pgx).**

![Go Version](https://img.shields.io/badge/go-1.26%2B-blue)
[![License: MIT](https://img.shields.io/badge/license-MIT-green.svg)](LICENSE)

</div>

`xpg` wraps the excellent [`pgx`](https://github.com/jackc/pgx) PostgreSQL driver with a compact API for common
PostgreSQL workflows: read/write connection pool splitting, transaction helpers, embedded SQL migrations, query
building, normalized errors, and exposing PostgreSQL observability with OpenTelemetry and Prometheus.

## Features

* **Pools**: Separate writer and reader connection pools.
* **Queries**: PostgreSQL-friendly query builder support.
* **Transactions**: Transaction helpers with automatic rollback and panic recovery.
* **Migrations**: Embedded SQL migrations via [golang-migrate](https://github.com/golang-migrate/migrate).
* **Errors**: Normalized PostgreSQL error codes for common failure cases.
* **Observability**: OpenTelemetry tracing and Prometheus metrics out of the box.
* **Configuration**: Configure via Go structs or environment variables.

## Installation

```bash
go get github.com/mkbeh/xpg
```

## Quick start

The example below creates separate writer and reader pools and runs a simple query.

```go
package main

import (
	"context"
	"fmt"
	"log"

	postgres "github.com/mkbeh/xpg"
)

func main() {
	ctx := context.Background()

	cfg := &postgres.Config{
		ClusterHost:        "127.0.0.1",
		ClusterPort:        "5432", // writer/master port
		ClusterReplicaPort: "5432", // reader/replica port; can be different in production
		User:               "user",
		Password:           "pass",
		DB:                 "postgres",
	}

	writer, err := postgres.NewWriter(
		postgres.WithConfig(cfg),
		postgres.WithClientID("my-service"), // appended to application_name and metrics labels
	)
	if err != nil {
		log.Fatal("failed to init writer pool:", err)
	}
	defer writer.Close()

	reader, err := postgres.NewReader(
		postgres.WithConfig(cfg),
		postgres.WithClientID("my-service"),
	)
	if err != nil {
		log.Fatal("failed to init reader pool:", err)
	}
	defer reader.Close()

	// Use the writer pool for write-side operations.
	if _, err := writer.Exec(ctx, "select 1"); err != nil {
		log.Fatal("writer query failed:", err)
	}

	var greeting string

	// Use the reader pool for read-only queries.
	if err := reader.QueryRow(ctx, "select 'Hello, world!'").Scan(&greeting); err != nil {
		log.Fatal("query failed:", err)
	}

	fmt.Println(greeting)
}
```

More examples: [examples/](https://github.com/mkbeh/xpg/tree/main/examples)

## Query Builder

Each pool includes a preconfigured [squirrel](https://github.com/Masterminds/squirrel) statement builder with PostgreSQL
dollar placeholders out of the box.

<!-- @formatter:off -->
```go
sql, args, err := writer.QueryBuilder().
	Insert("orders").
	Columns("id", "status").
	Values(orderID, "created").
	ToSql()
if err != nil {
	log.Fatalf("failed to build query: %v", err)
}

if _, err := writer.Exec(ctx, sql, args...); err != nil {
	log.Fatalf("failed to execute insert: %v", err)
}
```
<!-- @formatter:on -->

## Transactions

Use `RunInTxx` for transactions with default options. It acts as an alias for `RunInTx` using default `pgx.TxOptions`.

<!-- @formatter:off -->
```go
err := writer.RunInTxx(ctx, func(ctx context.Context) error {
	_, err := writer.Exec(ctx, "INSERT INTO orders (id) VALUES (\$1)", orderID)
	return err
})
if err != nil {
	log.Fatalf("transaction failed: %v", err)
}
```
<!-- @formatter:on -->

For a custom isolation level or access mode, use `RunInTx`:

<!-- @formatter:off -->
```go
err := writer.RunInTx(ctx, func(ctx context.Context) error {
	_, err := writer.Exec(ctx, "INSERT INTO orders (id) VALUES (\$1)", orderID)
	return err
}, pgx.TxOptions{
	IsoLevel: pgx.Serializable,
})
if err != nil {
	log.Fatalf("serializable transaction failed: %v", err)
}
```
<!-- @formatter:on -->

Rollback is handled automatically if an error occurs. The transaction is committed only if the function returns `nil`.
Any panics inside the block are recovered and returned as standard Go errors.

## Migrations

`xpg` supports embedded SQL migrations out of the box using [golang-migrate](https://github.com/golang-migrate/migrate).

First, create an `embed.go` file inside your migrations directory:

<!-- @formatter:off -->
```go
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS
```
<!-- @formatter:on -->

Then, pass the embedded filesystem using `WithMigrations`:

<!-- @formatter:off -->
```go
writer, err := postgres.NewWriter(
	postgres.WithConfig(&postgres.Config{
		ClusterHost:        "127.0.0.1",
		ClusterPort:        "5432",
		ClusterReplicaPort: "5432",
		User:               "user",
		Password:           "pass",
		DB:                 "postgres",
		MigrateEnabled:     true,
	}),
	postgres.WithMigrations(migrations.FS),
)
if err != nil {
	log.Fatalf("failed to initialize writer and run migrations: %v", err)
}
defer writer.Close()
```
<!-- @formatter:on -->

Migrations will run automatically during `NewWriter` initialization if `MigrateEnabled` is set to `true`.

Your SQL migration files must follow the standard `golang-migrate` naming convention:

```text
000001_create_users.up.sql
000001_create_users.down.sql
```

## Observability

`xpg` instruments PostgreSQL queries through native `pgx` tracing hooks and exposes pool metrics for Prometheus.

<!-- @formatter:off -->
```go
writer, err := postgres.NewWriter(
    postgres.WithConfig(cfg),
    postgres.WithClientID("orders-service"),
    postgres.WithTraceProvider(tracerProvider),
    postgres.WithMetricsNamespace("orders"),
)
if err != nil {
    log.Fatalf("failed to initialize observed writer pool: %v", err)
}
defer writer.Close()
```
<!-- @formatter:on -->


The following Prometheus metric labels are added automatically:

| Label | Description |
| :--- | :--- |
| `client_id` | Generated client identifier or configured ID with a unique suffix. |
| `client_kind` | `writer` for writer pools, `reader` for reader pools. |
| `db` | Database name from the configuration. |
| `shard_id` | Shard ID from the configuration. |

## Error Handling

`xpg` provides normalized PostgreSQL error codes through `ConvertError`, so application code does not need to deal with
raw `pgx` and `pgconn` error types directly.

<!-- @formatter:off -->
```go
err := writer.QueryRow(ctx, "SELECT id FROM users WHERE id = \$1", userID).Scan(&id)
if err != nil {
	pgErr := postgres.ConvertError(err)

	if pgErr.Code() == postgres.ErrNoRows {
		// handle missing row
		return nil
	}

	if pgErr.Code() == postgres.ErrSerializable {
		// retry transaction
		return nil
	}

	return pgErr
}
```
<!-- @formatter:on -->

Common PostgreSQL errors such as `ErrNoRows`, `ErrUniqViolation`, `ErrForeignKeyViolation`, and `ErrSerializable` are
mapped to stable `xpg` error codes.

## Configuration

The `Config` struct can be initialized directly in Go. It also includes `envconfig` tags, allowing you to seamlessly
populate it from environment variables using your preferred configuration library.

### Config Struct

<!-- @formatter:off -->
```go
cfg := &postgres.Config{
	ClusterHost:        "127.0.0.1", // required
	ClusterPort:        "5432",      // required, writer port
	ClusterReplicaPort: "5433",      // required, reader port
	User:               "user",      // required
	Password:           "pass",      // required
	DB:                 "mydb",      // required

	MaxRWConn:       16,
	MaxROConn:       16,
	MaxConnLifetime: 5 * time.Minute,
	MaxConnIdleTime: 30 * time.Second,

	MigrateEnabled: true,
}
```
<!-- @formatter:on -->

The connection DSN is dynamically built from the `Config` fields using the following format:

```text
postgres://user:pass@host:port/db?sslmode=disable&application_name=<id>&<args>
```

### Environment Variables

| Variable | Required | Default | Description |
| :--- | :---: | :--- | :--- |
| `POSTGRES_CLUSTER_HOST` | ✓ | — | Database host. |
| `POSTGRES_CLUSTER_PORT` | ✓ | — | Writer pool port. |
| `POSTGRES_CLUSTER_REPLICA_PORT` | ✓ | — | Reader pool port. |
| `POSTGRES_USER` | ✓ | — | Database user. |
| `POSTGRES_PASSWORD` | ✓ | — | Database password. |
| `POSTGRES_DB` | ✓ | — | Database name. |
| `POSTGRES_SHARD_ID` | | `0` | Shard ID exposed in metrics. |
| `POSTGRES_MIN_RW_CONN` | | `1` | Minimum connections in the writer pool. |
| `POSTGRES_MIN_RO_CONN` | | `1` | Minimum connections in the reader pool. |
| `POSTGRES_MAX_RW_CONN` | | `max(4, NumCPU)`| Maximum connections in the writer pool. |
| `POSTGRES_MAX_RO_CONN` | | `max(4, NumCPU)`| Maximum connections in the reader pool. |
| `POSTGRES_MAX_CONN_LIFETIME` | | `1m` | Maximum connection lifetime. |
| `POSTGRES_MAX_CONN_IDLE_TIME` | | `30s` | Maximum idle connection lifetime. |
| `POSTGRES_QUERY_EXEC_MODE` | | `cache_statement` | Query execution mode. |
| `POSTGRES_STATEMENT_CACHE_CAPACITY` | | `128` | Statement cache size. |
| `POSTGRES_DESCRIPTION_CACHE_CAPACITY`| | `512` | Description cache size. |
| `POSTGRES_WRITER_ARGS` | | — | Extra DSN args for the writer connection. |
| `POSTGRES_REPLICA_ARGS` | | — | Extra DSN args for the reader connection. |
| `POSTGRES_MIGRATE_ENABLED` | | `false` | Run migrations on writer startup. |
| `POSTGRES_MIGRATE_PORT` | | `POSTGRES_CLUSTER_PORT` | Port used for migrations. |
| `POSTGRES_MIGRATE_ARGS` | | — | Extra DSN args for the migration connection. |

### Query Execution Modes

| Value | Protocol | Round Trips | Description |
| :--- | :--- | :--- | :--- |
| `cache_statement` | Extended | 1 after warm-up | Automatically prepares and caches statements. **Default.** May fail on first execution after schema changes. |
| `cache_describe` | Extended | 1 after warm-up | Caches argument and result type descriptions instead of prepared statements. Has the same schema-change caveat. |
| `describe_exec` | Extended | 2 | Fetches description on every execution, then executes. Safer with concurrent schema changes, but may break with connection poolers that switch connections between round trips. |
| `exec` | Extended | 1 | No prepare and no describe. Infers PostgreSQL types from Go types using text format. Register custom types with `pgtype.Map.RegisterDefaultPgType`. |
| `simple_protocol` | Simple | 1 | Uses the simple protocol. Useful with PgBouncer or proxies that do not support the extended protocol. Prefer `exec` when possible. |

> 💡 **Tip for connection pooling:** For PgBouncer **transaction** pooling, use `simple_protocol`. For **session**
> pooling, `cache_statement` usually works fine.

## License

This project is licensed under the [MIT License](LICENSE).
