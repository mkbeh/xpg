package cluster

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/mkbeh/xpg"
)

func TestInPrimaryTxNilCluster(t *testing.T) {
	t.Parallel()

	var cluster *Cluster
	called := false

	err := cluster.InPrimaryTx(
		context.Background(),
		pgx.TxOptions{},
		func(context.Context, pgx.Tx) error {
			called = true
			return nil
		},
	)
	if err == nil {
		t.Fatal("expected error")
	}

	if got, want := err.Error(), "xpg/cluster: cluster is nil"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}

	if called {
		t.Fatal("transaction callback was called")
	}
}

func TestInPrimaryTxWithoutPrimary(t *testing.T) {
	t.Parallel()

	replica := newTestPool(t, "replica", nil)
	cluster := newTestCluster(t, Config{
		Replicas: []*xpg.Pool{replica},
	})

	called := false
	err := cluster.InPrimaryTx(
		context.Background(),
		pgx.TxOptions{},
		func(context.Context, pgx.Tx) error {
			called = true
			return nil
		},
	)

	if !errors.Is(err, ErrNoPrimary) {
		t.Fatalf("InPrimaryTx() error = %v, want ErrNoPrimary", err)
	}

	if called {
		t.Fatal("transaction callback was called")
	}
}

func TestInPrimaryTxDelegatesToPool(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	cluster := newTestCluster(t, Config{Primary: primary})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := cluster.InPrimaryTx(
		ctx,
		pgx.TxOptions{},
		func(context.Context, pgx.Tx) error {
			called = true
			return nil
		},
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if called {
		t.Fatal("transaction callback was called")
	}
}

func TestInReadTxRoutingError(t *testing.T) {
	t.Parallel()

	primary := newTestPool(t, "primary", nil)
	cluster := newTestCluster(t, Config{Primary: primary})

	called := false
	err := cluster.InReadTx(
		context.Background(),
		ReadReplicaRequired,
		ReadTxOptions{},
		func(context.Context, pgx.Tx) error {
			called = true
			return nil
		},
	)

	if !errors.Is(err, ErrNoReplica) {
		t.Fatalf("InReadTx() error = %v, want ErrNoReplica", err)
	}

	if called {
		t.Fatal("transaction callback was called")
	}
}

func TestInReadTxDelegatesToResolvedPool(t *testing.T) {
	t.Parallel()

	replica := newTestPool(t, "replica", nil)
	cluster := newTestCluster(t, Config{
		Replicas: []*xpg.Pool{replica},
	})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	called := false
	err := cluster.InReadTx(
		ctx,
		ReadReplicaRequired,
		ReadTxOptions{
			IsoLevel:       pgx.Serializable,
			DeferrableMode: pgx.Deferrable,
		},
		func(context.Context, pgx.Tx) error {
			called = true
			return nil
		},
	)

	if err == nil {
		t.Fatal("expected error")
	}

	if called {
		t.Fatal("transaction callback was called")
	}
}
