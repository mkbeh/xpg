package xpgotel

import (
	"context"
	"errors"
	"fmt"
	"slices"

	xpgcache "github.com/mkbeh/xpg/cache"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

const (
	cacheEntryCountMetricName      = "xpg.cache.entry.count"
	cacheEntryMaxMetricName        = "xpg.cache.entry.max"
	cacheSegmentCountMetricName    = "xpg.cache.segment.count"
	cacheLookupCountMetricName     = "xpg.cache.lookup.count"
	cacheLoadCountMetricName       = "xpg.cache.load.count"
	cacheLoadTimeMetricName        = "xpg.cache.load.time"
	cacheSharedCountMetricName     = "xpg.cache.singleflight.shared.count"
	cacheInvalidationMetricName    = "xpg.cache.invalidation.count"
	cacheEvictionCountMetricName   = "xpg.cache.entry.eviction.count"
	cacheExpirationCountMetricName = "xpg.cache.entry.expiration.count"
)

const (
	cacheNameAttribute              = "xpg.cache.name"
	cacheLookupResultAttribute      = "xpg.cache.lookup.result"
	cacheLoadResultAttribute        = "xpg.cache.load.result"
	cacheInvalidationScopeAttribute = "xpg.cache.invalidation.scope"
)

const (
	cacheLookupResultHit         = "hit"
	cacheLookupResultNegativeHit = "negative_hit"
	cacheLookupResultMiss        = "miss"

	cacheLoadResultFound    = "found"
	cacheLoadResultNotFound = "not_found"
	cacheLoadResultError    = "error"

	cacheInvalidationScopeKeys = "keys"
	cacheInvalidationScopeAll  = "all"
)

var _ xpgcache.Metrics = (*Metrics)(nil)

type cacheMetricInstruments struct {
	entryCount        metric.Int64ObservableGauge
	entryMax          metric.Int64ObservableGauge
	segmentCount      metric.Int64ObservableGauge
	lookupCount       metric.Int64ObservableCounter
	loadCount         metric.Int64ObservableCounter
	loadTime          metric.Float64ObservableCounter
	sharedCount       metric.Int64ObservableCounter
	invalidationCount metric.Int64ObservableCounter
	evictionCount     metric.Int64ObservableCounter
	expirationCount   metric.Int64ObservableCounter
}

type cacheMetricAttributes struct {
	base metric.ObserveOption

	hit         metric.ObserveOption
	negativeHit metric.ObserveOption
	miss        metric.ObserveOption

	loadFound    metric.ObserveOption
	loadNotFound metric.ObserveOption
	loadError    metric.ObserveOption

	invalidateKeys metric.ObserveOption
	invalidateAll  metric.ObserveOption
}

// RegisterCache registers OpenTelemetry metrics for one cache.
func (m *Metrics) RegisterCache(cache xpgcache.StatsProvider) (xpgcache.MetricsRegistration, error) {
	if m == nil {
		return nil, errors.New("xpg/otel: metrics is nil")
	}

	if cache == nil {
		return nil, errors.New("xpg/otel: cache is nil")
	}

	name := cache.Name()
	if name == "" {
		return nil, errors.New("xpg/otel: cache name is blank")
	}

	provider := m.meterProvider
	if provider == nil {
		provider = otel.GetMeterProvider()
	}

	return registerCacheMetrics(cache, name, provider)
}

func registerCacheMetrics(
	cache xpgcache.StatsProvider,
	name string,
	provider metric.MeterProvider,
) (xpgcache.MetricsRegistration, error) {
	meter := provider.Meter(instrumentationName)

	instruments, err := newCacheMetricInstruments(meter)
	if err != nil {
		return nil, err
	}

	attributes := newCacheMetricAttributes(name)

	registration, err := meter.RegisterCallback(
		func(_ context.Context, observer metric.Observer) error {
			instruments.observe(observer, cache.Stats(), attributes)

			return nil
		},
		instruments.observables()...,
	)
	if err != nil {
		return nil, fmt.Errorf("xpg/otel: register cache metrics callback: %w", err)
	}

	return &metricsRegistration{
		registration: registration,
	}, nil
}

func (instruments cacheMetricInstruments) observe(
	observer metric.Observer,
	stats xpgcache.Stats,
	attributes cacheMetricAttributes,
) {
	observer.ObserveInt64(
		instruments.entryCount,
		stats.EntryCount,
		attributes.base,
	)
	observer.ObserveInt64(
		instruments.entryMax,
		stats.MaxEntries,
		attributes.base,
	)
	observer.ObserveInt64(
		instruments.segmentCount,
		stats.SegmentCount,
		attributes.base,
	)
	observer.ObserveInt64(
		instruments.lookupCount,
		stats.HitCount,
		attributes.hit,
	)
	observer.ObserveInt64(
		instruments.lookupCount,
		stats.NegativeHitCount,
		attributes.negativeHit,
	)
	observer.ObserveInt64(
		instruments.lookupCount,
		stats.MissCount,
		attributes.miss,
	)
	observer.ObserveInt64(
		instruments.loadCount,
		stats.LoadFoundCount,
		attributes.loadFound,
	)
	observer.ObserveInt64(
		instruments.loadCount,
		stats.LoadNotFoundCount,
		attributes.loadNotFound,
	)
	observer.ObserveInt64(
		instruments.loadCount,
		stats.LoadErrorCount,
		attributes.loadError,
	)
	observer.ObserveFloat64(
		instruments.loadTime,
		stats.LoadDuration.Seconds(),
		attributes.base,
	)
	observer.ObserveInt64(
		instruments.sharedCount,
		stats.SharedCount,
		attributes.base,
	)
	observer.ObserveInt64(
		instruments.invalidationCount,
		stats.InvalidatedKeyCount,
		attributes.invalidateKeys,
	)
	observer.ObserveInt64(
		instruments.invalidationCount,
		stats.InvalidatedAllCount,
		attributes.invalidateAll,
	)
	observer.ObserveInt64(
		instruments.evictionCount,
		stats.EvictionCount,
		attributes.base,
	)
	observer.ObserveInt64(
		instruments.expirationCount,
		stats.ExpirationCount,
		attributes.base,
	)
}

func (instruments cacheMetricInstruments) observables() []metric.Observable {
	return []metric.Observable{
		instruments.entryCount,
		instruments.entryMax,
		instruments.segmentCount,
		instruments.lookupCount,
		instruments.loadCount,
		instruments.loadTime,
		instruments.sharedCount,
		instruments.invalidationCount,
		instruments.evictionCount,
		instruments.expirationCount,
	}
}

func newCacheMetricInstruments(meter metric.Meter) (cacheMetricInstruments, error) {
	var instruments cacheMetricInstruments

	var err error

	instruments.entryCount, err = meter.Int64ObservableGauge(
		cacheEntryCountMetricName,
		metric.WithDescription(
			"The number of entries currently resident in cache storage.",
		),
		metric.WithUnit("{entry}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheEntryCountMetricName,
			err,
		)
	}

	instruments.entryMax, err = meter.Int64ObservableGauge(
		cacheEntryMaxMetricName,
		metric.WithDescription(
			"The configured total cache entry budget.",
		),
		metric.WithUnit("{entry}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheEntryMaxMetricName,
			err,
		)
	}

	instruments.segmentCount, err = meter.Int64ObservableGauge(
		cacheSegmentCountMetricName,
		metric.WithDescription(
			"The number of independent cache storage segments.",
		),
		metric.WithUnit("{segment}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheSegmentCountMetricName,
			err,
		)
	}

	instruments.lookupCount, err = meter.Int64ObservableCounter(
		cacheLookupCountMetricName,
		metric.WithDescription(
			"The cumulative number of cache lookups by result.",
		),
		metric.WithUnit("{lookup}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheLookupCountMetricName,
			err,
		)
	}

	instruments.loadCount, err = meter.Int64ObservableCounter(
		cacheLoadCountMetricName,
		metric.WithDescription(
			"The cumulative number of actual cache loader invocations by result.",
		),
		metric.WithUnit("{load}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheLoadCountMetricName,
			err,
		)
	}

	instruments.loadTime, err = meter.Float64ObservableCounter(
		cacheLoadTimeMetricName,
		metric.WithDescription(
			"The cumulative time spent in actual cache loader invocations.",
		),
		metric.WithUnit("s"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheLoadTimeMetricName,
			err,
		)
	}

	instruments.sharedCount, err = meter.Int64ObservableCounter(
		cacheSharedCountMetricName,
		metric.WithDescription(
			"The cumulative number of cache callers that received a shared singleflight result.",
		),
		metric.WithUnit("{request}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheSharedCountMetricName,
			err,
		)
	}

	instruments.invalidationCount, err = meter.Int64ObservableCounter(
		cacheInvalidationMetricName,
		metric.WithDescription(
			"The cumulative number of resident cache entries removed by explicit invalidation, by scope.",
		),
		metric.WithUnit("{entry}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheInvalidationMetricName,
			err,
		)
	}

	instruments.evictionCount, err = meter.Int64ObservableCounter(
		cacheEvictionCountMetricName,
		metric.WithDescription(
			"The cumulative number of cache entries evicted because a storage segment reached capacity.",
		),
		metric.WithUnit("{entry}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheEvictionCountMetricName,
			err,
		)
	}

	instruments.expirationCount, err = meter.Int64ObservableCounter(
		cacheExpirationCountMetricName,
		metric.WithDescription(
			"The cumulative number of expired cache entries removed during lookup.",
		),
		metric.WithUnit("{entry}"),
	)
	if err != nil {
		return cacheMetricInstruments{}, fmt.Errorf(
			"xpg/otel: create %s: %w",
			cacheExpirationCountMetricName,
			err,
		)
	}

	return instruments, nil
}

func newCacheMetricAttributes(name string) cacheMetricAttributes {
	base := []attribute.KeyValue{
		attribute.String(cacheNameAttribute, name),
	}

	option := func(extra ...attribute.KeyValue) metric.ObserveOption {
		return metric.WithAttributeSet(
			attribute.NewSet(
				slices.Concat(base, extra)...,
			),
		)
	}

	return cacheMetricAttributes{
		base: option(),
		hit: option(
			attribute.String(
				cacheLookupResultAttribute,
				cacheLookupResultHit,
			),
		),
		negativeHit: option(
			attribute.String(
				cacheLookupResultAttribute,
				cacheLookupResultNegativeHit,
			),
		),
		miss: option(
			attribute.String(
				cacheLookupResultAttribute,
				cacheLookupResultMiss,
			),
		),
		loadFound: option(
			attribute.String(
				cacheLoadResultAttribute,
				cacheLoadResultFound,
			),
		),
		loadNotFound: option(
			attribute.String(
				cacheLoadResultAttribute,
				cacheLoadResultNotFound,
			),
		),
		loadError: option(
			attribute.String(
				cacheLoadResultAttribute,
				cacheLoadResultError,
			),
		),
		invalidateKeys: option(
			attribute.String(
				cacheInvalidationScopeAttribute,
				cacheInvalidationScopeKeys,
			),
		),
		invalidateAll: option(
			attribute.String(
				cacheInvalidationScopeAttribute,
				cacheInvalidationScopeAll,
			),
		),
	}
}
