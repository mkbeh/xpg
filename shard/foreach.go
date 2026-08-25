package shard

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// ForEachShardResult contains the result associated with one shard.
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
				"xpg/shard: shard %q: %w",
				result.ShardID,
				result.Err,
			),
		)
	}

	return errors.Join(errs...)
}

// ForEachShard invokes fn across the topology with at most concurrency
// callbacks running at once. Results are returned in topology registration
// order; callback execution order is not guaranteed.
//
// Callback failures and context cancellation are stored in the corresponding
// results and can be joined with ForEachShardResults.Err. The returned error is
// reserved for invalid invocation arguments.
//
// Once context cancellation is observed, callbacks that have not started are
// skipped and their results contain ctx.Err(). Callbacks already running are
// responsible for observing ctx. ForEachShard waits for all started callbacks
// to finish before returning.
func (t *Topology) ForEachShard(
	ctx context.Context,
	concurrency int,
	fn func(context.Context, Shard) error,
) (ForEachShardResults, error) {
	if t == nil {
		return nil, errors.New("xpg/shard: topology is nil")
	}

	if len(t.shards) == 0 {
		return nil, errors.New("xpg/shard: topology is empty")
	}

	if concurrency <= 0 {
		return nil, errors.New("xpg/shard: concurrency must be positive")
	}

	if fn == nil {
		return nil, errors.New("xpg/shard: callback is nil")
	}

	results := make(ForEachShardResults, len(t.shards))
	for index, shard := range t.shards {
		results[index].ShardID = shard.ID()
	}

	workerCount := min(concurrency, len(t.shards))
	jobs := make(chan int)

	var workers sync.WaitGroup
	workers.Add(workerCount)

	for range workerCount {
		go func() {
			defer workers.Done()

			for index := range jobs {
				// An index may have been scheduled immediately before context
				// cancellation. Skip callbacks that have not started yet.
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
		// The explicit context check above prevents scheduling new work after
		// cancellation has already been observed. The select still handles
		// cancellation that happens while waiting for a worker.
		select {
		case jobs <- nextIndex:
			nextIndex++
		case <-ctx.Done():
		}
	}

	close(jobs)
	workers.Wait()

	// Workers own results for scheduled indexes [0, nextIndex). After all
	// workers finish, remaining indexes can be marked canceled without races.
	if err := ctx.Err(); err != nil {
		for index := nextIndex; index < len(results); index++ {
			results[index].Err = err
		}
	}

	return results, nil
}
