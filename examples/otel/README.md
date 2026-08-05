# OpenTelemetry Metrics Example

This example shows how to export `xpg` connection pool metrics through the OpenTelemetry Prometheus exporter.

**This example demonstrates:**

* Exporting `xpg` pool metrics with OpenTelemetry and Prometheus
* Registering pool metrics through the global `MeterProvider`
* Generating pool contention through the `/load` endpoint
* Shutting down the pool and metrics provider in the correct order

## Configuration

The example uses the following connection string by default:

```text
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

Set `XPG_DATABASE_URL` to use another PostgreSQL instance:

```shell
export XPG_DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable'
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
go run ./examples/otel
```

The HTTP server starts on:

```text
http://localhost:9464
```

## View metrics

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

## Generate load

Run the debug workload:

```shell
curl -X POST 'http://localhost:9464/load'
```

While it is running, inspect the pool metrics from another terminal:

```shell
curl -s 'http://localhost:9464/metrics' \
  | grep -E 'db_client_connection_count|xpg_pool_connection_acquire_'
```

The workload runs six concurrent queries against a pool limited to two connections, making connection usage and wait
metrics visible.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile tools down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile tools down --remove-orphans -v
```