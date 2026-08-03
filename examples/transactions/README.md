# Transactions and savepoints

This example creates an order in a transaction and applies an optional promo code inside a PostgreSQL savepoint.
The promo code is already used, so only the savepoint is rolled back while the outer transaction commits the order.

**This example demonstrates:**

* Creating an `xpg.Pool` and explicitly checking PostgreSQL connectivity
* Executing a callback with an explicit `pgx.Tx`
* Isolating an optional operation with `xpg.InSavepoint`
* Inspecting a wrapped `pgconn.PgError`
* Continuing and committing the outer transaction after a savepoint rollback

## Configuration

The example connects to PostgreSQL using:

```text
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

Set `XPG_DATABASE_URL` to use another connection string:

```shell
export XPG_DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable'
```

## Local PostgreSQL setup

The example can use the local PostgreSQL setup from `examples/docker-compose.yml`.

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile tools up -d
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile tools up -d
```

To start only PostgreSQL, omit `--profile tools`.

PostgreSQL is available to applications running on the host at:

```text
localhost:5432
```

Adminer is available at:

```text
http://localhost:8080
```

Sign in to Adminer with:

```text
System:   PostgreSQL
Server:   postgres
Username: postgres
Password: postgres
Database: postgres
```

> [!IMPORTANT]
> Use `postgres`, not `localhost`, in the **Server** field. Adminer connects to PostgreSQL through the Docker Compose
> network, where the database is discoverable by its service name.

## Run

From this example directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/transactions
```

## Expected output

```text
order ID: 1
order status: new
promo code: PROMO2026
promo applied: false
```

The order is committed because the unique-key error is confined to the savepoint. `InSavepoint` rolls the failed promo
insert back before the outer transaction decides that this specific error is non-fatal.

## Inspect the result

The embedded `setup.sql` recreates the `xpg_transactions_example` schema before each run and leaves the resulting data
available for inspection.

In Adminer, open the `xpg_transactions_example` schema and inspect:

```text
orders
promo_redemptions
```

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile tools down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose -f ../docker-compose.yml --profile tools down --remove-orphans -v
```