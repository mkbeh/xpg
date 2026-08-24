# Observability Example

This example shows how to combine logging, distributed tracing, and connection pool metrics with `xpg`.

**This example demonstrates:**

* Adapting the standard library `log/slog` logger to `pgx/tracelog`
* Attaching an OpenTelemetry PostgreSQL tracer through `xpg.WithTracer`
* Combining the logger and tracer automatically through the `xpg` tracing pipeline
* Exporting `xpg` pool metrics through OpenTelemetry and Prometheus
* Propagating an application span through concurrent PostgreSQL operations

The signals remain independent at the application boundary:

```text
slog                  otelpgx                 otelxpg
  |                      |                       |
  +---- pgx tracing -----+                  OTel metrics
             |                                   |
            xpg                              Prometheus
                                                 |
                                             /metrics
```

`otelpgx.RecordStats` is intentionally not used here. PostgreSQL tracing is handled by `otelpgx`, while pool metrics are
owned by `xpg` and exported through `extra/otelxpg`.

## Configuration

The example uses the following connection string by default:

```text
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

Set `XPG_DATABASE_URL` to use another PostgreSQL instance:

```shell
export XPG_DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable'
```

The HTTP server listens on `localhost:9464` by default. Override it with:

```shell
export HTTP_ADDR='localhost:9464'
```

## Local setup

Start PostgreSQL and Adminer from the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile tools up -d
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile tools up -d
```

Services are available at:

```text
PostgreSQL: localhost:5432
Adminer:    http://localhost:8080
```

Sign in to Adminer with:

```text
System:   PostgreSQL
Server:   postgres
Username: postgres
Password: postgres
Database: postgres
```

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/observability
```

The example starts an HTTP server at:

```text
http://localhost:9464
```

## Logging

The example adapts `log/slog` to `tracelog.Logger` and passes it to the pool:

```go
xpg.WithLogger(
    newPGXLogger(logger),
    tracelog.LogLevelInfo,
)
```

The adapter also copies the active OpenTelemetry `trace_id` and `span_id` into pgx log records when a span is present,
which makes logs and database spans directly correlatable.

`pgx/tracelog` can include SQL text and query arguments in log records. Production applications should choose logging
levels and redaction policies appropriate for the data they process.

## Tracing

`otelpgx` is attached as a normal pgx tracer:

```go
xpg.WithTracer(
    otelpgx.NewTracer(
        otelpgx.WithTracerProvider(tracing.TracerProvider()),
    ),
)
```

The example writes completed spans to stdout as formatted JSON. The stdout exporter and synchronous span processor are
used only to make the example self-contained and immediately observable. Production applications should normally export
traces through OTLP and use a batch span processor.

Generate a traced workload:

```shell
curl -X POST 'http://localhost:9464/load'
```

The handler creates one application span and passes its context to six concurrent PostgreSQL operations. Database spans
created by `otelpgx` therefore appear as children of that application span.

## Metrics

Pool metrics use an explicit OpenTelemetry `MeterProvider` backed by the Prometheus exporter:

```go
xpg.WithMetrics(
    otelxpg.NewMetrics(
        otelxpg.WithMeterProvider(metrics.MeterProvider()),
    ),
)
```

Open the Prometheus endpoint:

```shell
curl 'http://localhost:9464/metrics'
```

Show only database connection pool and `xpg` metrics:

```shell
curl -s 'http://localhost:9464/metrics' \
  | grep -E '^(db_client_connection|xpg_pool_connection_)'
```

The example exports these metric families:

```text
db_client_connection_count
db_client_connection_max
xpg_pool_connection_constructing
xpg_pool_connection_acquire_count_total
xpg_pool_connection_acquire_time_seconds_total
xpg_pool_connection_acquire_canceled_count_total
xpg_pool_connection_acquire_empty_count_total
xpg_pool_connection_acquire_empty_wait_time_seconds_total
xpg_pool_connection_create_count_total
xpg_pool_connection_destroy_count_total
```

The Prometheus exporter converts OpenTelemetry dotted instrument names to Prometheus-compatible names and adds unit and
counter suffixes where required.

## Generate pool contention

The pool is intentionally limited to two connections. Run:

```shell
curl -X POST 'http://localhost:9464/load'
```

While it is running, inspect metrics from another terminal:

```shell
curl -s 'http://localhost:9464/metrics' \
  | grep -E 'db_client_connection_count|xpg_pool_connection_acquire_'
```

Six concurrent one-second queries make connection usage and acquire wait metrics visible while also producing related
logs and spans.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile tools down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile tools down --remove-orphans -v
```
