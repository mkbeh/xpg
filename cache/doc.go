// Package cache provides bounded in-process read-through caching.
//
// Cache entries use absolute TTL expiration, optional TTL jitter, LRU
// eviction, negative caching, duplicate load suppression, and explicit
// invalidation.
//
// Cache statistics are collected locally and exposed through Cache.Stats.
// Optional metrics integrations register during New and observe those snapshots
// without adding telemetry calls to the cache request path.
//
// The cache is local to one application process. It does not provide
// distributed cache coherence between application instances.
package cache
