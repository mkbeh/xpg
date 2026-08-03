package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg"
)

const (
	defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"
	jobLockKey         = int64(2026)
)

//go:embed setup.sql
var setupSQL string

type jobRun struct {
	Worker  string
	LockKey int64
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	pool, err := xpg.Open(
		ctx,
		databaseURL(),
		xpg.WithName("advisory-example"),
	)
	if err != nil {
		return fmt.Errorf("create pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	if err := prepareExample(ctx, pool); err != nil {
		return fmt.Errorf("prepare example: %w", err)
	}

	lockAcquired := make(chan struct{})
	releaseLock := make(chan struct{})
	holderDone := make(chan error, 1)

	go func() {
		holderDone <- holdJobLock(
			ctx,
			pool,
			"worker-a",
			jobLockKey,
			lockAcquired,
			releaseLock,
		)
	}()

	select {
	case <-lockAcquired:
		fmt.Println("worker-a acquired the lock")
	case err := <-holderDone:
		if err == nil {
			return errors.New("worker-a exited before acquiring the lock")
		}

		return fmt.Errorf("worker-a: %w", err)
	case <-ctx.Done():
		return ctx.Err()
	}

	workerBAcquired, workerBErr := tryRunJob(
		ctx,
		pool,
		"worker-b",
		jobLockKey,
	)
	fmt.Printf("worker-b acquired the lock: %t\n", workerBAcquired)

	close(releaseLock)
	holderErr := <-holderDone

	if workerBErr != nil {
		return fmt.Errorf("worker-b: %w", workerBErr)
	}

	if holderErr != nil {
		return fmt.Errorf("worker-a: %w", holderErr)
	}

	if workerBAcquired {
		return errors.New("worker-b acquired a lock that should still be held")
	}

	fmt.Println("worker-a committed and released the lock")

	workerCAcquired, err := tryRunJob(
		ctx,
		pool,
		"worker-c",
		jobLockKey,
	)
	if err != nil {
		return fmt.Errorf("worker-c: %w", err)
	}

	if !workerCAcquired {
		return errors.New("worker-c did not acquire the released lock")
	}

	fmt.Printf("worker-c acquired the lock: %t\n", workerCAcquired)

	runs, err := loadJobRuns(ctx, pool)
	if err != nil {
		return fmt.Errorf("load job runs: %w", err)
	}

	fmt.Println("recorded job runs:")
	for _, current := range runs {
		fmt.Printf("- %s (lock key: %d)\n", current.Worker, current.LockKey)
	}

	return nil
}

func prepareExample(
	ctx context.Context,
	pool *xpg.Pool,
) error {
	_, err := pool.Exec(
		ctx,
		setupSQL,
		pgx.QueryExecModeSimpleProtocol,
	)
	if err != nil {
		return fmt.Errorf("execute setup SQL: %w", err)
	}

	return nil
}

func holdJobLock(
	ctx context.Context,
	pool *xpg.Pool,
	worker string,
	lockKey int64,
	lockAcquired chan<- struct{},
	releaseLock <-chan struct{},
) error {
	return pool.InTx(
		ctx,
		pgx.TxOptions{},
		func(ctx context.Context, tx pgx.Tx) error {
			if err := xpg.AdvisoryXactLock(ctx, tx, lockKey); err != nil {
				return err
			}

			if err := recordJobRun(ctx, tx, worker, lockKey); err != nil {
				return fmt.Errorf("record job run: %w", err)
			}

			close(lockAcquired)

			select {
			case <-releaseLock:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	)
}

func tryRunJob(
	ctx context.Context,
	pool *xpg.Pool,
	worker string,
	lockKey int64,
) (bool, error) {
	var acquired bool

	err := pool.InTx(
		ctx,
		pgx.TxOptions{},
		func(ctx context.Context, tx pgx.Tx) error {
			var err error

			acquired, err = xpg.TryAdvisoryXactLock(ctx, tx, lockKey)
			if err != nil {
				return err
			}

			if !acquired {
				return nil
			}

			if err := recordJobRun(ctx, tx, worker, lockKey); err != nil {
				return fmt.Errorf("record job run: %w", err)
			}

			return nil
		},
	)
	if err != nil {
		return false, err
	}

	return acquired, nil
}

func recordJobRun(
	ctx context.Context,
	tx pgx.Tx,
	worker string,
	lockKey int64,
) error {
	_, err := tx.Exec(
		ctx,
		`INSERT INTO xpg_advisory_example.job_runs (worker, lock_key)
		VALUES ($1, $2)`,
		worker,
		lockKey,
	)

	return err
}

func loadJobRuns(
	ctx context.Context,
	pool *xpg.Pool,
) ([]jobRun, error) {
	rows, err := pool.Query(
		ctx,
		`SELECT worker, lock_key
		 FROM xpg_advisory_example.job_runs
		 ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	runs := make([]jobRun, 0, 2)

	for rows.Next() {
		var current jobRun

		if err := rows.Scan(
			&current.Worker,
			&current.LockKey,
		); err != nil {
			return nil, err
		}

		runs = append(runs, current)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return runs, nil
}

func databaseURL() string {
	if value := os.Getenv("XPG_DATABASE_URL"); value != "" {
		return value
	}

	return defaultDatabaseURL
}
