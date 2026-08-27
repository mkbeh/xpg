package shard

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg/topology/cluster"
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

	if _, err := shard.ReadPool(t.Context(), cluster.ReadPrimary); !errors.Is(err, ErrNoShard) {
		t.Fatalf("ReadPool() error = %v, want ErrNoShard", err)
	}

	if err := shard.InPrimaryTx(t.Context(), pgx.TxOptions{}, nil); !errors.Is(err, ErrNoShard) {
		t.Fatalf("InPrimaryTx() error = %v, want ErrNoShard", err)
	}

	if err := shard.InReadTx(
		t.Context(),
		cluster.ReadPrimary,
		cluster.ReadTxOptions{},
		nil,
	); !errors.Is(err, ErrNoShard) {
		t.Fatalf("InReadTx() error = %v, want ErrNoShard", err)
	}
}

func TestShardDelegatesClusterMetadataAndRouting(t *testing.T) {
	t.Parallel()

	dbCluster := newTestCluster(t, "shard-a", map[string]string{
		"region": "eu-west",
		"role":   "",
	})

	topology, err := NewTopology(dbCluster)
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

	if resolved.Primary() != dbCluster.Primary() {
		t.Fatal("Primary() did not return cluster primary")
	}

	pool, err := resolved.ReadPool(t.Context(), cluster.ReadPrimary)
	if err != nil {
		t.Fatalf("ReadPool() error = %v", err)
	}

	if pool != dbCluster.Primary() {
		t.Fatal("ReadPool() did not return cluster primary")
	}
}
