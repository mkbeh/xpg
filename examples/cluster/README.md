# Cluster routing

This example routes reads and transactions across one primary pool and two replica pools with `cluster.Cluster`.

```text
                 ┌─ primary
application ─ cluster
                 ├─ replica-one
                 └─ replica-two
```

**This example demonstrates:**

* Creating a cluster from primary and replica pools
* Routing reads to the primary or replicas
* Distributing replica reads with round-robin
* Running primary and read-only replica transactions

> [!NOTE]
> The local containers are independent PostgreSQL instances used to demonstrate routing. They do not configure streaming
replication.

## Configuration

The example uses the following connection strings by default:

```text
XPG_PRIMARY_DATABASE_URL=postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable&target_session_attrs=read-write
XPG_REPLICA_ONE_DATABASE_URL=postgres://postgres:postgres@localhost:55433/postgres?sslmode=disable&target_session_attrs=read-only
XPG_REPLICA_TWO_DATABASE_URL=postgres://postgres:postgres@localhost:55434/postgres?sslmode=disable&target_session_attrs=read-only
```

Set the corresponding environment variables to use different PostgreSQL endpoints:

```shell
export XPG_PRIMARY_DATABASE_URL='postgres://user:password@primary.example.com:5432/database?sslmode=disable&target_session_attrs=read-write'
export XPG_REPLICA_ONE_DATABASE_URL='postgres://user:password@replica-one.example.com:5432/database?sslmode=disable&target_session_attrs=read-only'
export XPG_REPLICA_TWO_DATABASE_URL='postgres://user:password@replica-two.example.com:5432/database?sslmode=disable&target_session_attrs=read-only'
```

## Local setup

Start the primary, both replica endpoints, and Adminer from the repository root:

```shell
docker compose -f examples/cluster/docker-compose.yml --profile tools up -d
```

Or from this example directory:

```shell
docker compose --profile tools up -d
```

Services are available at:

```text
Primary:    localhost:55432
Replica 1:  localhost:55433
Replica 2:  localhost:55434
Adminer:    http://localhost:58080
```

Sign in to Adminer with:

```text
System:   PostgreSQL
Server:   postgres-primary
Username: postgres
Password: postgres
Database: postgres
```

Use `postgres-replica-one` or `postgres-replica-two` in the **Server** field to inspect the replica endpoints.

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/basic
```

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/cluster
```

## Expected output

```text
primary:
- pool=cluster.primary node=primary role=primary
replica reads:
- pool=cluster.replica-one node=replica-one role=replica
- pool=cluster.replica-two node=replica-two role=replica
transactions:
- primary node=primary read_only=off
- replica node=replica-one read_only=on
```

The example performs one complete round-robin pass across the configured replicas before starting the read-only
transaction.

Pools remain owned by the caller until `cluster.New` succeeds. After successful cluster creation, `cluster.Cluster` owns
the pools and closes them when `Cluster.Close` is called.

## Stop services

From the repository root:

```shell
docker compose \
  -f examples/cluster/docker-compose.yml \
  --profile tools \
  down --remove-orphans -v
```

Or from this example directory:

```shell
docker compose --profile tools down --remove-orphans -v
```
