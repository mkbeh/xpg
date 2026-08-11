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
	errs := make([]error, 0, len(results))

	for _, result := range results {
		if result.Err == nil {
			continue
		}

		errs = append(
			errs,
			fmt.Errorf(
				"xpg/shard: shard %q callback: %w",
				result.ShardID,
				result.Err,
			),
		)
	}

	return errors.Join(errs...)
}

// ForEachShard invokes fn for each shard with at most concurrency callbacks
// running at once. Results are returned in topology registration order.
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
		return nil, errors.New("xpg/shard: topology is nil or empty")
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

	for range workerCount {
		workers.Go(func() {
			for index := range jobs {
				if err := ctx.Err(); err != nil {
					results[index].Err = err
					continue
				}

				results[index].Err = fn(ctx, t.shards[index])
			}
		})
	}

	nextIndex := 0

schedule:
	for nextIndex < len(t.shards) {
		// Check cancellation before entering select so that a ready worker does
		// not repeatedly win against an already canceled context.
		if ctx.Err() != nil {
			break
		}

		select {
		case jobs <- nextIndex:
			nextIndex++

		case <-ctx.Done():
			break schedule
		}
	}

	close(jobs)
	workers.Wait()

	if err := ctx.Err(); err != nil {
		// Every index before nextIndex was handed to exactly one worker.
		// Everything from nextIndex onward was never scheduled.
		for index := nextIndex; index < len(results); index++ {
			results[index].Err = err
		}
	}

	return results, nil
}
