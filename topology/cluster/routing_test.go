package cluster

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/mkbeh/xpg"
)

func TestParseReadPolicy(t *testing.T) {
	t.Parallel()

	tests := []struct {
		value string
		want  ReadPolicy
	}{
		{value: "primary", want: ReadPrimary},
		{value: "replica_preferred", want: ReadReplicaPreferred},
		{value: "replica_required", want: ReadReplicaRequired},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			t.Parallel()

			got, err := ParseReadPolicy(test.value)
			if err != nil {
				t.Fatalf("ParseReadPolicy() error = %v", err)
			}

			if got != test.want {
				t.Fatalf("ParseReadPolicy(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}

func TestParseReadPolicyRejectsUnknown(t *testing.T) {
	t.Parallel()

	_, err := ParseReadPolicy("nearest")
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), `xpg/topology/cluster: unknown read policy "nearest"`; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReadPolicyString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		policy ReadPolicy
		want   string
	}{
		{policy: ReadPrimary, want: "primary"},
		{policy: ReadReplicaPreferred, want: "replica_preferred"},
		{policy: ReadReplicaRequired, want: "replica_required"},
		{policy: ReadPolicy(255), want: "unknown"},
	}

	for _, test := range tests {
		if got := test.policy.String(); got != test.want {
			t.Fatalf("ReadPolicy(%d).String() = %q, want %q", test.policy, got, test.want)
		}
	}
}

func TestReadPoolNilCluster(t *testing.T) {
	t.Parallel()

	var cluster *Cluster

	pool, err := cluster.ReadPool(t.Context(), ReadPrimary)
	if pool != nil {
		t.Fatalf("ReadPool() pool = %p, want nil", pool)
	}

	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/cluster: cluster is nil"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReadPoolPrimary(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	cluster := newTestCluster(t, Config{Primary: primary})

	got, err := cluster.ReadPool(t.Context(), ReadPrimary)
	if err != nil {
		t.Fatalf("ReadPool() error = %v", err)
	}

	if got != primary {
		t.Fatalf("ReadPool() = %p, want %p", got, primary)
	}
}

func TestReadPoolPrimaryWithoutPrimary(t *testing.T) {
	t.Parallel()

	replica := newTestPool(t, "replica", nil)
	cluster := newTestCluster(t, Config{
		Replicas: []*xpg.Pool{replica},
	})

	pool, err := cluster.ReadPool(t.Context(), ReadPrimary)
	if pool != nil {
		t.Fatalf("ReadPool() pool = %p, want nil", pool)
	}

	if !errors.Is(err, ErrNoPrimary) {
		t.Fatalf("ReadPool() error = %v, want ErrNoPrimary", err)
	}
}

func TestReadPoolReplicaPreferredUsesReplica(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	replica := newTestPool(t, "replica", nil)
	cluster := newTestCluster(t, Config{
		Primary:  primary,
		Replicas: []*xpg.Pool{replica},
	})

	got, err := cluster.ReadPool(t.Context(), ReadReplicaPreferred)
	if err != nil {
		t.Fatalf("ReadPool() error = %v", err)
	}

	if got != replica {
		t.Fatalf("ReadPool() = %p, want replica %p", got, replica)
	}
}

func TestReadPoolReplicaPreferredFallsBackWithoutReplicas(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	cluster := newTestCluster(t, Config{Primary: primary})

	got, err := cluster.ReadPool(t.Context(), ReadReplicaPreferred)
	if err != nil {
		t.Fatalf("ReadPool() error = %v", err)
	}

	if got != primary {
		t.Fatalf("ReadPool() = %p, want primary %p", got, primary)
	}
}

func TestReadPoolReplicaPreferredFallsBackOnErrNoReplica(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	replica := newTestPool(t, "replica", nil)
	selector := ReplicaSelectorFunc(func(context.Context, ReplicaSet) (int, error) {
		return -1, errors.Join(errors.New("temporarily unavailable"), ErrNoReplica)
	})

	cluster := newTestCluster(t, Config{
		Primary:  primary,
		Replicas: []*xpg.Pool{replica},
		Selector: selector,
	})

	got, err := cluster.ReadPool(t.Context(), ReadReplicaPreferred)
	if err != nil {
		t.Fatalf("ReadPool() error = %v", err)
	}

	if got != primary {
		t.Fatalf("ReadPool() = %p, want primary %p", got, primary)
	}
}

func TestReadPoolReplicaPreferredDoesNotFallbackOnSelectorError(t *testing.T) {
	t.Parallel()

	errSelector := errors.New("selector failed")
	primary := newTestPool(t, "primary", nil)
	replica := newTestPool(t, "replica", nil)
	selector := ReplicaSelectorFunc(func(context.Context, ReplicaSet) (int, error) {
		return -1, errSelector
	})

	cluster := newTestCluster(t, Config{
		Primary:  primary,
		Replicas: []*xpg.Pool{replica},
		Selector: selector,
	})

	pool, err := cluster.ReadPool(t.Context(), ReadReplicaPreferred)
	if pool != nil {
		t.Fatalf("ReadPool() pool = %p, want nil", pool)
	}

	if !errors.Is(err, errSelector) {
		t.Fatalf("ReadPool() error = %v, want selector error", err)
	}
}

func TestReadPoolReplicaPreferredWithoutPrimary(t *testing.T) {
	t.Parallel()

	replica := newTestPool(t, "replica", nil)
	selector := ReplicaSelectorFunc(func(context.Context, ReplicaSet) (int, error) {
		return -1, ErrNoReplica
	})

	cluster := newTestCluster(t, Config{
		Replicas: []*xpg.Pool{replica},
		Selector: selector,
	})

	pool, err := cluster.ReadPool(t.Context(), ReadReplicaPreferred)
	if pool != nil {
		t.Fatalf("ReadPool() pool = %p, want nil", pool)
	}

	if !errors.Is(err, ErrNoPrimary) {
		t.Fatalf("ReadPool() error = %v, want ErrNoPrimary", err)
	}
}

func TestReadPoolReplicaRequired(t *testing.T) {
	t.Parallel()

	replica := newTestPool(t, "replica", nil)
	cluster := newTestCluster(t, Config{
		Replicas: []*xpg.Pool{replica},
	})

	got, err := cluster.ReadPool(t.Context(), ReadReplicaRequired)
	if err != nil {
		t.Fatalf("ReadPool() error = %v", err)
	}

	if got != replica {
		t.Fatalf("ReadPool() = %p, want %p", got, replica)
	}
}

func TestReadPoolReplicaRequiredWithoutReplicas(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	cluster := newTestCluster(t, Config{Primary: primary})

	pool, err := cluster.ReadPool(t.Context(), ReadReplicaRequired)
	if pool != nil {
		t.Fatalf("ReadPool() pool = %p, want nil", pool)
	}

	if !errors.Is(err, ErrNoReplica) {
		t.Fatalf("ReadPool() error = %v, want ErrNoReplica", err)
	}
}

func TestReadPoolDefaultSelectorRoundRobin(t *testing.T) {
	t.Parallel()

	replicaA := newTestPool(t, "replica-a", nil)
	replicaB := newTestPool(t, "replica-b", nil)
	cluster := newTestCluster(t, Config{
		Replicas: []*xpg.Pool{replicaA, replicaB},
	})

	want := []*xpg.Pool{
		replicaA,
		replicaB,
		replicaA,
		replicaB,
	}

	for call, wantPool := range want {
		got, err := cluster.ReadPool(t.Context(), ReadReplicaRequired)
		if err != nil {
			t.Fatalf("ReadPool() call %d error = %v", call, err)
		}

		if got != wantPool {
			t.Fatalf("ReadPool() call %d = %p, want %p", call, got, wantPool)
		}
	}
}

func TestReadPoolRejectsSelectorIndex(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		index int
	}{
		{name: "negative", index: -1},
		{name: "past end", index: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			replica := newTestPool(t, "replica", nil)
			selector := ReplicaSelectorFunc(func(context.Context, ReplicaSet) (int, error) {
				return test.index, nil
			})

			cluster := newTestCluster(t, Config{
				Replicas: []*xpg.Pool{replica},
				Selector: selector,
			})

			pool, err := cluster.ReadPool(t.Context(), ReadReplicaRequired)
			if pool != nil {
				t.Fatalf("ReadPool() pool = %p, want nil", pool)
			}

			if err == nil {
				t.Fatal("expected error")
			}

			want := fmt.Sprintf(
				"xpg/topology/cluster: replica selector returned invalid index %d for 1 replicas",
				test.index,
			)

			if got := err.Error(); got != want {
				t.Fatalf("error = %q, want %q", got, want)
			}
		})
	}
}

func TestReadPoolPreservesSelectorError(t *testing.T) {
	t.Parallel()

	errSelector := errors.New("boom")
	replica := newTestPool(t, "replica", nil)
	selector := ReplicaSelectorFunc(func(context.Context, ReplicaSet) (int, error) {
		return -1, errSelector
	})

	cluster := newTestCluster(t, Config{
		Replicas: []*xpg.Pool{replica},
		Selector: selector,
	})

	pool, err := cluster.ReadPool(t.Context(), ReadReplicaRequired)
	if pool != nil {
		t.Fatalf("ReadPool() pool = %p, want nil", pool)
	}

	if !errors.Is(err, errSelector) {
		t.Fatalf("ReadPool() error = %v, want wrapped selector error", err)
	}

	if got, want := err.Error(), "xpg/topology/cluster: select replica: boom"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestReadPoolRejectsUnsupportedPolicy(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	cluster := newTestCluster(t, Config{Primary: primary})

	pool, err := cluster.ReadPool(t.Context(), ReadPolicy(255))
	if pool != nil {
		t.Fatalf("ReadPool() pool = %p, want nil", pool)
	}

	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/cluster: unsupported read policy 255"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
