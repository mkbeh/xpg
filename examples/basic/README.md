# Basic pool usage

This example shows how to use `xpg.Pool` for common PostgreSQL operations:

* Create a named PostgreSQL connection pool and verify connectivity
* Execute a write and read a single record back
* Query and iterate over multiple rows

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

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/basic
```

## Expected output

```text
pool: basic-example
upserted users: 2
selected user: 1 Alice <alice@example.com> active=true
active users:
- 1 Alice <alice@example.com>
```

## Cleanup

To remove the example schema and data:

```shell
psql 'postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_basic_example CASCADE;'
```

Stop the local services:

```shell
docker compose down
```

To also remove the PostgreSQL data volume:

```shell
docker compose down -v
```
