package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/mkbeh/xpg"
)

const defaultDatabaseURL = "postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable"

type user struct {
	ID     int64
	Name   string
	Email  string
	Active bool
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
		xpg.WithName("basic-example"),
	)
	if err != nil {
		return fmt.Errorf("open pool: %w", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("ping PostgreSQL: %w", err)
	}

	inserted, err := upsertUsers(ctx, pool)
	if err != nil {
		return fmt.Errorf("upsert users: %w", err)
	}

	selected, err := loadUser(ctx, pool, 1)
	if err != nil {
		return fmt.Errorf("load user: %w", err)
	}

	active, err := listActiveUsers(ctx, pool)
	if err != nil {
		return fmt.Errorf("list active users: %w", err)
	}

	fmt.Printf("pool: %s\n", pool.Name())
	fmt.Printf("upserted users: %d\n", inserted)
	fmt.Printf(
		"selected user: %d %s <%s> active=%t\n",
		selected.ID,
		selected.Name,
		selected.Email,
		selected.Active,
	)

	fmt.Println("active users:")
	for _, current := range active {
		fmt.Printf("- %d %s <%s>\n", current.ID, current.Name, current.Email)
	}

	return nil
}

func upsertUsers(ctx context.Context, pool *xpg.Pool) (int64, error) {
	tag, err := pool.Exec(
		ctx,
		`INSERT INTO xpg_basic_example.users (
			id,
			name,
			email,
			active
		)
		VALUES
			($1, $2, $3, $4),
			($5, $6, $7, $8)
		ON CONFLICT (id) DO UPDATE SET
			name = EXCLUDED.name,
			email = EXCLUDED.email,
			active = EXCLUDED.active`,
		int64(1),
		"Alice",
		"alice@example.com",
		true,
		int64(2),
		"Bob",
		"bob@example.com",
		false,
	)
	if err != nil {
		return 0, err
	}

	return tag.RowsAffected(), nil
}

func loadUser(ctx context.Context, pool *xpg.Pool, userID int64) (user, error) {
	var selected user

	err := pool.QueryRow(
		ctx,
		`SELECT
			id,
			name,
			email,
			active
		FROM xpg_basic_example.users
		WHERE id = $1`,
		userID,
	).Scan(
		&selected.ID,
		&selected.Name,
		&selected.Email,
		&selected.Active,
	)
	if err != nil {
		return user{}, err
	}

	return selected, nil
}

func listActiveUsers(ctx context.Context, pool *xpg.Pool) ([]user, error) {
	rows, err := pool.Query(
		ctx,
		`SELECT
			id,
			name,
			email,
			active
		FROM xpg_basic_example.users
		WHERE active
		ORDER BY id`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []user

	for rows.Next() {
		var current user

		if err := rows.Scan(
			&current.ID,
			&current.Name,
			&current.Email,
			&current.Active,
		); err != nil {
			return nil, err
		}

		users = append(users, current)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func databaseURL() string {
	if value := os.Getenv("XPG_DATABASE_URL"); value != "" {
		return value
	}

	return defaultDatabaseURL
}
