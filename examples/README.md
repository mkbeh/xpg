# Examples

This directory contains runnable examples demonstrating the main features and usage patterns of `xpg`.

| Example                        | Demonstrates                                                                              |
|:-------------------------------|:------------------------------------------------------------------------------------------|
| [`basic`](basic)               | Pool lifecycle and common query methods                                                   |
| [`transactions`](transactions) | Committing an outer transaction after an optional operation is rolled back to a savepoint |

## Running the examples

The examples use Docker Compose to start PostgreSQL and any required supporting services.

From the `examples` directory, start PostgreSQL and Adminer:

```shell
docker compose --profile tools up -d
```

Then run the example from its directory:

```shell
cd transactions
go run .
```

> [!NOTE]
> Some examples may require different services or configuration. Refer to the README in the corresponding example
> directory for the exact startup command, connection settings, and expected output.
