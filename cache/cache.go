package cache

import (
	"cmp"
	"context"
	"errors"
	"math/rand/v2"
	"slices"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// Cache is a concurrency-safe bounded in-process cache.
//
// Cache must be created with New and must not be copied after first use.
type Cache[V any] struct {
	name string

	store  *storage[V]
	states []cacheState

	ttl         time.Duration
	jitter      time.Duration
	negativeTTL time.Duration
}

type cacheState struct {
	// mu serializes invalidation with singleflight registration and cache
	// publication for keys routed to this state segment.
	mu sync.RWMutex

	generation uint64
	group      *singleflight.Group
}

type invalidationTarget struct {
	key   string
	index int
}

// New creates a bounded in-process cache.
func New[V any](config Config) (*Cache[V], error) {
	if err := config.validate(); err != nil {
		return nil, err
	}

	store := newStorage[V](config.MaxEntries, config.Segments)

	return &Cache[V]{
		name:        config.Name,
		store:       store,
		states:      newCacheStates(len(store.segments)),
		ttl:         config.TTL,
		jitter:      config.Jitter,
		negativeTTL: config.NegativeTTL,
	}, nil
}

// Name returns the configured cache name.
func (cache *Cache[V]) Name() string {
	if cache == nil {
		return ""
	}

	return cache.name
}

// GetOrLoad returns a cached value or executes loader on a cache miss.
//
// Concurrent misses for the same key share the loader started by the first
// caller. Each caller may stop waiting through its own context.
//
// The context of the caller that starts the shared load controls the loader.
//
// found=false represents a negative result. Negative results are cached only
// when Config.NegativeTTL is greater than zero. Loader errors are never cached.
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

	if cached, ok := cache.store.getAt(index, key, time.Now()); ok {
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
			if cached, ok := cache.store.getAt(index, key, time.Now()); ok {
				return loadResult[V]{
					value: cached.value,
					found: cached.found,
				}, nil
			}

			value, found, err := loader(ctx)
			if err != nil {
				return nil, err
			}

			if !found {
				value = zero
			}

			loaded := loadResult[V]{
				value: value,
				found: found,
			}

			// Generation validation and publication are atomic with respect to
			// invalidation for this state segment. A pre-invalidation load may
			// still return to callers that already joined it, but cannot
			// repopulate the cache after the barrier.
			state.mu.RLock()

			if state.generation == generation {
				cache.storeLoaded(index, key, loaded)
			}

			state.mu.RUnlock()

			return loaded, nil
		},
	)

	state.mu.RUnlock()

	select {
	case <-ctx.Done():
		return zero, false, ctx.Err()

	case result := <-resultChannel:
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

// Invalidate removes keys from the cache and prevents loads registered before
// the invalidation from repopulating them.
//
// Already running loaders are not canceled. Callers already waiting for such a
// loader may still receive its result.
func (cache *Cache[V]) Invalidate(keys ...string) {
	if !cache.initialized() || len(keys) == 0 {
		return
	}

	// Keep the common single-key path allocation-free.
	if len(keys) == 1 {
		cache.invalidateOne(keys[0])
		return
	}

	targets := make([]invalidationTarget, len(keys))

	for index, key := range keys {
		targets[index] = invalidationTarget{
			key:   key,
			index: cache.store.segmentIndex(key),
		}
	}

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

	for _, target := range targets {
		state := &cache.states[target.index]

		// Forget ensures callers registered after this invalidation cannot join
		// the pre-invalidation flight for key.
		state.group.Forget(target.key)
		cache.store.deleteAt(target.index, target.key)
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
}

// InvalidateAll removes every cached entry and detaches future callers from all
// currently running singleflight calls.
//
// Existing loaders continue for callers that already joined them, but their
// results cannot repopulate the cache.
func (cache *Cache[V]) InvalidateAll() {
	if !cache.initialized() {
		return
	}

	// Lock every state in a stable order. InvalidateAll is intentionally a
	// cache-wide barrier and is expected to be rare compared with key-scoped
	// invalidation.
	for index := range cache.states {
		cache.states[index].mu.Lock()
	}

	for index := range cache.states {
		state := &cache.states[index]

		state.generation++

		// singleflight.Group has no ForgetAll operation. Existing callers retain
		// the old group while future callers use this new group.
		state.group = &singleflight.Group{}
	}

	cache.store.deleteAll()

	for index := len(cache.states) - 1; index >= 0; index-- {
		cache.states[index].mu.Unlock()
	}
}

func (cache *Cache[V]) invalidateOne(
	key string,
) {
	index := cache.store.segmentIndex(key)
	state := &cache.states[index]

	state.mu.Lock()

	state.generation++
	state.group.Forget(key)
	cache.store.deleteAt(index, key)

	state.mu.Unlock()
}

func (cache *Cache[V]) storeLoaded(
	index int,
	key string,
	loaded loadResult[V],
) {
	now := time.Now()

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
		)

	case cache.negativeTTL > 0:
		cache.store.setAt(
			index,
			key,
			cachedValue[V]{
				found: false,
			},
			now.Add(cache.negativeTTL),
		)
	}
}

func (cache *Cache[V]) effectiveTTL() time.Duration {
	if cache.jitter == 0 {
		return cache.ttl
	}

	return cache.ttl + time.Duration(
		rand.Int64N(
			int64(cache.jitter),
		),
	)
}

func (cache *Cache[V]) initialized() bool {
	return cache != nil &&
		cache.store != nil &&
		len(cache.states) == len(cache.store.segments)
}

func newCacheStates(count int) []cacheState {
	states := make([]cacheState, count)

	for index := range states {
		states[index].group = &singleflight.Group{}
	}

	return states
}
