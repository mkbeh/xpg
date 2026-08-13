package cache

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type Cache[V any] struct {
	name string

	store  *storage[V]
	states []cacheState
	stats  *statsCollector

	metrics   MetricsRegistration
	closeOnce sync.Once

	ttl         time.Duration
	jitter      time.Duration
	negativeTTL time.Duration
}

type cacheState struct {
	mu sync.RWMutex

	generation uint64
	group      *singleflight.Group
}

type invalidationTarget struct {
	key   string
	index int
}

func New[V any](config Config) (*Cache[V], error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	store := newStorage[V](config.MaxEntries, config.Segments)

	cache := &Cache[V]{
		name: config.Name,

		store:  store,
		states: newCacheStates(len(store.segments)),
		stats:  newStatsCollector(len(store.segments)),

		ttl:         config.TTL,
		jitter:      config.Jitter,
		negativeTTL: config.NegativeTTL,
	}

	if err := cache.registerMetrics(config.Metrics); err != nil {
		return nil, fmt.Errorf("xpg/cache: register metrics: %w", err)
	}

	return cache, nil
}

func (cache *Cache[V]) Name() string {
	if cache == nil {
		return ""
	}

	return cache.name
}

func (cache *Cache[V]) Close() {
	if cache == nil {
		return
	}

	cache.closeOnce.Do(
		func() {
			if cache.metrics != nil {
				cache.metrics.Close()
			}
		},
	)
}

func (cache *Cache[V]) GetOrLoad(
	ctx context.Context,
	key string,
	loader Loader[V],
) (V, bool, error) {
	var zero V

	if !cache.initialized() {
		return zero, false, errors.New("xpg/cache: cache is not initialized")
	}

	if loader == nil {
		return zero, false, errors.New("xpg/cache: loader is nil")
	}

	index := cache.store.segmentIndex(key)
	stats := cache.stats.shard(index)

	if cached, ok := cache.store.lookupAt(index, key, time.Now(), stats); ok {
		return cached.value, cached.found, nil
	}

	state := &cache.states[index]

	// Registration and generation selection must be atomic with respect to
	// invalidation for this state segment. DoChan only registers or starts the
	// shared call; the loader itself executes outside state.mu.
	state.mu.RLock()

	generation := state.generation
	group := state.group

	resultChannel := group.DoChan(
		key,
		func() (any, error) {
			// Another caller may have populated the cache between the initial
			// lookup and this call becoming the singleflight owner.
			if cached, ok := cache.store.getAt(index, key, time.Now(), cache.stats.shard(index)); ok {
				return loadResult[V](cached), nil
			}

			startedAt := time.Now()

			value, found, err := loader(ctx)

			finishedAt := time.Now()

			cache.stats.recordLoad(index, found, err, finishedAt.Sub(startedAt))

			if err != nil {
				return nil, err
			}

			if !found {
				var zeroValue V

				value = zeroValue
			}

			loaded := loadResult[V]{
				value: value,
				found: found,
			}

			// Generation validation and publication are atomic with respect to
			// invalidation for this state segment. A pre-invalidation load may
			// still return to callers that already joined it, but cannot
			// repopulate the cache after the barrier.
			publishState := &cache.states[index]

			publishState.mu.RLock()

			if publishState.generation == generation {
				cache.storeLoaded(index, key, loaded, finishedAt)
			}

			publishState.mu.RUnlock()

			return loaded, nil
		},
	)

	state.mu.RUnlock()

	select {
	case <-ctx.Done():
		return zero, false, ctx.Err()

	case result := <-resultChannel:
		if result.Shared {
			cache.stats.recordShared(index)
		}

		if result.Err != nil {
			return zero, false, result.Err
		}

		loaded, ok := result.Val.(loadResult[V])
		if !ok {
			return zero, false, errors.New("xpg/cache: unexpected singleflight result type")
		}

		return loaded.value, loaded.found, nil
	}
}

func (cache *Cache[V]) Invalidate(keys ...string) {
	if !cache.initialized() ||
		len(keys) == 0 {
		return
	}

	if len(keys) == 1 {
		index := cache.store.segmentIndex(keys[0])

		if cache.invalidateOne(index, keys[0]) {
			cache.stats.recordKeyInvalidation(index, 1)
		}

		return
	}

	targets := make([]invalidationTarget, len(keys))

	for index, key := range keys {
		targets[index] = invalidationTarget{
			key:   key,
			index: cache.store.segmentIndex(key),
		}
	}

	// Keep the statistics shard based on the caller's first key rather than
	// the sorted target order. The shard records the total number of resident
	// entries actually removed by this batch.
	statsIndex := targets[0].index

	// Every invalidation path acquires state locks in ascending segment order.
	// This keeps multi-key invalidation and InvalidateAll deadlock-free.
	slices.SortFunc(
		targets,
		func(left, right invalidationTarget) int {
			return cmp.Compare(left.index, right.index)
		},
	)

	previous := -1

	for _, target := range targets {
		if target.index == previous {
			continue
		}

		cache.states[target.index].mu.Lock()

		previous = target.index
	}

	// Once every affected state is locked, advance each generation. Loads that
	// registered before this barrier may finish for existing waiters, but they
	// cannot publish into any affected segment afterward.
	previous = -1

	for _, target := range targets {
		if target.index == previous {
			continue
		}

		cache.states[target.index].generation++

		previous = target.index
	}

	var invalidated int64

	for _, target := range targets {
		state := &cache.states[target.index]

		state.group.Forget(target.key)

		if cache.store.deleteAt(target.index, target.key) {
			invalidated++
		}
	}

	previous = -1

	for index := len(targets) - 1; index >= 0; index-- {
		target := targets[index]

		if target.index == previous {
			continue
		}

		cache.states[target.index].mu.Unlock()

		previous = target.index
	}

	cache.stats.recordKeyInvalidation(statsIndex, invalidated)
}

func (cache *Cache[V]) InvalidateAll() {
	if !cache.initialized() {
		return
	}

	for index := range cache.states {
		cache.states[index].mu.Lock()
	}

	for index := range cache.states {
		state := &cache.states[index]

		state.generation++
		state.group = &singleflight.Group{}
	}

	invalidated := cache.store.deleteAll()

	for index := len(cache.states) - 1; index >= 0; index-- {
		cache.states[index].mu.Unlock()
	}

	cache.stats.recordAllInvalidation(invalidated)
}

func (cache *Cache[V]) invalidateOne(
	index int,
	key string,
) bool {
	state := &cache.states[index]

	state.mu.Lock()

	state.generation++
	state.group.Forget(key)

	removed := cache.store.deleteAt(index, key)

	state.mu.Unlock()

	return removed
}

func (cache *Cache[V]) storeLoaded(
	index int,
	key string,
	loaded loadResult[V],
	now time.Time,
) {
	switch {
	case loaded.found:
		cache.store.setAt(
			index,
			key,
			cachedValue[V]{
				value: loaded.value,
				found: true,
			},
			now.Add(cache.effectiveTTL()),
			cache.stats.shard(index),
		)

	case cache.negativeTTL > 0:
		cache.store.setAt(
			index,
			key,
			cachedValue[V]{
				found: false,
			},
			now.Add(cache.negativeTTL),
			cache.stats.shard(index),
		)
	}
}

func (cache *Cache[V]) effectiveTTL() time.Duration {
	if cache.jitter == 0 {
		return cache.ttl
	}

	return cache.ttl + time.Duration(
		rand.Int64N(int64(cache.jitter)),
	)
}

func (cache *Cache[V]) registerMetrics(metrics Metrics) error {
	if metrics == nil {
		return nil
	}

	registration, err := metrics.RegisterCache(cache)
	if err != nil {
		return err
	}

	cache.metrics = registration

	return nil
}

func (cache *Cache[V]) initialized() bool {
	return cache != nil &&
		cache.store != nil &&
		cache.stats != nil &&
		len(cache.states) ==
			len(cache.store.segments) &&
		len(cache.stats.shards) ==
			len(cache.store.segments)
}

func newCacheStates(count int) []cacheState {
	states := make([]cacheState, count)

	for index := range states {
		states[index].group = &singleflight.Group{}
	}

	return states
}
