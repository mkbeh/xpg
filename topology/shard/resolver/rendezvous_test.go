package resolver

import (
	"errors"
	"fmt"
	"testing"

	"github.com/mkbeh/xpg/topology/shard"
)

func TestNewRendezvousValidatesArguments(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	tests := []struct {
		name      string
		topology  *shard.Topology
		namespace string
		encoder   KeyEncoder[string]
		wantError string
	}{
		{
			name:      "nil topology",
			namespace: "users",
			encoder:   StringKeyEncoder(),
			wantError: "xpg/topology/shard/resolver: topology is nil or empty",
		},
		{
			name:      "nil encoder",
			topology:  topology,
			namespace: "users",
			wantError: "xpg/topology/shard/resolver: key encoder is nil",
		},
		{
			name:      "empty namespace",
			topology:  topology,
			encoder:   StringKeyEncoder(),
			wantError: "xpg/topology/shard/resolver: rendezvous namespace must not be empty",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NewRendezvous(
				test.topology,
				test.namespace,
				test.encoder,
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

func TestNewRendezvousTreatsNamespaceAsOpaqueNonEmptyString(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")

	for _, namespace := range []string{"users", " users ", " "} {
		resolver, err := NewRendezvous(topology, namespace, StringKeyEncoder())
		if err != nil {
			t.Fatalf("NewRendezvous(%q) error = %v", namespace, err)
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

func TestRendezvousResolverStablePlacementVectors(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b", "shard-c")
	resolver, err := NewRendezvous(topology, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewRendezvous() error = %v", err)
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
			t.Parallel()

			resolved, err := resolver.Resolve(test.key)
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}

			if got := resolved.ID(); got != test.want {
				t.Fatalf(
					"Resolve(%q).ID() = %q, want %q",
					test.key,
					got,
					test.want,
				)
			}
		})
	}
}

func TestRendezvousResolverPlacementDoesNotDependOnTopologyOrder(t *testing.T) {
	t.Parallel()

	first := newTestTopology(t, "shard-a", "shard-b", "shard-c")
	second := newTestTopology(t, "shard-c", "shard-a", "shard-b")

	firstResolver, err := NewRendezvous(first, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewRendezvous(first) error = %v", err)
	}

	secondResolver, err := NewRendezvous(second, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewRendezvous(second) error = %v", err)
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

func TestRendezvousResolverAddingShardOnlyMovesKeysToNewShard(t *testing.T) {
	t.Parallel()

	before := newTestTopology(t, "shard-a", "shard-b")
	after := newTestTopology(t, "shard-a", "shard-b", "shard-c")

	beforeResolver, err := NewRendezvous(before, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewRendezvous(before) error = %v", err)
	}

	afterResolver, err := NewRendezvous(after, "users", StringKeyEncoder())
	if err != nil {
		t.Fatalf("NewRendezvous(after) error = %v", err)
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

func TestRendezvousResolverWrapsEncoderError(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	sentinel := errors.New("encode failed")

	resolver, err := NewRendezvous(
		topology,
		"users",
		KeyEncoderFunc[string](func(string) ([]byte, error) {
			return nil, sentinel
		}),
	)
	if err != nil {
		t.Fatalf("NewRendezvous() error = %v", err)
	}

	_, err = resolver.Resolve("alice")
	if !errors.Is(err, sentinel) {
		t.Fatalf("Resolve() error = %v, want wrapped sentinel", err)
	}
}

func TestRendezvousResolverUninitialized(t *testing.T) {
	t.Parallel()

	var resolver *RendezvousResolver[string]

	_, err := resolver.Resolve("alice")
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/shard/resolver: rendezvous resolver is not initialized"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}
