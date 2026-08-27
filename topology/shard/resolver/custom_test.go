package resolver

import (
	"errors"
	"testing"

	"github.com/mkbeh/xpg/topology/shard"
)

func TestNewCustomValidatesArguments(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	validResolve := ResolveFunc[int](
		func(int) (shard.ID, error) {
			return "shard-a", nil
		},
	)

	tests := []struct {
		name      string
		topology  *shard.Topology
		resolve   ResolveFunc[int]
		wantError string
	}{
		{
			name:      "nil topology",
			resolve:   validResolve,
			wantError: "xpg/topology/shard/resolver: topology is nil or empty",
		},
		{
			name:      "nil resolve function",
			topology:  topology,
			wantError: "xpg/topology/shard/resolver: custom resolve function is nil",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewCustom(
				test.topology,
				test.resolve,
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

func TestCustomResolverResolve(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")

	resolver, err := NewCustom(
		topology,
		func(key int) (shard.ID, error) {
			if key < 100 {
				return "shard-a", nil
			}

			return "shard-b", nil
		},
	)
	if err != nil {
		t.Fatalf("NewCustom() error = %v", err)
	}

	resolved, err := resolver.Resolve(142)
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if got, want := resolved.ID(), shard.ID("shard-b"); got != want {
		t.Fatalf("Resolve().ID() = %q, want %q", got, want)
	}
}

func TestCustomResolverPropagatesResolveError(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	sentinel := errors.New("resolve failed")

	resolver, err := NewCustom(
		topology,
		func(int) (shard.ID, error) {
			return "", sentinel
		},
	)
	if err != nil {
		t.Fatalf("NewCustom() error = %v", err)
	}

	_, err = resolver.Resolve(1)
	if !errors.Is(err, sentinel) {
		t.Fatalf("Resolve() error = %v, want sentinel", err)
	}
}

func TestCustomResolverPropagatesErrNoShard(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	resolver, err := NewCustom(
		topology,
		func(int) (shard.ID, error) {
			return "", shard.ErrNoShard
		},
	)
	if err != nil {
		t.Fatalf("NewCustom() error = %v", err)
	}

	_, err = resolver.Resolve(1)
	if !errors.Is(err, shard.ErrNoShard) {
		t.Fatalf("Resolve() error = %v, want ErrNoShard", err)
	}
}

func TestCustomResolverRejectsUnknownShard(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	resolver, err := NewCustom(
		topology,
		func(int) (shard.ID, error) {
			return "missing", nil
		},
	)
	if err != nil {
		t.Fatalf("NewCustom() error = %v", err)
	}

	_, err = resolver.Resolve(1)
	if !errors.Is(err, shard.ErrUnknownShard) {
		t.Fatalf("Resolve() error = %v, want ErrUnknownShard", err)
	}

	var unknown *shard.UnknownShardError
	if !errors.As(err, &unknown) {
		t.Fatalf("Resolve() error = %T, want *shard.UnknownShardError", err)
	}

	if got, want := unknown.ShardID, shard.ID("missing"); got != want {
		t.Fatalf("ShardID = %q, want %q", got, want)
	}
}

func TestCustomResolverUninitialized(t *testing.T) {
	t.Parallel()

	var resolver *CustomResolver[int]

	_, err := resolver.Resolve(1)
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/shard/resolver: custom resolver is not initialized"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
