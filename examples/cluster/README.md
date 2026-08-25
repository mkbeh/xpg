# Cluster routing

This example shows how `cluster.Cluster` coordinates primary and replica access:

* Route reads explicitly to the primary or replicas
* Distribute replica reads with round-robin selection
* Run write transactions on the primary
* Run read-only transactions on replicas

> [!NOTE]
> The local services are independent PostgreSQL instances used only to demonstrate routing. They do not configure
> streaming replication.

## Local setup

From this directory, start the three PostgreSQL nodes and Adminer:

```shell
docker compose up -d
```

Apply the node-specific setup:

```shell
psql 'postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable' \
  < sql/primary.sql

psql 'postgres://postgres:postgres@localhost:55433/postgres?sslmode=disable' \
  < sql/replica-one.sql

psql 'postgres://postgres:postgres@localhost:55434/postgres?sslmode=disable' \
  < sql/replica-two.sql
```

The services are available at:

```text
Primary:   localhost:55432
Replica 1: localhost:55433
Replica 2: localhost:55434
Adminer:   http://localhost:8080
```

To inspect the nodes in Adminer, sign in with:

```text
System:   PostgreSQL
Server:   postgres-primary
Username: postgres
Password: postgres
Database: postgres
```

Use `postgres-replica-one` or `postgres-replica-two` in the **Server** field to inspect a replica.

## Configuration

By default, the example connects to:

```text
Primary:   postgres://postgres:postgres@localhost:55432/postgres?sslmode=disable&target_session_attrs=read-write
Replica 1: postgres://postgres:postgres@localhost:55433/postgres?sslmode=disable&target_session_attrs=read-only
Replica 2: postgres://postgres:postgres@localhost:55434/postgres?sslmode=disable&target_session_attrs=read-only
```

To use other PostgreSQL endpoints, set `XPG_PRIMARY_DATABASE_URL`, `XPG_REPLICA_ONE_DATABASE_URL`, and
`XPG_REPLICA_TWO_DATABASE_URL`.

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
primary read:
- pool=cluster.primary node=primary role=primary
replica reads:
- pool=cluster.replica-one node=replica-one role=replica
- pool=cluster.replica-two node=replica-two role=replica
transactions:
- primary node=primary read_only=off
- replica node=replica-one read_only=on
```

The two replica reads show one complete round-robin cycle. The following read transaction continues from the same
selector and therefore resolves the first replica again.

## Cleanup

Stop the local services:

```shell
docker compose down
```

To also remove all PostgreSQL data volumes:

```shell
docker compose down -v
```
