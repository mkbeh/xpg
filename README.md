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

* **Pool Management:** Thin pool management over `pgx` with direct access to the underlying client.
* **Transactions and Savepoints:** Managed transactions and savepoints for multi-step workflows.
* **Advisory Locking:** Transaction-level advisory locks for coordinating concurrent database operations.
* **Error Classification:** Semantic helpers for PostgreSQL constraints, transaction failures, cancellation, connection
  errors, and SQLSTATE inspection.
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

Open an `xpg` pool and execute PostgreSQL queries using the familiar `pgx` query API.

<!-- @formatter:off -->

```go
pool, err := xpg.Open(
    ctx,
    os.Getenv("DATABASE_URL"),
    xpg.WithName("example-pool"),
)
if err != nil {
    return fmt.Errorf("open pool: %w", err)
}
defer pool.Close()

var message string
err = pool.QueryRow(ctx, "SELECT 'hello from xpg'").Scan(&message)
if err != nil {
    return fmt.Errorf("query: %w", err)
}

fmt.Println(message) // hello from xpg
```

<!-- @formatter:on -->

### Managed Transactions

`xpg` manages the transaction lifecycle while preserving the native `pgx.Tx` API. Returning `nil` commits the
transaction; returning an error rolls it back.

<!-- @formatter:off -->

```go
err := pool.InTx(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
    _, err := tx.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", userID)
    return err
})
```

<!-- @formatter:on -->

Savepoints can isolate optional transactional work without aborting the outer transaction.

### Advisory Locks

Transaction-level PostgreSQL advisory locks can coordinate concurrent work across application instances using the same
database. The lock is held for the lifetime of the transaction and released automatically on commit or rollback.

<!-- @formatter:off -->

```go
err := pool.InTx(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
    // Acquire a transaction-scoped advisory lock before updating the user.
    if err := xpg.AdvisoryXactLock(ctx, tx, lockID); err != nil {
        return err
    }

    _, err := tx.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", userID)
    return err
})
```

<!-- @formatter:on -->

### Error Handling

`xpg` provides semantic helpers for classifying PostgreSQL errors and inspecting SQLSTATE codes without manual string
matching.

<!-- @formatter:off -->

```go
_, err := pool.Exec(ctx, "INSERT INTO users (email) VALUES ($1)", email)

switch {
case xpg.IsUniqueViolation(err):
    // Handle duplicate data.
case xpg.IsRetryableTransaction(err):
    // Retry when the operation is safe to replay.
case err != nil:
    return err
}
```

<!-- @formatter:on -->

The underlying SQLSTATE code is available through `xpg.SQLState(err)`. Built-in helpers cover constraint violations,
serialization failures, deadlocks, lock errors, query cancellation, and connection failures.

## Clustering

The `topology/cluster` package groups primary and replica pools into a logical cluster with explicit read routing.

<!-- @formatter:off -->

```go
users, err := cluster.New(cluster.Config{
    ID:       "users",
    Primary:  pool1,
    Replicas: []*xpg.Pool{pool2, pool3},
})
if err != nil {
    return fmt.Errorf("initialize cluster: %w", err)
}
defer users.Close()

// Route writes explicitly to the primary.
writer := users.Primary()

_, err = writer.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", userID)
if err != nil {
    return err
}

// Route reads according to the selected policy.
reader, err := users.ReadPool(ctx, cluster.ReadReplicaPreferred)
if err != nil {
    return err
}

var active bool
err = reader.QueryRow(ctx, "SELECT active FROM users WHERE id = $1", userID).Scan(&active)
if err != nil {
    return err
}
```

<!-- @formatter:on -->

### Cluster Transactions

Cluster transactions combine explicit primary/replica routing with the native `pgx` transaction API.

**Primary Transactions**

`InPrimaryTx` runs the transaction on the cluster primary and is intended for atomic writes. The transaction commits on
`nil` and rolls back on error.

<!-- @formatter:off -->

```go
err := users.InPrimaryTx(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
    _, err := tx.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", userID)
    return err
})
```

<!-- @formatter:on -->

**Read Transactions**

`InReadTx` routes the transaction according to the selected read policy and enforces PostgreSQL read-only mode. Use it
for read workloads that can run on replicas.

<!-- @formatter:off -->

```go
err := users.InReadTx(
    ctx,
    cluster.ReadReplicaPreferred,
    cluster.ReadTxOptions{
        IsoLevel: pgx.RepeatableRead,
    },
    func(ctx context.Context, tx pgx.Tx) error {
        return tx.QueryRow(ctx, "SELECT active FROM users WHERE id = $1", userID).Scan(&active)
    },
)
```

<!-- @formatter:on -->

### Read Routing Policies

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
topology, err := shard.NewTopology(cluster1, cluster2)
if err != nil {
    panic(err)
}
defer topology.Close()

// Route user IDs using rendezvous hashing.
userResolver, err := resolver.NewRendezvous(topology, "users", resolver.Uint64KeyEncoder())
if err != nil {
    panic(err)
}

// Resolve the target shard.
targetShard, err := userResolver.Resolve(userID)
if err != nil {
    panic(err)
}

// Route writes to the shard primary.
writer := targetShard.Primary()

_, err = writer.Exec(ctx, "UPDATE users SET active = true WHERE id = $1", userID)
if err != nil {
    panic(err)
}

// Route reads according to the selected policy.
reader, err := targetShard.ReadPool(ctx, cluster.ReadReplicaPreferred)
if err != nil {
    panic(err)
}

var active bool
err = reader.QueryRow(ctx, "SELECT active FROM users WHERE id = $1", userID).Scan(&active)
if err != nil {
    panic(err)
}
```

<!-- @formatter:on -->

### Routing Strategies

Resolvers bind a data-placement strategy to an immutable shard topology. Every resolver exposes the same routing
contract, allowing application code to resolve keys independently of the selected strategy.

| Resolver             | Best Suited For                       | Routing Model                                                         |
| :------------------- | :------------------------------------ | :-------------------------------------------------------------------- |
| `RendezvousResolver` | Keys without natural ranges           | Deterministic Highest Random Weight (HRW) hashing within a namespace. |
| `RangeResolver`      | Ordered numeric or string keys        | Bounded, non-overlapping half-open intervals `[Start, End)`.          |
| `TimeRangeResolver`  | Time-series or partitioned event data | Bounded chronological intervals normalized to UTC.                    |
| `CustomResolver`     | Domain-specific placement rules       | Application-defined mapping from a key to `shard.ID`.                 |

<!-- @formatter:off -->

```go
// Rendezvous hashing distributes arbitrary keys deterministically across the topology.
usersByHash, _ := resolver.NewRendezvous(topology, "users", resolver.Uint64KeyEncoder())

// Ordered ranges provide explicit control over the keyspace.
usersByRange, _ := resolver.NewRange(topology, []resolver.Range[uint64]{
    {Start: 0,   End: 100, ShardID: "shard-a"},
    {Start: 100, End: 200, ShardID: "shard-b"},
})

// Time ranges route records through bounded chronological intervals.
t0, _ := time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")
t1 := t0.AddDate(0, 1, 0)
t2 := t0.AddDate(0, 2, 0)

eventsByTime, _ := resolver.NewTimeRange(topology, []resolver.TimeRange{
    {Start: t0, End: t1, ShardID: "shard-a"},
    {Start: t1, End: t2, ShardID: "shard-b"},
})

// Custom routing keeps domain-specific placement rules in application code.
tenantsByRegion, _ := resolver.NewCustom(topology, func(region string) (shard.ID, error) {
    switch region {
    case "eu":
        return "shard-a", nil
    case "us":
        return "shard-b", nil
    default:
        return "", shard.ErrNoShard
    }
})
```

<!-- @formatter:on -->

Regardless of the selected strategy, routing uses the same `Resolve` contract:

<!-- @formatter:off -->

```go
targetShard, err := usersByHash.Resolve(userID)
if err != nil {
    panic(err)
}

log.Printf("resolved shard: %s", targetShard.ID())
```

<!-- @formatter:on -->

Range and time-range resolvers may contain intentional gaps in the configured keyspace; keys that do not match any range
return `ErrNoShard`. Custom resolvers can return the same error when a domain key has no valid destination.

> [!IMPORTANT]
> For rendezvous routing, the namespace, key encoding, and stable shard IDs are part of the placement contract.

### Multi-Key Routing

For complex batch operations, `xpg` provides routing primitives to analyze, group, and partition multi-key workloads
across a shard topology.

<!-- @formatter:off -->

```go
// Add range-based routing over the same shard topology.
rangeResolver, _ := resolver.NewRange(topology, []resolver.Range[uint64]{
    {Start: 0,   End: 100, ShardID: "shard-a"},
    {Start: 100, End: 200, ShardID: "shard-b"},
})
```

<!-- @formatter:on -->

**Strict Colocation**

`SameShard` verifies that all keys resolve to the same shard before a shard-local transaction or another operation that
must remain colocated.

<!-- @formatter:off -->

```go
// Verify that all keys resolve to the same shard.
targetShard, err := shard.SameShard(rangeResolver, 42, 43)
if err != nil {
    panic(err)
}

// Use the resolved shard for a shard-local operation.
log.Printf("resolved shard: %s", targetShard.ID())
```

<!-- @formatter:on -->

**Strict Batch Grouping**

`GroupByShard` groups keys by destination shard and fails if any key cannot be resolved.

<!-- @formatter:off -->

```go
keys := []uint64{42, 142, 43, 143}

groups, err := shard.GroupByShard(rangeResolver, keys)
if err != nil {
    panic(err)
}

for _, group := range groups {
    // Execute one shard-local batch update for each resolved group.
    _ = group.Shard.InPrimaryTx(ctx, pgx.TxOptions{}, func(ctx context.Context, tx pgx.Tx) error {
        _, err := tx.Exec(ctx, "UPDATE users SET active = true WHERE id = ANY($1)", group.Keys)
        return err
    })
}
```

<!-- @formatter:on -->

**Tolerant Partitioning**

`PartitionByShard` groups routable keys by shard while collecting keys that do not resolve to any shard separately.

<!-- @formatter:off -->

```go
keys := []uint64{42, 142, 250, 43, 143} // 250 falls outside the configured ranges.

partition, err := shard.PartitionByShard(rangeResolver, keys)
if err != nil {
    panic(err) // Resolver errors other than ErrNoShard still abort the operation.
}

// Process all routable groups.
for _, group := range partition.Groups {
    log.Printf("process shard=%s user_ids=%v", group.Shard.ID(), group.Keys)
}

// Handle unresolved keys separately.
if len(partition.Unresolved) != 0 {
    log.Printf("unresolved keys: %v", partition.Unresolved)
}
```

<!-- @formatter:on -->

### Parallel Fan-Out Operations

`ForEachShard` executes an operation across the entire topology with bounded concurrency. `maxConcurrency` controls how
many shard callbacks may run at the same time; setting it to `1` makes execution sequential.

<!-- @formatter:off -->

```go
const maxConcurrency = 4

timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
defer cancel()

cutoff := time.Now()

results, err := topology.ForEachShard(
    timeoutCtx,
    maxConcurrency,
    func(ctx context.Context, target shard.Shard) error {
        writer := target.Primary()
        if writer == nil {
            return cluster.ErrNoPrimary
        }

        _, err := writer.Exec(ctx, "DELETE FROM sessions WHERE expired_at < $1", cutoff)
        return err
    },
)
if err != nil {
    log.Printf("fan-out completed with errors: %v", err)
}

// Inspect individual shard failures when detailed handling is required.
for _, result := range results {
    if result.Err != nil {
        log.Printf("shard=%s failed: %v", result.ShardID, result.Err)
    }
}
```

<!-- @formatter:on -->

Results preserve topology registration order and retain individual shard failures, while the returned error aggregates
callback and context cancellation errors.

## Examples

See the [examples](examples) directory for runnable examples covering the main `xpg` usage patterns.

## License

This project is licensed under the [MIT License](LICENSE).
