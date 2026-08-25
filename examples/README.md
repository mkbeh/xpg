# Examples

This directory contains runnable examples covering the main `xpg` usage patterns.

| Example                          | Covers                                                                      |
|:---------------------------------|:----------------------------------------------------------------------------|
| [`basic`](basic)                 | Core `xpg.Pool` usage for common PostgreSQL operations                      |
| [`transactions`](transactions)   | Transactions, savepoints, and recovering from an optional operation failure |
| [`advisory`](advisory)           | Coordinating concurrent work with transaction-level advisory locks          |
| [`observability`](observability) | `slog` logging, OpenTelemetry tracing, and Prometheus pool metrics          |
| [`cluster`](cluster)             | Primary and replica routing, round-robin reads, and read-only transactions  |
| [`shard`](shard)                 | Range-based shard routing and grouping keys by shard                        |
| [`shard_geo`](shard_geo)         | Custom geographic routing built from shard metadata                         |

## Running the examples

Each example is self-contained and includes its own setup and run instructions.

Open the corresponding directory and follow its README.
