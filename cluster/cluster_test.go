package cluster

import (
	"context"
	"testing"

	"github.com/mkbeh/xpg"
)

func TestNewRequiresPool(t *testing.T) {
	t.Parallel()

	cluster, err := New(Config{})
	if err == nil {
		cluster.Close()
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/cluster: at least one pool is required"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNewRejectsInvalidPrimary(t *testing.T) {
	t.Parallel()

	cluster, err := New(Config{
		Primary: &xpg.Pool{},
	})
	if err == nil {
		cluster.Close()
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/cluster: primary pool is invalid"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNewRejectsInvalidReplica(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		replica *xpg.Pool
	}{
		{
			name:    "nil",
			replica: nil,
		},
		{
			name:    "zero value",
			replica: &xpg.Pool{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cluster, err := New(Config{
				Replicas: []*xpg.Pool{test.replica},
			})
			if err == nil {
				cluster.Close()
				t.Fatal("expected error")
			}

			if got, want := err.Error(), "xpg/cluster: replica 0 is invalid"; got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
		})
	}
}

func TestNewLabels(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	labels := map[string]string{
		"environment": "",
		"   ":         "whitespace-key",
	}

	cluster, err := New(Config{
		ID:      "orders",
		Labels:  labels,
		Primary: primary,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(cluster.Close)

	labels["environment"] = "changed"
	labels["new"] = "value"

	if got, want := cluster.ID(), ID("orders"); got != want {
		t.Fatalf("ID() = %q, want %q", got, want)
	}

	if got, ok := cluster.Label("environment"); !ok || got != "" {
		t.Fatalf("Label(environment) = %q, %v; want empty value, true", got, ok)
	}

	if got, ok := cluster.Label("   "); !ok || got != "whitespace-key" {
		t.Fatalf("Label(whitespace) = %q, %v; want %q, true", got, ok, "whitespace-key")
	}

	if _, ok := cluster.Label("new"); ok {
		t.Fatal("cluster labels changed after input map mutation")
	}

	cloned := cluster.Labels()
	cloned["environment"] = "mutated"
	delete(cloned, "   ")

	if got, _ := cluster.Label("environment"); got != "" {
		t.Fatalf("Label(environment) after Labels mutation = %q, want empty value", got)
	}

	if got, ok := cluster.Label("   "); !ok || got != "whitespace-key" {
		t.Fatalf("Label(whitespace) after Labels mutation = %q, %v; want %q, true", got, ok, "whitespace-key")
	}
}

func TestNewRejectsEmptyLabelKey(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)

	cluster, err := New(Config{
		Labels: map[string]string{
			"": "value",
		},
		Primary: primary,
	})
	if err == nil {
		cluster.Close()
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/cluster: label key must not be empty"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestNewClonesReplicaSlice(t *testing.T) {
	t.Parallel()

	replicaA := newTestPool(t, "replica-a", nil)
	replicaB := newTestPool(t, "replica-b", nil)
	replicas := []*xpg.Pool{replicaA}

	cluster, err := New(Config{
		Replicas: replicas,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(cluster.Close)

	replicas[0] = replicaB

	if got := cluster.ReplicaAt(0); got != replicaA {
		t.Fatalf("ReplicaAt(0) = %p, want original replica %p", got, replicaA)
	}
}

func TestNewAllowsDuplicatePools(t *testing.T) {
	t.Parallel()

	pool := newTestPool(t, "shared", nil)

	cluster, err := New(Config{
		Primary: pool,
		Replicas: []*xpg.Pool{
			pool,
			pool,
		},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(cluster.Close)

	if cluster.Primary() != pool {
		t.Fatal("Primary() did not return the configured pool")
	}

	if got, want := cluster.ReplicaCount(), 2; got != want {
		t.Fatalf("ReplicaCount() = %d, want %d", got, want)
	}

	if cluster.ReplicaAt(0) != pool || cluster.ReplicaAt(1) != pool {
		t.Fatal("duplicate topology entries were not preserved")
	}
}

func TestNewCapturesReplicaMetadata(t *testing.T) {
	t.Parallel()

	replica := newTestPool(t, "replica-a", map[string]string{
		"region": "eu",
		"role":   "read",
	})

	selectorCalls := 0
	selector := ReplicaSelectorFunc(func(_ context.Context, replicas ReplicaSet) (int, error) {
		selectorCalls++

		if got, want := replicas.Len(), 1; got != want {
			t.Fatalf("replicas.Len() = %d, want %d", got, want)
		}

		info := replicas.At(0)

		if got, want := info.Name(), "replica-a"; got != want {
			t.Fatalf("replica name = %q, want %q", got, want)
		}

		if got, ok := info.Label("region"); !ok || got != "eu" {
			t.Fatalf("region label = %q, %v; want %q, true", got, ok, "eu")
		}

		labels := info.Labels()
		labels["region"] = "mutated"

		if got, _ := info.Label("region"); got != "eu" {
			t.Fatalf("replica metadata mutated through Labels(): got %q", got)
		}

		return 0, nil
	})

	cluster, err := New(Config{
		Replicas: []*xpg.Pool{replica},
		Selector: selector,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	t.Cleanup(cluster.Close)

	resolved, err := cluster.ReadPool(context.Background(), ReadReplicaRequired)
	if err != nil {
		t.Fatalf("ReadPool() error = %v", err)
	}

	if resolved != replica {
		t.Fatalf("ReadPool() = %p, want %p", resolved, replica)
	}

	if selectorCalls != 1 {
		t.Fatalf("selector calls = %d, want 1", selectorCalls)
	}
}

func TestClusterNilReceiverMetadata(t *testing.T) {
	t.Parallel()

	var cluster *Cluster

	if got := cluster.ID(); got != "" {
		t.Fatalf("ID() = %q, want empty", got)
	}

	if value, ok := cluster.Label("key"); ok || value != "" {
		t.Fatalf("Label() = %q, %v; want empty, false", value, ok)
	}

	if labels := cluster.Labels(); labels != nil {
		t.Fatalf("Labels() = %v, want nil", labels)
	}

	if primary := cluster.Primary(); primary != nil {
		t.Fatalf("Primary() = %p, want nil", primary)
	}

	if got := cluster.ReplicaCount(); got != 0 {
		t.Fatalf("ReplicaCount() = %d, want 0", got)
	}

	cluster.Close()
}

func TestReplicaAtPanicsOutOfRange(t *testing.T) {
	t.Parallel()

	replica := newTestPool(t, "replica", nil)
	cluster := newTestCluster(t, Config{
		Replicas: []*xpg.Pool{replica},
	})

	defer func() {
		if recover() == nil {
			t.Fatal("ReplicaAt() did not panic")
		}
	}()

	_ = cluster.ReplicaAt(1)
}

func TestCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	replica := newTestPool(t, "replica", nil)

	cluster, err := New(Config{
		Primary:  primary,
		Replicas: []*xpg.Pool{replica},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	cluster.Close()
	cluster.Close()
}
