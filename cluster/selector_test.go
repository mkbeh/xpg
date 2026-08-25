package cluster

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestReplicaInfoLabelsAreDefensive(t *testing.T) {
	t.Parallel()

	info := ReplicaInfo{
		name: "replica",
		labels: map[string]string{
			"region": "eu",
		},
	}

	labels := info.Labels()
	labels["region"] = "us"
	labels["new"] = "value"

	if got, want := info.Name(), "replica"; got != want {
		t.Fatalf("Name() = %q, want %q", got, want)
	}

	if got, ok := info.Label("region"); !ok || got != "eu" {
		t.Fatalf("Label(region) = %q, %v; want %q, true", got, ok, "eu")
	}

	if _, ok := info.Label("new"); ok {
		t.Fatal("Labels() exposed internal map")
	}
}

func TestReplicaMetadataAtPanicsOutOfRange(t *testing.T) {
	t.Parallel()

	replicas := replicaMetadata{{name: "replica"}}

	defer func() {
		if recover() == nil {
			t.Fatal("At() did not panic")
		}
	}()

	_ = replicas.At(1)
}

func TestReplicaSelectorFunc(t *testing.T) {
	t.Parallel()

	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "value")
	replicas := replicaMetadata{{name: "replica"}}

	selector := ReplicaSelectorFunc(func(gotCtx context.Context, gotReplicas ReplicaSet) (int, error) {
		if got := gotCtx.Value(contextKey{}); got != "value" {
			t.Fatalf("context value = %v, want %q", got, "value")
		}

		if got, want := gotReplicas.Len(), 1; got != want {
			t.Fatalf("replicas.Len() = %d, want %d", got, want)
		}

		return 0, nil
	})

	index, err := selector.Select(ctx, replicas)
	if err != nil {
		t.Fatalf("Select() error = %v", err)
	}

	if index != 0 {
		t.Fatalf("Select() index = %d, want 0", index)
	}
}

func TestReplicaSelectorFuncNil(t *testing.T) {
	t.Parallel()

	var selector ReplicaSelectorFunc

	index, err := selector.Select(t.Context(), nil)
	if index != -1 {
		t.Fatalf("Select() index = %d, want -1", index)
	}

	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/cluster: replica selector function is nil"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestRoundRobinSelectorEmpty(t *testing.T) {
	t.Parallel()

	selector := RoundRobinSelector()

	index, err := selector.Select(t.Context(), replicaMetadata(nil))
	if index != -1 {
		t.Fatalf("Select() index = %d, want -1", index)
	}

	if !errors.Is(err, ErrNoReplica) {
		t.Fatalf("Select() error = %v, want ErrNoReplica", err)
	}
}

func TestRoundRobinSelectorSingleReplica(t *testing.T) {
	t.Parallel()

	selector := RoundRobinSelector()
	replicas := replicaMetadata{{name: "replica"}}

	for range 10 {
		index, err := selector.Select(t.Context(), replicas)
		if err != nil {
			t.Fatalf("Select() error = %v", err)
		}

		if index != 0 {
			t.Fatalf("Select() index = %d, want 0", index)
		}
	}
}

func TestRoundRobinSelectorSequence(t *testing.T) {
	t.Parallel()

	selector := RoundRobinSelector()
	replicas := replicaMetadata{
		{name: "replica-a"},
		{name: "replica-b"},
		{name: "replica-c"},
	}
	want := []int{0, 1, 2, 0, 1, 2, 0}

	for call, wantIndex := range want {
		index, err := selector.Select(t.Context(), replicas)
		if err != nil {
			t.Fatalf("Select() call %d error = %v", call, err)
		}

		if index != wantIndex {
			t.Fatalf("Select() call %d index = %d, want %d", call, index, wantIndex)
		}
	}
}

func TestRoundRobinSelectorConcurrent(t *testing.T) {
	t.Parallel()

	const (
		replicaCount = 3
		callCount    = 600
	)

	selector := RoundRobinSelector()
	replicas := make(replicaMetadata, replicaCount)
	results := make(chan int, callCount)
	ctx := t.Context()

	var waitGroup sync.WaitGroup
	waitGroup.Add(callCount)

	for range callCount {
		go func() {
			defer waitGroup.Done()

			index, err := selector.Select(ctx, replicas)
			if err != nil {
				results <- -1
				return
			}

			results <- index
		}()
	}

	waitGroup.Wait()
	close(results)

	counts := make([]int, replicaCount)

	for index := range results {
		if index < 0 || index >= replicaCount {
			t.Fatalf("Select() returned invalid index %d", index)
		}

		counts[index]++
	}

	for index, count := range counts {
		if count != callCount/replicaCount {
			t.Fatalf("replica %d selections = %d, want %d", index, count, callCount/replicaCount)
		}
	}
}
