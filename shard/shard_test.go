package shard

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg/cluster"
)

func TestShardZeroValue(t *testing.T) {
	t.Parallel()

	var shard Shard

	if got := shard.ID(); got != "" {
		t.Fatalf("ID() = %q, want empty", got)
	}

	if value, ok := shard.Label("region"); ok || value != "" {
		t.Fatalf("Label() = %q, %v; want empty, false", value, ok)
	}

	if labels := shard.Labels(); labels != nil {
		t.Fatalf("Labels() = %#v, want nil", labels)
	}

	if primary := shard.Primary(); primary != nil {
		t.Fatalf("Primary() = %p, want nil", primary)
	}

	if _, err := shard.ReadPool(context.Background(), cluster.ReadPrimary); !errors.Is(err, ErrNoShard) {
		t.Fatalf("ReadPool() error = %v, want ErrNoShard", err)
	}

	if err := shard.InPrimaryTx(context.Background(), pgx.TxOptions{}, nil); !errors.Is(err, ErrNoShard) {
		t.Fatalf("InPrimaryTx() error = %v, want ErrNoShard", err)
	}

	if err := shard.InReadTx(
		context.Background(),
		cluster.ReadPrimary,
		cluster.ReadTxOptions{},
		nil,
	); !errors.Is(err, ErrNoShard) {
		t.Fatalf("InReadTx() error = %v, want ErrNoShard", err)
	}
}

func TestShardDelegatesClusterMetadataAndRouting(t *testing.T) {
	t.Parallel()

	shardCluster := newTestCluster(t, "shard-a", map[string]string{
		"region": "eu-west",
		"role":   "",
	})

	topology, err := NewTopology([]Config{{Cluster: shardCluster}})
	if err != nil {
		t.Fatalf("NewTopology() error = %v", err)
	}
	t.Cleanup(topology.Close)

	resolved := topology.At(0)

	if got, want := resolved.ID(), ID("shard-a"); got != want {
		t.Fatalf("ID() = %q, want %q", got, want)
	}

	if got, ok := resolved.Label("region"); !ok || got != "eu-west" {
		t.Fatalf("Label(region) = %q, %v", got, ok)
	}

	if got, ok := resolved.Label("role"); !ok || got != "" {
		t.Fatalf("Label(role) = %q, %v", got, ok)
	}

	labels := resolved.Labels()
	labels["region"] = "changed"

	if got, _ := resolved.Label("region"); got != "eu-west" {
		t.Fatalf("Label(region) after mutation = %q, want eu-west", got)
	}

	if resolved.Primary() != shardCluster.Primary() {
		t.Fatal("Primary() did not return cluster primary")
	}

	pool, err := resolved.ReadPool(context.Background(), cluster.ReadPrimary)
	if err != nil {
		t.Fatalf("ReadPool() error = %v", err)
	}

	if pool != shardCluster.Primary() {
		t.Fatal("ReadPool() did not return cluster primary")
	}
}
