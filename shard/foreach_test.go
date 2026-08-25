package shard

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestForEachShardValidatesArguments(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	tests := []struct {
		name        string
		topology    *Topology
		concurrency int
		fn          func(context.Context, Shard) error
		wantError   string
	}{
		{
			name:        "nil topology",
			topology:    nil,
			concurrency: 1,
			fn:          func(context.Context, Shard) error { return nil },
			wantError:   "xpg/shard: topology is nil",
		},
		{
			name:        "empty topology",
			topology:    &Topology{},
			concurrency: 1,
			fn:          func(context.Context, Shard) error { return nil },
			wantError:   "xpg/shard: topology is empty",
		},
		{
			name:        "zero concurrency",
			topology:    topology,
			concurrency: 0,
			fn:          func(context.Context, Shard) error { return nil },
			wantError:   "xpg/shard: concurrency must be positive",
		},
		{
			name:        "nil callback",
			topology:    topology,
			concurrency: 1,
			fn:          nil,
			wantError:   "xpg/shard: callback is nil",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := test.topology.ForEachShard(
				t.Context(),
				test.concurrency,
				test.fn,
			)
			if err == nil {
				t.Fatal("expected error")
			}

			if got := err.Error(); got != test.wantError {
				t.Fatalf("error = %q, want %q", got, test.wantError)
			}
		})
	}
}

func TestForEachShardPreservesRegistrationOrder(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-c", "shard-a", "shard-b")

	results, err := topology.ForEachShard(
		t.Context(),
		2,
		func(context.Context, Shard) error { return nil },
	)
	if err != nil {
		t.Fatalf("ForEachShard() error = %v", err)
	}

	want := []ID{"shard-c", "shard-a", "shard-b"}
	for index, wantID := range want {
		if got := results[index].ShardID; got != wantID {
			t.Fatalf("results[%d].ShardID = %q, want %q", index, got, wantID)
		}

		if results[index].Err != nil {
			t.Fatalf("results[%d].Err = %v", index, results[index].Err)
		}
	}
}

func TestForEachShardHonorsConcurrencyLimit(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(
		t,
		"shard-a",
		"shard-b",
		"shard-c",
		"shard-d",
		"shard-e",
		"shard-f",
	)

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()

	started := make(chan struct{}, topology.Len())
	release := make(chan struct{})

	var active atomic.Int32
	var maximum atomic.Int32
	var calls atomic.Int32

	done := make(chan struct {
		results ForEachShardResults
		err     error
	}, 1)

	go func() {
		results, err := topology.ForEachShard(
			ctx,
			2,
			func(ctx context.Context, _ Shard) error {
				current := active.Add(1)
				defer active.Add(-1)

				calls.Add(1)

				for {
					observed := maximum.Load()
					if current <= observed || maximum.CompareAndSwap(observed, current) {
						break
					}
				}

				started <- struct{}{}

				select {
				case <-release:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			},
		)

		done <- struct {
			results ForEachShardResults
			err     error
		}{
			results: results,
			err:     err,
		}
	}()

	for range 2 {
		select {
		case <-started:
		case <-ctx.Done():
			close(release)
			t.Fatal("two callbacks did not start concurrently")
		}
	}

	close(release)

	var outcome struct {
		results ForEachShardResults
		err     error
	}

	select {
	case outcome = <-done:
	case <-ctx.Done():
		t.Fatal("ForEachShard() did not finish")
	}

	if outcome.err != nil {
		t.Fatalf("ForEachShard() error = %v", outcome.err)
	}

	if got, want := calls.Load(), int32(topology.Len()); got != want {
		t.Fatalf("callback calls = %d, want %d", got, want)
	}

	if got, want := maximum.Load(), int32(2); got != want {
		t.Fatalf("maximum concurrent callbacks = %d, want %d", got, want)
	}

	if err := outcome.results.Err(); err != nil {
		t.Fatalf("results.Err() = %v", err)
	}
}

func TestForEachShardCallbackErrorsDoNotStopOtherShards(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b", "shard-c")
	sentinel := errors.New("callback failed")
	var calls atomic.Int32

	results, err := topology.ForEachShard(
		t.Context(),
		2,
		func(_ context.Context, current Shard) error {
			calls.Add(1)
			if current.ID() == "shard-b" {
				return sentinel
			}

			return nil
		},
	)
	if err != nil {
		t.Fatalf("ForEachShard() error = %v", err)
	}

	if got, want := calls.Load(), int32(3); got != want {
		t.Fatalf("callback calls = %d, want %d", got, want)
	}

	if results[0].Err != nil || !errors.Is(results[1].Err, sentinel) || results[2].Err != nil {
		t.Fatalf("results = %+v", results)
	}

	joined := results.Err()
	if !errors.Is(joined, sentinel) {
		t.Fatalf("results.Err() = %v, want wrapped sentinel", joined)
	}

	if got, want := joined.Error(),
		`xpg/shard: shard "shard-b": callback failed`; got != want {
		t.Fatalf("results.Err() = %q, want %q", got, want)
	}
}

func TestForEachShardCanceledBeforeScheduling(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b", "shard-c")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	var calls atomic.Int32

	results, err := topology.ForEachShard(
		ctx,
		2,
		func(context.Context, Shard) error {
			calls.Add(1)
			return nil
		},
	)
	if err != nil {
		t.Fatalf("ForEachShard() error = %v", err)
	}

	if got := calls.Load(); got != 0 {
		t.Fatalf("callback calls = %d, want 0", got)
	}

	for index, result := range results {
		if !errors.Is(result.Err, context.Canceled) {
			t.Fatalf("results[%d].Err = %v, want context.Canceled", index, result.Err)
		}
	}
}

func TestForEachShardCancellationSkipsCallbacksNotStarted(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b", "shard-c", "shard-d")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	started := make(chan struct{})
	var calls atomic.Int32

	done := make(chan struct {
		results ForEachShardResults
		err     error
	}, 1)

	go func() {
		results, err := topology.ForEachShard(
			ctx,
			1,
			func(ctx context.Context, _ Shard) error {
				if calls.Add(1) == 1 {
					close(started)
				}

				<-ctx.Done()
				return ctx.Err()
			},
		)

		done <- struct {
			results ForEachShardResults
			err     error
		}{results: results, err: err}
	}()

	<-started
	cancel()

	outcome := <-done
	if outcome.err != nil {
		t.Fatalf("ForEachShard() error = %v", outcome.err)
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("callback calls = %d, want 1", got)
	}

	for index, result := range outcome.results {
		if !errors.Is(result.Err, context.Canceled) {
			t.Fatalf("results[%d].Err = %v, want context.Canceled", index, result.Err)
		}
	}
}

func TestForEachShardResultsErr(t *testing.T) {
	t.Parallel()

	first := errors.New("first")
	second := errors.New("second")

	results := ForEachShardResults{
		{ShardID: "shard-a", Err: first},
		{ShardID: "shard-b"},
		{ShardID: "shard-c", Err: second},
	}

	err := results.Err()
	if !errors.Is(err, first) || !errors.Is(err, second) {
		t.Fatalf("Err() = %v, want both failures", err)
	}

	want := "xpg/shard: shard \"shard-a\": first\n" +
		"xpg/shard: shard \"shard-c\": second"

	if got := err.Error(); got != want {
		t.Fatalf("Err() = %q, want %q", got, want)
	}

	if err := (ForEachShardResults{{ShardID: "shard-a"}}).Err(); err != nil {
		t.Fatalf("Err() = %v, want nil", err)
	}
}
