# Sharding

This example shows how to distribute application data across PostgreSQL shards:

* Route user IDs with a range-based shard resolver
* Write records to the resolved shard
* Group keys by shard for efficient batch processing

The example uses two primary-only shards:

```text
[0, 100)   -> shard-a
[100, 200) -> shard-b
```

## Local setup

From this directory, start both PostgreSQL shards and Adminer:

```shell
docker compose up -d
```

Apply the example schema to both shards:

```shell
psql 'postgres://postgres:postgres@localhost:56431/postgres?sslmode=disable' \
  < sql/schema.sql

psql 'postgres://postgres:postgres@localhost:56432/postgres?sslmode=disable' \
  < sql/schema.sql
```

The services are available at:

```text
Shard A: localhost:56431
Shard B: localhost:56432
Adminer: http://localhost:8080
```

To inspect a shard in Adminer, sign in with:

```text
System:   PostgreSQL
Server:   postgres-shard-a
Username: postgres
Password: postgres
Database: postgres
```

Use `postgres-shard-b` in the **Server** field to inspect the second shard.

## Configuration

By default, the example connects to:

```text
Shard A: postgres://postgres:postgres@localhost:56431/postgres?sslmode=disable
Shard B: postgres://postgres:postgres@localhost:56432/postgres?sslmode=disable
```

To use other PostgreSQL endpoints, set `XPG_SHARD_A_DATABASE_URL` and `XPG_SHARD_B_DATABASE_URL`.

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/shard
```

## Expected output

```text
range routing:
- user_id=42 shard=shard-a pool=shard.shard-a.primary
- user_id=142 shard=shard-b pool=shard.shard-b.primary

grouping:
- shard=shard-b user_ids=[142 143]
- shard=shard-a user_ids=[42 43]
```

`GroupByShard` preserves the order in which shards first appear in the input and the relative order of keys within each
group.

## Cleanup

To remove the example schema and data:

```shell
psql 'postgres://postgres:postgres@localhost:56431/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_shard_example CASCADE;'

psql 'postgres://postgres:postgres@localhost:56432/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_shard_example CASCADE;'
```

Stop the local services:

```shell
docker compose down
```

To also remove both PostgreSQL data volumes:

```shell
docker compose down -v
```
