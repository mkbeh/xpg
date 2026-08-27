package shard

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ForEachShardResult contains the result of one shard callback invocation.
type ForEachShardResult struct {
	ShardID ID
	Err     error
}

// ForEachShardResults contains results in topology registration order.
type ForEachShardResults []ForEachShardResult

// Err returns all shard failures joined in registration order.
func (results ForEachShardResults) Err() error {
	var errs []error

	for _, result := range results {
		if result.Err == nil {
			continue
		}

		errs = append(
			errs,
			fmt.Errorf(
				"xpg/topology/shard: shard %q callback: %w",
				result.ShardID,
				result.Err,
			),
		)
	}

	return errors.Join(errs...)
}

// ForEachShard invokes fn for each shard with at most concurrency callbacks
// running at once. Results are returned in topology registration order. The
// returned error joins all per-shard failures and is equivalent to results.Err().
//
// Once context cancellation is observed, callbacks that have not started are
// skipped and their results contain ctx.Err(). Callbacks already running are
// responsible for observing ctx.
func (t *Topology) ForEachShard(
	ctx context.Context,
	concurrency int,
	fn func(context.Context, Shard) error,
) (ForEachShardResults, error) {
	if t == nil || len(t.shards) == 0 {
		return nil, errors.New("xpg/topology/shard: topology is nil or empty")
	}

	if concurrency <= 0 {
		return nil, errors.New("xpg/topology/shard: concurrency must be positive")
	}

	if fn == nil {
		return nil, errors.New("xpg/topology/shard: callback is nil")
	}

	results := make(ForEachShardResults, len(t.shards))
	for index, current := range t.shards {
		results[index].ShardID = current.ID()
	}

	if err := ctx.Err(); err != nil {
		for index := range results {
			results[index].Err = err
		}

		return results, results.Err()
	}

	workerCount := min(concurrency, len(t.shards))
	jobs := make(chan int)

	var workers sync.WaitGroup
	workers.Add(workerCount)

	for range workerCount {
		go func() {
			defer workers.Done()

			for index := range jobs {
				if err := ctx.Err(); err != nil {
					results[index].Err = err
					continue
				}

				results[index].Err = fn(ctx, t.shards[index])
			}
		}()
	}

	nextIndex := 0

	for nextIndex < len(t.shards) && ctx.Err() == nil {
		select {
		case jobs <- nextIndex:
			nextIndex++
		case <-ctx.Done():
		}
	}

	close(jobs)
	workers.Wait()

	if err := ctx.Err(); err != nil {
		for index := nextIndex; index < len(results); index++ {
			results[index].Err = err
		}
	}

	return results, results.Err()
}
