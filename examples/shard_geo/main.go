package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg/topology/shard"
)

type tenantKey struct {
	TenantID string
	Region   string
}

type tenant struct {
	ID     string
	Name   string
	Region string
}

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	topology, err := openTopology(ctx)
	if err != nil {
		return err
	}
	defer topology.Close()

	tenantResolver, err := newTenantResolver(topology)
	if err != nil {
		return fmt.Errorf("create tenant resolver: %w", err)
	}

	tenants := []tenant{
		{
			ID:     "tenant-42",
			Name:   "Alice",
			Region: "eu",
		},
		{
			ID:     "tenant-77",
			Name:   "Bob",
			Region: "us",
		},
	}

	fmt.Println("geo routing:")

	for _, current := range tenants {
		key := tenantKey{
			TenantID: current.ID,
			Region:   current.Region,
		}

		resolved, err := tenantResolver.Resolve(key)
		if err != nil {
			return fmt.Errorf("resolve tenant %q: %w", current.ID, err)
		}

		if err := upsertTenant(ctx, resolved, current); err != nil {
			return fmt.Errorf("upsert tenant %q: %w", current.ID, err)
		}

		stored, err := loadTenant(ctx, resolved, current.ID)
		if err != nil {
			return fmt.Errorf("load tenant %q: %w", current.ID, err)
		}

		fmt.Printf(
			"- tenant=%s region=%s shard=%s name=%s\n",
			stored.ID,
			stored.Region,
			resolved.ID(),
			stored.Name,
		)
	}

	_, err = tenantResolver.Resolve(tenantKey{
		TenantID: "tenant-99",
		Region:   "apac",
	})
	if !errors.Is(err, shard.ErrNoShard) {
		return fmt.Errorf("resolve unsupported region: got %v, want shard.ErrNoShard", err)
	}

	fmt.Println("unsupported region: no shard")

	return nil
}

func upsertTenant(ctx context.Context, resolved shard.Shard, current tenant) error {
	return resolved.InPrimaryTx(
		ctx,
		pgx.TxOptions{},
		func(ctx context.Context, tx pgx.Tx) error {
			_, err := tx.Exec(
				ctx,
				`INSERT INTO xpg_shard_geo_example.tenants (
					id,
					name,
					region,
					last_seen_at
				)
				VALUES ($1, $2, $3, now())
				ON CONFLICT (id) DO UPDATE SET
					name = EXCLUDED.name,
					region = EXCLUDED.region,
					last_seen_at = EXCLUDED.last_seen_at`,
				current.ID,
				current.Name,
				current.Region,
			)

			return err
		},
	)
}

func loadTenant(ctx context.Context, resolved shard.Shard, tenantID string) (tenant, error) {
	primary := resolved.Primary()
	if primary == nil {
		return tenant{}, fmt.Errorf("shard %q has no primary", resolved.ID())
	}

	var stored tenant

	err := primary.QueryRow(
		ctx,
		`SELECT id, name, region
		 FROM xpg_shard_geo_example.tenants
		 WHERE id = $1`,
		tenantID,
	).Scan(
		&stored.ID,
		&stored.Name,
		&stored.Region,
	)
	if err != nil {
		return tenant{}, err
	}

	return stored, nil
}
