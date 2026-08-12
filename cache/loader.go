package cache

import "context"

// Loader loads one value.
//
// found=false represents a successful negative result. Negative results are
// cached only when Config.NegativeTTL is greater than zero.
//
// Loader errors are never cached.
type Loader[V any] func(ctx context.Context) (value V, found bool, err error)

type loadResult[V any] struct {
	value V
	found bool
}

type cachedValue[V any] struct {
	value V
	found bool
}
