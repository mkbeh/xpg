package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg"
)

const defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	pool, err := xpg.Open(
		ctx,
		databaseURL(),
		xpg.WithName("transactions-example"),
	)
	if err != nil {
		return fmt.Errorf("open pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	const (
		orderID   = int64(1)
		promoCode = "PROMO2026"
	)

	if err := processOrder(ctx, pool, orderID, promoCode); err != nil {
		return fmt.Errorf("process order: %w", err)
	}

	status, promoApplied, err := loadOrder(ctx, pool, orderID, promoCode)
	if err != nil {
		return fmt.Errorf("load order: %w", err)
	}

	fmt.Printf("order ID: %d\n", orderID)
	fmt.Printf("order status: %s\n", status)
	fmt.Printf("promo code: %s\n", promoCode)
	fmt.Printf("promo applied: %t\n", promoApplied)

	return nil
}

func processOrder(
	ctx context.Context,
	pool *xpg.Pool,
	orderID int64,
	promoCode string,
) error {
	return pool.InTx(
		ctx,
		pgx.TxOptions{},
		func(ctx context.Context, tx pgx.Tx) error {
			if _, err := tx.Exec(
				ctx,
				`INSERT INTO xpg_transactions_example.orders (id, status)
				 VALUES ($1, $2)
				 ON CONFLICT (id) DO UPDATE
				 SET status = EXCLUDED.status`,
				orderID,
				"new",
			); err != nil {
				return fmt.Errorf("upsert order: %w", err)
			}

			err := xpg.InSavepoint(
				ctx,
				tx,
				func(ctx context.Context, savepoint pgx.Tx) error {
					_, err := savepoint.Exec(
						ctx,
						`INSERT INTO xpg_transactions_example.promo_redemptions (
							code,
							order_id
						)
						VALUES ($1, $2)`,
						promoCode,
						orderID,
					)

					return err
				},
			)
			if err == nil {
				return nil
			}

			if xpg.IsUniqueViolation(err) {
				// InSavepoint already rolled back the failed promo insert.
				return nil
			}

			return fmt.Errorf("apply promo: %w", err)
		},
	)
}

func loadOrder(
	ctx context.Context,
	pool *xpg.Pool,
	orderID int64,
	promoCode string,
) (string, bool, error) {
	var (
		status       string
		promoApplied bool
	)

	err := pool.QueryRow(
		ctx,
		`SELECT
			o.status,
			EXISTS (
				SELECT 1
				FROM xpg_transactions_example.promo_redemptions AS p
				WHERE p.order_id = o.id
				  AND p.code = $2
			)
		FROM xpg_transactions_example.orders AS o
		WHERE o.id = $1`,
		orderID,
		promoCode,
	).Scan(
		&status,
		&promoApplied,
	)
	if err != nil {
		return "", false, err
	}

	return status, promoApplied, nil
}

func databaseURL() string {
	if value := os.Getenv("XPG_DATABASE_URL"); value != "" {
		return value
	}

	return defaultDatabaseURL
}
