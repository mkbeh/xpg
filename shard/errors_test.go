package shard

import (
	"errors"
	"testing"
)

func TestUnknownShardError(t *testing.T) {
	t.Parallel()

	err := &UnknownShardError{ShardID: "missing"}

	if got, want := err.Error(), `xpg/shard: unknown shard "missing"`; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	if !errors.Is(err, ErrUnknownShard) {
		t.Fatal("errors.Is() = false, want ErrUnknownShard")
	}
}

func TestMismatchError(t *testing.T) {
	t.Parallel()

	err := &MismatchError{
		Expected: "shard-a",
		Actual:   "shard-b",
		Index:    2,
	}

	if got, want := err.Error(), `xpg/shard: key 2 resolved to shard "shard-b" instead of "shard-a"`; got != want {
		t.Fatalf("Error() = %q, want %q", got, want)
	}

	if !errors.Is(err, ErrShardMismatch) {
		t.Fatal("errors.Is() = false, want ErrShardMismatch")
	}
}
