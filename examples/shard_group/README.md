# Shard grouping

This example demonstrates multi-key operations across a sharded PostgreSQL topology:

* `SameShard` verifies that keys belong to the same shard before a shard-local transaction.
* `GroupByShard` splits a batch into per-shard groups and fails if any key cannot be resolved.
* `PartitionByShard` groups resolvable keys while returning unresolved keys separately.

The example uses two range-based shards:

```text
[0, 100)   -> shard-a
[100, 200) -> shard-b
```

Given the batch:

```text
[42, 142, 43, 143]
```

`GroupByShard` produces:

```text
shard-a -> [42, 43]
shard-b -> [142, 143]
```

Each group is then processed with a single SQL statement on its shard.

For tolerant reads, the batch may also contain keys outside the configured ranges:

```text
[42, 142, 250, 43, 143]
```

`PartitionByShard` returns:

```text
shard-a    -> [42, 43]
shard-b    -> [142, 143]
unresolved -> [250]
```

This allows the application to process routable keys while handling unresolved keys explicitly.

## Local setup

From this directory, start both PostgreSQL shards and Adminer:

```shell
docker compose up -d
```

Apply the example schema to both shards:

```shell
psql 'postgres://postgres:postgres@localhost:58431/postgres?sslmode=disable' \
  < sql/schema.sql

psql 'postgres://postgres:postgres@localhost:58432/postgres?sslmode=disable' \
  < sql/schema.sql
```

The services are available at:

```text
Shard A: localhost:58431
Shard B: localhost:58432
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
Shard A: postgres://postgres:postgres@localhost:58431/postgres?sslmode=disable
Shard B: postgres://postgres:postgres@localhost:58432/postgres?sslmode=disable
```

To use other PostgreSQL endpoints, set `XPG_SHARD_GROUP_A_DATABASE_URL` and `XPG_SHARD_GROUP_B_DATABASE_URL`.

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/shard_group
```

## Expected output

```text
batch upsert:
- shard=shard-a user_ids=[42 43]
- shard=shard-b user_ids=[142 143]

colocation:
- user_ids=[42 43] shard=shard-a transaction=committed
- user_ids=[42 142] mismatch=shard-a->shard-b

batch select:
- unresolved user_ids=[250]
- shard=shard-a user_ids=[42 43]
  user_id=42 name=alice active=true
  user_id=43 name=carol active=true
- shard=shard-b user_ids=[142 143]
  user_id=142 name=bob active=false
  user_id=143 name=dave active=false
```

## Cleanup

To remove the example schema and data:

```shell
psql 'postgres://postgres:postgres@localhost:58431/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_shard_group_example CASCADE;'

psql 'postgres://postgres:postgres@localhost:58432/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_shard_group_example CASCADE;'
```

Stop the local services:

```shell
docker compose down
```

To also remove both PostgreSQL data volumes:

```shell
docker compose down -v
```
