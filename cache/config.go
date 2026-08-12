package cache

import (
	"errors"
	"strings"
	"time"
)

const (
	maxDuration       = time.Duration(1<<63 - 1)
	defaultMaxEntries = 10_000
)

// Config configures a bounded in-process cache.
type Config struct {
	// Name identifies the cache for diagnostics and observability.
	Name string

	// MaxEntries is the maximum total entry budget of the cache.
	//
	// The budget is distributed across storage segments. Because each segment
	// enforces its capacity independently, the number of simultaneously resident
	// entries may be slightly lower than MaxEntries depending on key distribution.
	//
	// Zero uses the default.
	MaxEntries int

	// Segments is the number of independent storage segments used to reduce lock
	// contention.
	//
	// Zero uses the default segment count.
	Segments int

	// TTL is the lifetime of positive entries.
	TTL time.Duration

	// Jitter adds a random duration in [0, Jitter) to positive-entry TTLs.
	// It can be used to spread expiration of entries loaded around the same time.
	Jitter time.Duration

	// NegativeTTL is the lifetime of cached negative results.
	// Zero disables negative caching.
	NegativeTTL time.Duration
}

func (config Config) validate() error {
	name := strings.TrimSpace(config.Name)

	if name == "" {
		return errors.New(
			"xpg/cache: name must not be blank",
		)
	}

	if name != config.Name {
		return errors.New(
			"xpg/cache: name must not contain surrounding whitespace",
		)
	}

	if config.MaxEntries < 0 {
		return errors.New(
			"xpg/cache: max entries must not be negative",
		)
	}

	if config.Segments < 0 {
		return errors.New(
			"xpg/cache: segments must not be negative",
		)
	}

	if config.TTL <= 0 {
		return errors.New(
			"xpg/cache: ttl must be greater than zero",
		)
	}

	if config.Jitter < 0 {
		return errors.New(
			"xpg/cache: jitter must not be negative",
		)
	}

	if config.NegativeTTL < 0 {
		return errors.New(
			"xpg/cache: negative ttl must not be negative",
		)
	}

	if config.Jitter > maxDuration-config.TTL {
		return errors.New(
			"xpg/cache: ttl and jitter overflow time.Duration",
		)
	}

	return nil
}
