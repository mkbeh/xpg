# Changelog

All notable changes to this project will be documented in this file.

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