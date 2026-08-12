// Package cache provides bounded in-process read-through caching.
//
// Cache entries use absolute TTL expiration, optional TTL jitter, LRU
// eviction, negative caching, duplicate load suppression, and explicit
// invalidation.
//
// The cache is local to one application process. It does not provide
// distributed cache coherence between application instances.
package cache
