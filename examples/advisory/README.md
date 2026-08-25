# Transaction advisory locks

This example shows how transaction-level advisory locks protect a shared operation across concurrent workers:

* Acquire a lock for the lifetime of a transaction
* Check the same lock without blocking
* Release the lock automatically when the transaction completes

Worker A holds the lock while its transaction is open. Worker B cannot enter the protected section, and worker C
acquires the lock after worker A commits.

## Local setup

From this directory, start PostgreSQL and Adminer:

```shell
docker compose up -d
```

Apply the example schema:

```shell
psql 'postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  < sql/schema.sql
```

The services are available at:

```text
PostgreSQL: localhost:5432
Adminer:    http://localhost:8080
```

To inspect the example data in Adminer, sign in with:

```text
System:   PostgreSQL
Server:   postgres
Username: postgres
Password: postgres
Database: postgres
```

## Configuration

By default, the example connects to:

```text
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

To use another PostgreSQL instance, set `XPG_DATABASE_URL`:

```shell
export XPG_DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable'
```

The target database must contain the schema from `sql/schema.sql`.

> [!NOTE]
> The example runs two transactions concurrently, so the pool must allow at least two connections.

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/advisory
```

## Expected output

```text
worker-a acquired the lock
worker-b acquired the lock: false
worker-a committed and released the lock
worker-c acquired the lock: true
recorded job runs:
- worker-a (lock key: 2026)
- worker-c (lock key: 2026)
```

Transaction-level advisory locks are released automatically on commit or rollback. Keep the protected transaction short
because it holds both the lock and a pool connection.

## Cleanup

To remove the example schema and data:

```shell
psql 'postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_advisory_example CASCADE;'
```

Stop the local services:

```shell
docker compose down
```

To also remove the PostgreSQL data volume:

```shell
docker compose down -v
```
