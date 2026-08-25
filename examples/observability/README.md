# Observability

This example shows how to add observability to `xpg`:

* Log PostgreSQL activity with `slog`
* Trace database operations with OpenTelemetry
* Export connection-pool metrics to Prometheus
* Generate concurrent load to observe pool behavior

## Configuration

By default, the example connects to:

```text
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

To use another PostgreSQL instance, set `XPG_DATABASE_URL`:

```shell
export XPG_DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable'
```

The HTTP server listens on `localhost:9464`. To use another address, set `HTTP_ADDR`:

```shell
export HTTP_ADDR='localhost:9464'
```

## Local setup

From this directory, start PostgreSQL and Adminer:

```shell
docker compose up -d
```

The services are available at:

```text
PostgreSQL: localhost:5432
Adminer:    http://localhost:8080
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

The HTTP server starts on:

```shell
localhost:9464
```

## Generate load

Run six concurrent one-second queries against a pool limited to two connections:

```shell
curl -X POST 'http://localhost:9464/load'
```

While the request is running, inspect pool contention from another terminal:

```shell
curl -s 'http://localhost:9464/metrics' \
  | grep -E 'db_client_connection_count|xpg_pool_connection_acquire_'
```

To inspect all exported metrics:

```shell
curl 'http://localhost:9464/metrics'
```

The request produces pgx logs and OpenTelemetry spans in the application output, while pool metrics remain available
from the `/metrics` endpoint.

## Cleanup

Stop the local services:

```shell
docker compose down
```

To also remove the PostgreSQL data volume:

```shell
docker compose down -v
```