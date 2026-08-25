<div align="center">

# Postgres toolkit for Go

**Lightweight PostgreSQL wrapper for Go, built on top of [pgx](https://github.com/jackc/pgx).**

[![Go Reference](https://pkg.go.dev/badge/github.com/mkbeh/xpg.svg)](https://pkg.go.dev/github.com/mkbeh/xpg)
[![Test](https://github.com/mkbeh/xpg/actions/workflows/test.yml/badge.svg)](https://github.com/mkbeh/xpg/actions/workflows/test.yml)
[![Coverage](https://codecov.io/gh/mkbeh/xpg/graph/badge.svg)](https://codecov.io/gh/mkbeh/xpg)

</div>

`xpg` builds on the `pgx` client with a compact API for common PostgreSQL infrastructure patterns. It adds support for
pool lifecycle, transactions and savepoints, PostgreSQL error classification, advisory locks, primary/replica routing,
application-level sharding, and observability.

The library uses `pgx` types and query model directly while keeping its core behavior and reducing boilerplate around
connection management, routing, and common production workflows.

## Features

* **Pool Lifecycle Management:** Thin pool management on top of `pgx` with direct access to the underlying PostgreSQL
  client.
* **Transactions and Savepoints:** Managed transactions, savepoints, and helpers for common multi-step transactional
  workflows.
* **Error Classification:** Classification of PostgreSQL constraint, transaction, cancellation, connection, and other
  common database errors.
* **Advisory Locking:** Transaction-level advisory locks for coordinating concurrent database operations.
* **Primary/Replica Routing:** Explicit read policies, replica selection, primary fallback, and read-only transactions
  across PostgreSQL nodes.
* **Application-Level Sharding:** Hash, range, time-based, and custom routing with colocation checks, key grouping, and
  bounded parallel operations across shards.
* **Observability:** Structured logging, tracing, pool statistics, and optional OpenTelemetry metrics.

## Installation

This repository contains the core `xpg` module. The core module is released from the repository root:

```bash
go get github.com/mkbeh/xpg
```

Optional integrations are released independently under `extra`:

```bash
go get github.com/mkbeh/xpg/extra/otelxpg
```

## Usage

Open an `xpg` pool and execute a PostgreSQL query:

<!-- @formatter:off -->
```go
// urlExample := "postgres://username:password@localhost:5432/database_name"
pool, err := xpg.Open(
	context.Background(),
	os.Getenv("DATABASE_URL"),
	xpg.WithName("example-pool"),
)
if err != nil {
	log.Fatalf("failed to open pool: %v", err)
}
defer pool.Close()

var message string
err = pool.QueryRow(context.Background(), "SELECT 'hello from xpg'").Scan(&message)
if err != nil {
	log.Fatalf("query failed: %v", err)
}

fmt.Println(message) // Outputs: hello from xpg
```
<!-- @formatter:on -->

### Transactions

`xpg` provides managed transactions using the native `pgx` transaction API. Returning `nil` commits the transaction;
returning an error rolls it back.

<!-- @formatter:off -->
```go
err := pool.InTx(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", userID)
	return err
})
```
<!-- @formatter:on -->

Savepoints can isolate optional work without aborting the outer transaction.

### Advisory Locks

`xpg` provides transaction-level PostgreSQL advisory locks for coordinating concurrent work.

<!-- @formatter:off -->
```go
err := pool.InTx(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
	if err := xpg.AdvisoryXactLock(ctx, tx, lockID); err != nil {
		return err
	}

	_, err := tx.Exec(ctx, "UPDATE jobs SET status = 'running' WHERE id = $1", jobID)
	return err
})
```
<!-- @formatter:on -->

The lock is held for the duration of the transaction and released automatically on commit or rollback.

### Error Handling

`xpg` provides helpers for classifying PostgreSQL errors and inspecting SQLSTATE codes.

<!-- @formatter:off -->
```go
_, err := pool.Exec(ctx, "INSERT INTO users (id, email) VALUES ($1, $2)", userID, email)

switch {
case xpg.IsUniqueViolation(err):
    // Handle duplicate data.
case xpg.IsRetryableTransaction(err):
    // Retry the transaction when the operation is safe to replay.
case err != nil:
    return err
}
```
<!-- @formatter:on -->

The underlying SQLSTATE code is also available through `xpg.SQLState`. Helpers cover constraint violations,
serialization failures, deadlocks, lock errors, query cancellation, and connection failures.

## Clustering

`xpg` groups primary and replica pools into a logical cluster with explicit read routing.

<!-- @formatter:off -->
```go
orders, err := cluster.New(cluster.Config{
	ID:       "orders",
	Primary:  primary,
	Replicas: []*xpg.Pool{replicaA, replicaB},
})
if err != nil {
	panic(err)
}
defer orders.Close()

// Route writes explicitly to the primary.
primaryPool := orders.Primary()

_, err = primaryPool.Exec(ctx, "UPDATE orders SET status = 'processed' WHERE id = $1", orderID)
if err != nil {
	panic(err)
}

// Route reads according to the selected policy.
readPool, err := orders.ReadPool(ctx, cluster.ReadReplicaPreferred)
if err != nil {
	panic(err)
}

var status string
err = readPool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", orderID).Scan(&status)
if err != nil {
	panic(err)
}
```
<!-- @formatter:on -->

Read policies support primary-only, replica-required, and replica-preferred routing with primary fallback when no
replica is available. Replica selection is round-robin by default and can be customized.

## Sharding

`xpg` provides application-level sharding with explicit key routing across an immutable shard topology.

<!-- @formatter:off -->
```go
topology, err := shard.NewTopology([]shard.Config{
    {Cluster: shardA},
    {Cluster: shardB},
})
if err != nil {
    panic(err)
}
defer topology.Close()

// Partition user IDs into shard ranges.
users, err := resolver.NewRange(
    topology,
    []resolver.Range[uint64]{
        {Start: 0, End: 100, ShardID: "shard-a"},
        {Start: 100, End: 200, ShardID: "shard-b"},
    },
)
if err != nil {
    panic(err)
}

// Resolve the target shard.
shard, err := users.Resolve(userID)
if err != nil {
    panic(err)
}

// Write to the shard primary.
primaryPool := shard.Primary()

_, err = primaryPool.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", userID)
if err != nil {
    panic(err)
}

// Read from the same shard using the selected read policy.
readPool, err := shard.ReadPool(ctx, cluster.ReadReplicaPreferred)
if err != nil {
    panic(err)
}

var active bool
err = readPool.QueryRow(ctx, "SELECT active FROM users WHERE id = $1", userID).Scan(&active)
if err != nil {
    panic(err)
}
```
<!-- @formatter:on -->

Built-in routing strategies include rendezvous hashing, ordered ranges, time ranges, and custom resolvers. Sharding
utilities cover key colocation, grouping by shard, and parallel operations across shards.

## Examples

See the [examples](examples) directory for runnable examples covering the main `xpg` usage patterns.

## License

This project is licensed under the [MIT License](LICENSE).
