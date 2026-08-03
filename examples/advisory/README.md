# Transaction advisory locks

This example coordinates concurrent workers with PostgreSQL transaction-level advisory locks. The first worker acquires
an advisory lock and holds it until its transaction commits. A second worker uses the non-blocking try variant and
cannot
enter the protected section while the lock is held. After the first transaction commits, a third worker acquires the
same
lock successfully.

**This example demonstrates:**

* Acquiring a transaction-level lock with `xpg.AdvisoryXactLock`
* Trying to acquire a lock without waiting with `xpg.TryAdvisoryXactLock`
* Holding a lock for the lifetime of an explicit `pgx.Tx`
* Releasing a transaction-level lock automatically on commit or rollback
* Coordinating concurrent database work without `pg_sleep`

## Configuration

The example uses the following connection string by default:

```text
postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable
```

Set `XPG_DATABASE_URL` to use another PostgreSQL instance:

```shell
export XPG_DATABASE_URL='postgres://user:password@localhost:5432/database?sslmode=disable'
```

> [!NOTE]
> The example runs two transactions concurrently, so the pool must allow at least two connections. The default pgxpool
> configuration satisfies this requirement.

## Local PostgreSQL setup

From the repository root, start PostgreSQL and Adminer:

```shell
docker compose -f examples/docker-compose.yml --profile tools up -d
```

Or from this directory:

```shell
docker compose -f ../docker-compose.yml --profile tools up -d
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
> Use `postgres`, not `localhost`, in the **Server** field. Adminer connects through the Docker Compose network, where
> PostgreSQL is available by its service name.

## Run

From this directory:

```shell
go run .
```

Or from the repository root:

```shell
go run ./examples/advisory
```

## Flow

This example is easier to follow as a sequence:

```text
1. Reset the example state
2. Worker A acquires the advisory lock
3. Worker B tries the same lock without waiting
4. Worker A commits and releases the lock
5. Worker C acquires the released lock
6. Read the committed job runs
```

## Expected output

```text
worker-a acquired the lock
worker-b acquired the lock: false
worker-a committed and released the lock
worker-c acquired the lock: true
recorded job runs:
- worker-a (lock key: 2026)
- worker-c (lock key: 2026)
```

> [!IMPORTANT]
> Advisory lock keys are application-defined `int64` values. Use a stable key mapping and keep the protected transaction
> short because it holds both the lock and a pool connection until commit or rollback.

## Stop services

From the repository root:

```shell
docker compose -f examples/docker-compose.yml --profile tools down --remove-orphans -v
```

Or from this directory:

```shell
docker compose -f ../docker-compose.yml --profile tools down --remove-orphans -v
```