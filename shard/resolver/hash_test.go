package resolver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mkbeh/xpg/shard"
)

func TestNewHashValidatesArguments(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	if resolver, err := NewHash[string](nil, "users", StringKeyEncoder()); err == nil {
		_ = resolver
		t.Fatal("expected topology error")
	}

	if resolver, err := NewHash[string](topology, "users", nil); err == nil {
		_ = resolver
		t.Fatal("expected encoder error")
	}

	if resolver, err := NewHash(topology, "", StringKeyEncoder()); err == nil {
		_ = resolver
		t.Fatal("expected namespace error")
	}
}

func TestNewHashTreatsNamespaceAsOpaqueNonEmptyString(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	for _, namespace := range []string{"users", " users ", " "} {
		resolver, err := NewHash(topology, namespace, StringKeyEncoder())
		if err != nil {
			t.Fatalf("NewHash(%q) error = %v", namespace, err)
		}

		resolved, err := resolver.Resolve("alice")
		if err != nil {
			t.Fatalf("Resolve() error = %v", err)
		}

		if got, want := resolved.ID(), shard.ID("shard-a"); got != want {
			t.Fatalf("Resolve().ID() = %q, want %q", got, want)
		}
	}
}

func TestHashResolverStablePlacementVectors(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b", "shard-c")
	resolver, err := NewHash(topology, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewHash() error = %v", err)
	}

	tests := []struct {
		key  string
		want shard.ID
	}{
		{key: "alice", want: "shard-a"},
		{key: "bob", want: "shard-b"},
		{key: "carol", want: "shard-b"},
		{key: "dave", want: "shard-b"},
		{key: "eve", want: "shard-c"},
		{key: "0", want: "shard-a"},
		{key: "1", want: "shard-b"},
		{key: "2", want: "shard-c"},
	}

	for _, test := range tests {
		t.Run(test.key, func(t *testing.T) {
			resolved, err := resolver.Resolve(test.key)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			if got := resolved.ID(); got != test.want {
				t.Fatalf("Resolve(%q).ID() = %q, want %q", test.key, got, test.want)
			}
		})
	}
}

func TestHashResolverPlacementDoesNotDependOnTopologyOrder(t *testing.T) {
	t.Parallel()

	first := newTestTopology(t, "shard-a", "shard-b", "shard-c")
	second := newTestTopology(t, "shard-c", "shard-a", "shard-b")

	firstResolver, err := NewHash(first, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewHash(first) error = %v", err)
	}
	secondResolver, err := NewHash(second, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewHash(second) error = %v", err)
	}

	for _, key := range []string{"alice", "bob", "carol", "dave", "eve", "user-123"} {
		firstShard, err := firstResolver.Resolve(key)
		if err != nil {
			t.Fatalf("first Resolve(%q) error = %v", key, err)
		}
		secondShard, err := secondResolver.Resolve(key)
		if err != nil {
			t.Fatalf("second Resolve(%q) error = %v", key, err)
		}

		if firstShard.ID() != secondShard.ID() {
			t.Fatalf(
				"Resolve(%q) = %q and %q for different topology orders",
				key,
				firstShard.ID(),
				secondShard.ID(),
			)
		}
	}
}

func TestHashResolverAddingShardOnlyMovesKeysToNewShard(t *testing.T) {
	t.Parallel()

	before := newTestTopology(t, "shard-a", "shard-b")
	after := newTestTopology(t, "shard-a", "shard-b", "shard-c")

	beforeResolver, err := NewHash(before, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewHash(before) error = %v", err)
	}
	afterResolver, err := NewHash(after, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewHash(after) error = %v", err)
	}

	moved := 0

	for index := range 256 {
		key := fmt.Sprintf("user-%d", index)

		previous, err := beforeResolver.Resolve(key)
		if err != nil {
			t.Fatalf("before Resolve(%q) error = %v", key, err)
		}
		current, err := afterResolver.Resolve(key)
		if err != nil {
			t.Fatalf("after Resolve(%q) error = %v", key, err)
		}

		if previous.ID() == current.ID() {
			continue
		}

		moved++
		if current.ID() != "shard-c" {
			t.Fatalf(
				"Resolve(%q) moved from %q to existing shard %q",
				key,
				previous.ID(),
				current.ID(),
			)
		}
	}

	if moved == 0 {
		t.Fatal("expected at least one key to move to the new shard")
	}
}

func TestHashResolverWrapsEncoderError(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	sentinel := errors.New("encode failed")

	resolver, err := NewHash(
		topology,
		"users",
		KeyEncoderFunc[string](func(string) ([]byte, error) {
			return nil, sentinel
		}),
	)
	if err != nil {
		t.Fatalf("NewHash() error = %v", err)
	}

	_, err = resolver.Resolve("alice")
	if !errors.Is(err, sentinel) {
		t.Fatalf("Resolve() error = %v, want wrapped sentinel", err)
	}
}

func TestHashResolverUninitialized(t *testing.T) {
	t.Parallel()

	var resolver *HashResolver[string]

	_, err := resolver.Resolve("alice")
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/shard/resolver: hash resolver is not initialized"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
