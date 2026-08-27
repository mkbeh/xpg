package shard

import (
	"errors"
	"fmt"
	"slices"
	"testing"
)

func TestSameShardReturnsColocatedShard(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	shardA := topology.At(0)
	shardB := topology.At(1)

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		if key < 100 {
			return shardA, nil
		}

		return shardB, nil
	})

	resolved, err := SameShard(resolver, 1, 2, 3)
	if err != nil {
		t.Fatalf("SameShard() error = %v", err)
	}

	if got, want := resolved.ID(), ID("shard-a"); got != want {
		t.Fatalf("SameShard().ID() = %q, want %q", got, want)
	}
}

func TestSameShardSingleKey(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	want := topology.At(0)

	resolver := testResolverFunc[int](func(int) (Shard, error) {
		return want, nil
	})

	got, err := SameShard(resolver, 42)
	if err != nil {
		t.Fatalf("SameShard() error = %v", err)
	}

	if got.ID() != want.ID() {
		t.Fatalf("SameShard().ID() = %q, want %q", got.ID(), want.ID())
	}
}

func TestSameShardReturnsErrNoShard(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	resolved := topology.At(0)

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		if key == 2 {
			return Shard{}, ErrNoShard
		}

		return resolved, nil
	})

	_, err := SameShard(resolver, 1, 2)
	if !errors.Is(err, ErrNoShard) {
		t.Fatalf("error = %v, want ErrNoShard", err)
	}

	if got, want := err.Error(), "xpg/topology/shard: resolve key 1: "+ErrNoShard.Error(); got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestSameShardResolvesEachKeyOnce(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	resolved := topology.At(0)
	calls := 0

	resolver := testResolverFunc[int](func(int) (Shard, error) {
		calls++

		return resolved, nil
	})

	keys := []int{1, 2, 3, 4}

	_, err := SameShard(resolver, keys...)
	if err != nil {
		t.Fatalf("SameShard() error = %v", err)
	}

	if got, want := calls, len(keys); got != want {
		t.Fatalf("resolve calls = %d, want %d", got, want)
	}
}

func TestSameShardStopsAtFirstMismatch(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	shardA := topology.At(0)
	shardB := topology.At(1)
	calls := 0

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		calls++

		switch key {
		case 1:
			return shardA, nil
		case 2:
			return shardB, nil
		default:
			t.Fatal("resolver should stop after first mismatch")

			return Shard{}, nil
		}
	})

	_, err := SameShard(resolver, 1, 2, 3)
	if !errors.Is(err, ErrShardMismatch) {
		t.Fatalf("error = %v, want ErrShardMismatch", err)
	}

	if got, want := calls, 2; got != want {
		t.Fatalf("resolve calls = %d, want %d", got, want)
	}
}

func TestSameShardRejectsNilResolver(t *testing.T) {
	t.Parallel()

	_, err := SameShard[int](nil, 1)
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/shard: resolver is nil"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestSameShardRequiresKey(t *testing.T) {
	t.Parallel()

	resolver := testResolverFunc[int](func(int) (Shard, error) {
		t.Fatal("resolver should not be called")
		return Shard{}, nil
	})

	_, err := SameShard(resolver)
	if !errors.Is(err, ErrNoShard) {
		t.Fatalf("error = %v, want ErrNoShard", err)
	}
}

func TestSameShardWrapsFirstResolveError(t *testing.T) {
	t.Parallel()

	sentinel := errors.New("resolve failed")
	resolver := testResolverFunc[int](func(int) (Shard, error) {
		return Shard{}, sentinel
	})

	_, err := SameShard(resolver, 1)
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want wrapped sentinel", err)
	}

	if got, want := err.Error(), "xpg/topology/shard: resolve key 0: resolve failed"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestSameShardWrapsResolveErrorWithIndex(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	resolved := topology.At(0)
	sentinel := errors.New("resolve failed")

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		if key == 2 {
			return Shard{}, sentinel
		}

		return resolved, nil
	})

	_, err := SameShard(resolver, 1, 2)
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want wrapped sentinel", err)
	}

	if got, want := err.Error(), "xpg/topology/shard: resolve key 1: resolve failed"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestSameShardReturnsMismatchDetails(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	shardA := topology.At(0)
	shardB := topology.At(1)

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		if key == 3 {
			return shardB, nil
		}

		return shardA, nil
	})

	_, err := SameShard(resolver, 1, 2, 3)
	if !errors.Is(err, ErrShardMismatch) {
		t.Fatalf("error = %v, want ErrShardMismatch", err)
	}

	var mismatch *MismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("error = %T, want *MismatchError", err)
	}

	if mismatch.Expected != "shard-a" || mismatch.Actual != "shard-b" || mismatch.Index != 2 {
		t.Fatalf("mismatch = %+v", mismatch)
	}
}

func TestGroupByShardPreservesGroupAndKeyOrder(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	shardA := topology.At(0)
	shardB := topology.At(1)

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		if key < 100 {
			return shardA, nil
		}

		return shardB, nil
	})

	groups, err := GroupByShard(resolver, []int{142, 42, 143, 43})
	if err != nil {
		t.Fatalf("GroupByShard() error = %v", err)
	}

	if got, want := len(groups), 2; got != want {
		t.Fatalf("len(groups) = %d, want %d", got, want)
	}

	if got, want := groups[0].Shard.ID(), ID("shard-b"); got != want {
		t.Fatalf("groups[0].Shard.ID() = %q, want %q", got, want)
	}
	if got, want := groups[0].Keys, []int{142, 143}; !slices.Equal(got, want) {
		t.Fatalf("groups[0].Keys = %v, want %v", got, want)
	}

	if got, want := groups[1].Shard.ID(), ID("shard-a"); got != want {
		t.Fatalf("groups[1].Shard.ID() = %q, want %q", got, want)
	}
	if got, want := groups[1].Keys, []int{42, 43}; !slices.Equal(got, want) {
		t.Fatalf("groups[1].Keys = %v, want %v", got, want)
	}
}

func TestGroupByShardResolvesEachKeyOnce(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	resolved := topology.At(0)
	calls := 0

	resolver := testResolverFunc[int](func(int) (Shard, error) {
		calls++
		return resolved, nil
	})

	keys := []int{1, 2, 3, 4}
	groups, err := GroupByShard(resolver, keys)
	if err != nil {
		t.Fatalf("GroupByShard() error = %v", err)
	}

	if got, want := calls, len(keys); got != want {
		t.Fatalf("resolve calls = %d, want %d", got, want)
	}

	if got, want := len(groups), 1; got != want {
		t.Fatalf("len(groups) = %d, want %d", got, want)
	}
}

func TestGroupByShardRejectsNilResolver(t *testing.T) {
	t.Parallel()

	_, err := GroupByShard[int](nil, []int{1})
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/shard: resolver is nil"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestGroupByShardReturnsErrNoShard(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	resolved := topology.At(0)

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		if key == 2 {
			return Shard{}, ErrNoShard
		}

		return resolved, nil
	})

	groups, err := GroupByShard(resolver, []int{1, 2, 3})
	if !errors.Is(err, ErrNoShard) {
		t.Fatalf("error = %v, want ErrNoShard", err)
	}

	if groups != nil {
		t.Fatalf("groups = %+v, want nil", groups)
	}
}

func TestGroupByShardEmptyKeys(t *testing.T) {
	t.Parallel()

	resolver := testResolverFunc[int](func(int) (Shard, error) {
		t.Fatal("resolver should not be called")
		return Shard{}, nil
	})

	groups, err := GroupByShard(resolver, nil)
	if err != nil {
		t.Fatalf("GroupByShard() error = %v", err)
	}

	if got := len(groups); got != 0 {
		t.Fatalf("len(groups) = %d, want 0", got)
	}
}

func TestGroupByShardWrapsResolveErrorWithIndex(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	resolved := topology.At(0)
	sentinel := errors.New("resolve failed")

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		if key == 3 {
			return Shard{}, sentinel
		}

		return resolved, nil
	})

	_, err := GroupByShard(resolver, []int{1, 2, 3})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want wrapped sentinel", err)
	}

	if got, want := err.Error(), "xpg/topology/shard: resolve key 2: resolve failed"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestPartitionByShardGroupsAndCollectsUnresolved(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a", "shard-b")
	shardA := topology.At(0)
	shardB := topology.At(1)

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		switch {
		case key < 0 || key >= 200:
			return Shard{}, ErrNoShard
		case key < 100:
			return shardA, nil
		default:
			return shardB, nil
		}
	})

	partition, err := PartitionByShard(resolver, []int{142, 250, 42, 250, -1, 143, 142, 43, 42})
	if err != nil {
		t.Fatalf("PartitionByShard() error = %v", err)
	}

	if got, want := len(partition.Groups), 2; got != want {
		t.Fatalf("len(Groups) = %d, want %d", got, want)
	}

	if got, want := partition.Groups[0].Shard.ID(), ID("shard-b"); got != want {
		t.Fatalf("Groups[0].Shard.ID() = %q, want %q", got, want)
	}
	if got, want := partition.Groups[0].Keys, []int{142, 143, 142}; !slices.Equal(got, want) {
		t.Fatalf("Groups[0].Keys = %v, want %v", got, want)
	}

	if got, want := partition.Groups[1].Shard.ID(), ID("shard-a"); got != want {
		t.Fatalf("Groups[1].Shard.ID() = %q, want %q", got, want)
	}
	if got, want := partition.Groups[1].Keys, []int{42, 43, 42}; !slices.Equal(got, want) {
		t.Fatalf("Groups[1].Keys = %v, want %v", got, want)
	}

	if got, want := partition.Unresolved, []int{250, 250, -1}; !slices.Equal(got, want) {
		t.Fatalf("Unresolved = %v, want %v", got, want)
	}
}

func TestPartitionByShardTreatsWrappedErrNoShardAsUnresolved(t *testing.T) {
	t.Parallel()

	resolver := testResolverFunc[int](func(int) (Shard, error) {
		return Shard{}, fmt.Errorf("outside keyspace: %w", ErrNoShard)
	})

	partition, err := PartitionByShard(resolver, []int{1, 2})
	if err != nil {
		t.Fatalf("PartitionByShard() error = %v", err)
	}

	if got := len(partition.Groups); got != 0 {
		t.Fatalf("len(Groups) = %d, want 0", got)
	}
	if got, want := partition.Unresolved, []int{1, 2}; !slices.Equal(got, want) {
		t.Fatalf("Unresolved = %v, want %v", got, want)
	}
}

func TestPartitionByShardResolvesEachKeyOnce(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	resolved := topology.At(0)
	calls := 0

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		calls++
		if key == 2 {
			return Shard{}, ErrNoShard
		}

		return resolved, nil
	})

	keys := []int{1, 2, 3, 4}
	partition, err := PartitionByShard(resolver, keys)
	if err != nil {
		t.Fatalf("PartitionByShard() error = %v", err)
	}

	if got, want := calls, len(keys); got != want {
		t.Fatalf("resolve calls = %d, want %d", got, want)
	}
	if got, want := partition.Unresolved, []int{2}; !slices.Equal(got, want) {
		t.Fatalf("Unresolved = %v, want %v", got, want)
	}
}

func TestPartitionByShardRejectsNilResolver(t *testing.T) {
	t.Parallel()

	_, err := PartitionByShard[int](nil, []int{1})
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/topology/shard: resolver is nil"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestPartitionByShardEmptyKeys(t *testing.T) {
	t.Parallel()

	resolver := testResolverFunc[int](func(int) (Shard, error) {
		t.Fatal("resolver should not be called")
		return Shard{}, nil
	})

	partition, err := PartitionByShard(resolver, nil)
	if err != nil {
		t.Fatalf("PartitionByShard() error = %v", err)
	}

	if len(partition.Groups) != 0 || len(partition.Unresolved) != 0 {
		t.Fatalf("PartitionByShard() = %+v, want empty partition", partition)
	}
}

func TestPartitionByShardWrapsNonRoutingErrorWithIndex(t *testing.T) {
	t.Parallel()

	topology := newTestTopology(t, "shard-a")
	resolved := topology.At(0)
	sentinel := errors.New("resolve failed")

	calls := 0

	resolver := testResolverFunc[int](func(key int) (Shard, error) {
		calls++

		switch key {
		case 2:
			return Shard{}, ErrNoShard
		case 3:
			return Shard{}, sentinel
		default:
			return resolved, nil
		}
	})

	partition, err := PartitionByShard(resolver, []int{1, 2, 3, 4})
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v, want wrapped sentinel", err)
	}

	if got, want := err.Error(), "xpg/topology/shard: resolve key 2: resolve failed"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}

	if len(partition.Groups) != 0 || len(partition.Unresolved) != 0 {
		t.Fatalf("PartitionByShard() = %+v, want zero partition on error", partition)
	}

	if got, want := calls, 3; got != want {
		t.Fatalf("resolve calls = %d, want %d", got, want)
	}
}
