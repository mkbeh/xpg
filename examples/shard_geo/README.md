# Geographic sharding

This example shows how to route tenants by region across PostgreSQL shards:

* Associate each shard with a region
* Build a custom resolver from shard metadata
* Route tenant operations to the matching shard
* Handle unsupported regions with `shard.ErrNoShard`

The topology contains two primary-only shards:

```text
eu -> shard-eu
us -> shard-us
```

## Local setup

From this directory, start both PostgreSQL shards and Adminer:

```shell
docker compose up -d
```

Apply the example schema to both shards:

```shell
psql 'postgres://postgres:postgres@localhost:57431/postgres?sslmode=disable' \
  < sql/schema.sql

psql 'postgres://postgres:postgres@localhost:57432/postgres?sslmode=disable' \
  < sql/schema.sql
```

The services are available at:

```text
EU shard: localhost:57431
US shard: localhost:57432
Adminer:  http://localhost:8080
```

To inspect a shard in Adminer, sign in with:

```text
System:   PostgreSQL
Server:   postgres-shard-eu
Username: postgres
Password: postgres
Database: postgres
```

Use `postgres-shard-us` in the **Server** field to inspect the US shard.

## Configuration

By default, the example connects to:

```text
EU shard: postgres://postgres:postgres@localhost:57431/postgres?sslmode=disable
US shard: postgres://postgres:postgres@localhost:57432/postgres?sslmode=disable
```

To use other PostgreSQL endpoints, set `XPG_SHARD_EU_DATABASE_URL` and `XPG_SHARD_US_DATABASE_URL`.

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/shard_geo
```

## Expected output

```text
geo routing:
- tenant=tenant-42 region=eu shard=shard-eu name=Alice
- tenant=tenant-77 region=us shard=shard-us name=Bob
unsupported region: no shard
```

The region index is built once from immutable shard metadata. Each subsequent route is a local map lookup followed by
the normal `shard.Shard` execution API.

## Cleanup

To remove the example schema and data:

```shell
psql 'postgres://postgres:postgres@localhost:57431/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_shard_geo_example CASCADE;'

psql 'postgres://postgres:postgres@localhost:57432/postgres?sslmode=disable' \
  -c 'DROP SCHEMA IF EXISTS xpg_shard_geo_example CASCADE;'
```

Stop the local services:

```shell
docker compose down
```

To also remove both PostgreSQL data volumes:

```shell
docker compose down -v
```
