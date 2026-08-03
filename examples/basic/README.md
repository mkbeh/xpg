# Basic pool usage

This example opens an `xpg.Pool`, checks PostgreSQL connectivity, writes two rows, and reads them back with the common
query methods exposed by the pool.

**This example demonstrates:**

* Creating and closing an `xpg.Pool`
* Explicitly checking connectivity with `Pool.Ping`
* Executing a statement with `Pool.Exec`
* Reading one row with `Pool.QueryRow`
* Iterating over rows returned by `Pool.Query`
* Using `Pool.Name` as logical pool metadata

## Configuration

The example uses the following connection string by default:

```text
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

Set `XPG_DATABASE_URL` to use another PostgreSQL instance:

```shell
export XPG_DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable'
```

## Local PostgreSQL setup

From the repository root, start PostgreSQL and Adminer:

```shell
docker compose -f examples/docker-compose.yml --profile tools up -d
```

Or from this directory:

```shell
docker compose -f ../docker-compose.yml --profile tools up -d
```

Adminer is available at <http://localhost:8080>. Sign in with:

```text
System:   PostgreSQL
Server:   postgres
Username: postgres
Password: postgres
Database: postgres
```

> [!IMPORTANT]
> Use `postgres`, not `localhost`, in the **Server** field. Adminer connects through the Docker Compose network, where
> PostgreSQL is available by its service name.

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
inserted users: 2
selected user: 1 Alice <alice@example.com> active=true
active users:
- 1 Alice <alice@example.com>
- 2 Bob <bob@example.com>
```

The embedded `setup.sql` file recreates the `xpg_basic_example` schema before each run. The resulting data remains in
PostgreSQL so it can be inspected in Adminer.

## Stop services

```shell
docker compose -f examples/docker-compose.yml down
```

Add `-v` to remove the PostgreSQL volume as well.
