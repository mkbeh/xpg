package cluster

import (
	"context"
	"errors"
	"maps"
	"sync/atomic"
)

// ReplicaInfo contains immutable metadata captured from one replica pool when
// the Cluster is created.
type ReplicaInfo struct {
	name   string
	labels map[string]string
}

// Name returns the stable logical pool name.
func (info ReplicaInfo) Name() string {
	return info.name
}

// Label returns one replica label without allocating a copy of all labels.
func (info ReplicaInfo) Label(key string) (string, bool) {
	value, ok := info.labels[key]
	return value, ok
}

// Labels returns a defensive copy of replica labels.
func (info ReplicaInfo) Labels() map[string]string {
	return cloneLabels(info.labels)
}

// ReplicaSet provides read-only access to replica metadata.
type ReplicaSet interface {
	Len() int
	At(index int) ReplicaInfo
}

type replicaMetadata []ReplicaInfo

func (replicas replicaMetadata) Len() int {
	return len(replicas)
}

func (replicas replicaMetadata) At(index int) ReplicaInfo {
	return replicas[index]
}

// ReplicaSelector selects one replica index from the supplied metadata.
// Implementations used by concurrent callers must be concurrency-safe.
type ReplicaSelector interface {
	Select(ctx context.Context, replicas ReplicaSet) (index int, err error)
}

// ReplicaSelectorFunc adapts a function to ReplicaSelector.
type ReplicaSelectorFunc func(context.Context, ReplicaSet) (int, error)

// Select calls selector.
func (selector ReplicaSelectorFunc) Select(ctx context.Context, replicas ReplicaSet) (int, error) {
	if selector == nil {
		return -1, errors.New("xpg/cluster: replica selector function is nil")
	}

	return selector(ctx, replicas)
}

type roundRobinSelector struct {
	next atomic.Uint64
}

// RoundRobinSelector returns a concurrency-safe selector that distributes
// selections across replicas in registration order.
func RoundRobinSelector() ReplicaSelector {
	return &roundRobinSelector{}
}

func (selector *roundRobinSelector) Select(_ context.Context, replicas ReplicaSet) (int, error) {
	length := replicas.Len()

	if length == 0 {
		return -1, ErrNoReplica
	}

	if length == 1 {
		return 0, nil
	}

	next := selector.next.Add(1) - 1

	return int(next % uint64(length)), nil
}

func cloneLabels(labels map[string]string) map[string]string {
	if len(labels) == 0 {
		return nil
	}

	cloned := make(map[string]string, len(labels))
	maps.Copy(cloned, labels)

	return cloned
}
