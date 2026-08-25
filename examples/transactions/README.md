# Transactions and savepoints

This example shows how to keep an outer transaction commit-able when an optional operation fails:

* Execute the main operation inside a transaction
* Isolate optional work with a savepoint
* Detect an expected PostgreSQL constraint violation
* Roll back only the savepoint while allowing the outer transaction to commit

The example attempts to redeem `PROMO2026`, which is already in use. The promo write fails and is rolled back to the
savepoint, while the order is committed successfully.

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

## Run

From this directory:

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

The failed promo insert is rolled back to the savepoint. The outer transaction then returns `nil`, so the order is
committed.

## Cleanup

To remove the example schema and data:

```shell
psql 'postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_transactions_example CASCADE;'
```

Stop the local services:

```shell
docker compose down
```

To also remove the PostgreSQL data volume:

```shell
docker compose down -v
```
