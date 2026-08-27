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
* **Primary/Replica Routing:** Logical cluster topologies with explicit read policies, replica selection, primary
  fallback, and read-only transactions across PostgreSQL nodes.
* **Application-Level Sharding:** Rendezvous, range, time-based, and custom routing with colocation checks, key
  grouping, and bounded parallel operations across shards.
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

Transaction-level PostgreSQL advisory locks can coordinate concurrent work across application instances using the same
database. The lock is held for the lifetime of the transaction and released automatically on commit or rollback.

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

For error handling, `xpg` provides semantic helpers for classifying PostgreSQL failures and inspecting SQLSTATE codes
without manual string matching.

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

The underlying SQLSTATE code is available through `xpg.SQLState(err)`. Helpers cover constraint violations,
serialization failures, deadlocks, lock errors, query cancellation, and connection failures.

## Clustering

The `topology/cluster` package groups primary and replica pools into a logical cluster with explicit read routing.

<!-- @formatter:off -->
```go
wallets, err := cluster.New(cluster.Config{
	ID:       "wallets",
	Primary:  primary,
	Replicas: []*xpg.Pool{replicaA, replicaB},
})
if err != nil {
	panic(err)
}
defer wallets.Close()

// Route writes explicitly to the primary.
primaryPool := wallets.Primary()

_, err = primaryPool.Exec(ctx, "UPDATE wallets SET frozen = true WHERE id = $1", walletID)
if err != nil {
	panic(err)
}

// Route reads according to the selected policy.
readPool, err := wallets.ReadPool(ctx, cluster.ReadReplicaPreferred)
if err != nil {
	panic(err)
}

var frozen bool
err = readPool.QueryRow(ctx, "SELECT frozen FROM wallets WHERE id = $1", walletID).Scan(&frozen)
if err != nil {
	panic(err)
}
```
<!-- @formatter:on -->

### Cluster Transactions

Cluster transactions combine explicit primary/replica routing with the native `pgx` transaction API.

#### Primary Transactions

`InPrimaryTx` runs the transaction on the cluster primary and is intended for atomic multi-step writes.

<!-- @formatter:off -->

```go
err := wallets.InPrimaryTx(
    ctx,
    pgx.TxOptions{},
    func(ctx context.Context, tx pgx.Tx) error {
        const debitQuery = `
            UPDATE wallets
            SET balance = balance - $1
            WHERE id = $2 AND balance >= $1
        `

        result, err := tx.Exec(ctx, debitQuery, amount, fromID)
        if err != nil {
            return err
        }

        if result.RowsAffected() == 0 {
            return errors.New("insufficient funds or wallet not found")
        }

        const creditQuery = `
            UPDATE wallets
            SET balance = balance + $1
            WHERE id = $2
        `

        _, err = tx.Exec(ctx, creditQuery, amount, toID)

        return err
    },
)
```

<!-- @formatter:on -->

The transaction commits on `nil` and rolls back on error.

#### Read Transactions

`InReadTx` routes the transaction according to the selected read policy and enforces PostgreSQL read-only mode. Use it
for read workloads that can run on replicas.

<!-- @formatter:off -->

```go
var (
    totalWallets int64
    totalBalance int64
)

err := wallets.InReadTx(
    ctx,
    cluster.ReadReplicaPreferred,
    cluster.ReadTxOptions{
        IsoLevel: pgx.RepeatableRead,
    },
    func(ctx context.Context, tx pgx.Tx) error {
        const query = `
            SELECT count(*), coalesce(sum(balance), 0)
            FROM wallets
            WHERE created_at > $1
        `

        return tx.QueryRow(ctx, query, since).Scan(
            &totalWallets,
            &totalBalance,
        )
    },
)
```

<!-- @formatter:on -->

#### Read Routing Policies

Read policies control how reads and read-only transactions are routed across the cluster:

| Policy                 | Primary Fallback | Behavior                                                                              |
| :--------------------- | :--------------: | :------------------------------------------------------------------------------------ |
| `ReadPrimary`          |         —        | Always routes reads to the primary.                                                   |
| `ReadReplicaRequired`  |        No        | Requires a replica and returns `ErrNoReplica` when none can be selected.              |
| `ReadReplicaPreferred` |        Yes       | Prefers a replica and falls back to the primary only when no replica can be selected. |

Replica selection uses round-robin by default and can be customized by implementing `ReplicaSelector`.

## Sharding

The `topology/shard` package provides application-level sharding with explicit key routing across an immutable shard
topology. Routing strategies live under `topology/shard/resolver`.

<!-- @formatter:off -->
```go
topology, err := shard.NewTopology(clusterA, ClusterB)
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
targetShard, err := users.Resolve(userID)
if err != nil {
	panic(err)
}

// Write to the shard primary.
primaryPool := targetShard.Primary()

_, err = primaryPool.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", userID)
if err != nil {
	panic(err)
}

// Read from the same shard using the selected read policy.
readPool, err := targetShard.ReadPool(ctx, cluster.ReadReplicaPreferred)
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
utilities cover key colocation, grouping by shard, and bounded parallel operations across shards.

## Examples

See the [examples](examples) directory for runnable examples covering the main `xpg` usage patterns.

## License

This project is licensed under the [MIT License](LICENSE).
