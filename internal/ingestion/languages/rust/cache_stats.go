package rust

import "sync/atomic"

// Atomic counters for Rust capture cache statistics.
var rustCacheHits, rustCacheMisses atomic.Int64

// RecordRustCacheHit increments the cache hit counter.
func RecordRustCacheHit() { rustCacheHits.Add(1) }

// RecordRustCacheMiss increments the cache miss counter.
func RecordRustCacheMiss() { rustCacheMisses.Add(1) }

// GetRustCaptureCacheStats returns (hits, misses) for the Rust capture cache.
func GetRustCaptureCacheStats() (hits int64, misses int64) {
	return rustCacheHits.Load(), rustCacheMisses.Load()
}