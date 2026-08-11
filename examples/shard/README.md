# Sharding

This example routes typed application keys across two logical PostgreSQL shards. Resolvers are bound to an immutable
`shard.Topology` and return a `shard.Shard`, which delegates database operations to its `cluster.Cluster`.

```text
                 Topology []Shard
                        │
application key ─── Resolver
                        │
                      Shard
                        │
                     Cluster
                        │
                primary / replicas
```

**This example demonstrates:**

* Building a topology from primary-only and primary/replica clusters
* Routing `uint64` keys through bounded numeric ranges
* Running shard-local primary and read-only replica transactions
* Checking key colocation and grouping keys by shard
* Reading reference-table copies across shards with bounded fan-out

> [!NOTE]
> The `shard-b` replica is an independent read-only PostgreSQL instance used to
> demonstrate routing. The local setup does not configure streaming replication.

## Configuration

The example uses the following connection strings by default:

```text
XPG_SHARD_A_PRIMARY_DATABASE_URL=postgres://postgres:postgres@localhost:56431/postgres?sslmode=disable&target_session_attrs=read-write
XPG_SHARD_B_PRIMARY_DATABASE_URL=postgres://postgres:postgres@localhost:56432/postgres?sslmode=disable&target_session_attrs=read-write
XPG_SHARD_B_REPLICA_DATABASE_URL=postgres://postgres:postgres@localhost:56433/postgres?sslmode=disable&target_session_attrs=read-only
```

Set the corresponding environment variables to use different PostgreSQL endpoints:

```shell
export XPG_SHARD_A_PRIMARY_DATABASE_URL='postgres://user:password@shard-a.example.com:5432/database?sslmode=disable&target_session_attrs=read-write'
export XPG_SHARD_B_PRIMARY_DATABASE_URL='postgres://user:password@shard-b-primary.example.com:5432/database?sslmode=disable&target_session_attrs=read-write'
export XPG_SHARD_B_REPLICA_DATABASE_URL='postgres://user:password@shard-b-replica.example.com:5432/database?sslmode=disable&target_session_attrs=read-only'
```

## Local setup

Start all PostgreSQL endpoints and Adminer from the repository root:

```shell
docker compose -f examples/shard/docker-compose.yml --profile tools up -d
```

Or from this example directory:

```shell
docker compose --profile tools up -d
```

Services are available at:

```text
Shard A primary:  localhost:56431
Shard B primary:  localhost:56432
Shard B replica:  localhost:56433
Adminer:          http://localhost:58082
```

Sign in to Adminer with:

```text
System:   PostgreSQL
Server:   postgres-shard-a-primary
Username: postgres
Password: postgres
Database: postgres
```

Use `postgres-shard-b-primary` or `postgres-shard-b-replica` in the **Server** field to inspect another shard endpoint.

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
range routing and primary transactions:
- user_id=42 shard=shard-a primary_pool=shard.shard-a.primary
- user_id=142 shard=shard-b primary_pool=shard.shard-b.primary

grouping and colocation:
- shard=shard-b user_ids=[142 143]
- shard=shard-a user_ids=[42 43]
- colocated shard=shard-a
- cross-shard SameShard returns ErrShardMismatch=true

replica routing and read-only transaction:
- user_id=142 shard=shard-b read_pool=shard.shard-b.replica read_node=shard-b-replica tx_node=shard-b-replica role=replica read_only=on

reference table copies:
- shard=shard-a countries=2
- shard=shard-b countries=2
```

## Stop services

From the repository root:

```shell
docker compose \
  -f examples/shard/docker-compose.yml \
  --profile tools \
  down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose --profile tools down --remove-orphans -v
```
