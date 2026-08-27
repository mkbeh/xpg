# Changelog

All notable changes to this project will be documented in this file.

## v0.4.0

This release expands multi-key sharding with tolerant partitioning and improves the documentation and examples for batch
routing across shard topologies.

### Added

* **Tolerant Shard Partitioning:** Added `PartitionByShard` and `Partition[K]` for grouping routable keys by shard while
  collecting keys that resolve to `ErrNoShard` separately. Other resolver errors continue to abort the operation.
* **Multi-Key Sharding Example:** Added the runnable `shard_group` example covering colocation checks, strict grouping,
  tolerant partitioning, and per-shard batch operations.

### Changed

* **Sharding Documentation:** Expanded the root README with resolver strategy guidance, multi-key routing patterns, and
  bounded parallel fan-out operations.

---

## v0.3.0

This release reorganizes the cluster and shard APIs under a common topology namespace and simplifies several sharding
contracts.

### Changed

* **Topology Package Layout:** Moved cluster and shard packages to `topology/cluster` and `topology/shard`, with shard
  resolvers under `topology/shard/resolver`.
* **Cluster Identity:** Cluster IDs are now required when creating a `cluster.Cluster`.
* **Shard Topology Construction:** Simplified topology creation from `shard.NewTopology([]shard.Config{...})` to
  `shard.NewTopology(clusters...)`.
* **Rendezvous Resolver:** Renamed `HashResolver` and `NewHash` to `RendezvousResolver` and `NewRendezvous`, making the
  routing algorithm explicit while preserving the existing rendezvous placement contract.
* **Custom Resolvers:** Simplified custom resolver callbacks to map a key directly to `shard.ID` without receiving the
  topology on every call.
* **Cross-Shard Operations:** `ForEachShard` now returns callback and cancellation failures through its function error
  while preserving detailed per-shard results.

### Fixed

* **Rendezvous Portability:** Fixed length validation in rendezvous routing so the resolver also compiles correctly on
  32-bit architectures.

### Removed

* **Shard Configuration Layer:** Removed `shard.Config`; shard topologies are now created directly from clusters.
* **Generic Hash API:** Removed the `HashResolver` and `NewHash` names in favor of the explicit rendezvous API.

---

## v0.2.0

Initial production release of `xpg`, built around `pgx` with PostgreSQL transaction helpers, primary/replica clustering,
application-level sharding, and observability integrations.

### Added

* **Pool Lifecycle Management:** Added a thin pool layer over `pgxpool` with pgx query operations, pool metadata,
  runtime statistics, configuration options, and direct access to the underlying pool.
* **Transactions and Savepoints:** Added managed transactions and savepoints for isolating optional work within a
  transaction.
* **Advisory Locks:** Added transaction-level PostgreSQL advisory locks for coordinating concurrent work.
* **PostgreSQL Error Classification:** Added SQLSTATE inspection and helpers for constraint violations, serialization
  failures, deadlocks, lock errors, query cancellation, and connection failures.
* **Primary/Replica Clustering:** Added logical clusters with explicit read policies, round-robin and custom replica
  selection, replica-preferred fallback when no replica can be selected, and enforced read-only transactions.
* **Application-Level Sharding:** Added immutable shard topologies with rendezvous hashing, ordered ranges, time ranges,
  and custom resolvers, together with colocation checks, grouping by shard, and bounded parallel operations.
* **Logging and Tracing:** Added pgx-compatible logging and tracing hooks with support for combining multiple query
  tracers.
* **Examples:** Added runnable examples covering pool usage, transactions, advisory locks, observability, clustering,
  range-based sharding, and custom geographic routing.

---

## extra/otelxpg/v0.1.0

Initial release of the `otelxpg` integration module.

### Added

* **OpenTelemetry Pool Metrics:** Added metrics for PostgreSQL connection-pool state, usage, acquisition behavior, and
  lifecycle counters.
* **Metric Attributes:** Added pool names and custom labels as metric attributes for filtering and aggregation across
  standalone pools, cluster nodes, and shard pools.
* **Meter Provider Integration:** Added support for an application-provided `metric.MeterProvider`.

---

## extra/slogxpg/v0.1.0

Initial release of the `slogxpg` integration module.

### Added

* **slog Adapter:** Added an adapter from the standard library `log/slog` logger to `pgx/tracelog.Logger`.
* **Log Level Mapping:** Added deterministic mapping from pgx trace log levels to `slog` levels.