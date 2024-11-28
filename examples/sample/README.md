# Description

This is a sample REST API service implemented using pgx as the connector to a PostgreSQL data store.

# Usage

Create a PostgreSQL database.

Configure the database connection with environment variables:

```text
POSTGRES_CLUSTER_HOST=127.0.0.1
POSTGRES_CLUSTER_PORT=5432
POSTGRES_DB=db
POSTGRES_PASSWORD=pass
POSTGRES_USER=user
```

Run main.go:

```
go run main.go
```

## Create tasks

```shell
curl '127.0.0.1:8080/create'
```

## Get tasks

```shell
curl '127.0.0.1:8080/get'
```

## Metrics

```shell
curl 'http://localhost:8080/metrics'
```