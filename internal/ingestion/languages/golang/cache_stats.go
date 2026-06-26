// Package golang — Go capture cache statistics.
// Tracks hit/miss counters for the Go tree-sitter capture pass,
// useful for profiling and debugging scope extraction throughput.
package golang

import "sync/atomic"

// goCaptureCacheStats holds atomic counters for Go capture cache hits/misses.
var goCaptureCacheStats struct {
	hits int64
	miss int64
}

// RecordGoCacheHit increments the Go capture cache hit counter.
func RecordGoCacheHit() {
	atomic.AddInt64(&goCaptureCacheStats.hits, 1)
}

// RecordGoCacheMiss increments the Go capture cache miss counter.
func RecordGoCacheMiss() {
	atomic.AddInt64(&goCaptureCacheStats.miss, 1)
}

// CacheStatsResult holds snapshot of cache hit/miss counters.
type CacheStatsResult struct {
	Hits  int64
	Miss  int64
}

// GetGoCaptureCacheStats returns a snapshot of current cache hit/miss counters.
func GetGoCaptureCacheStats() CacheStatsResult {
	return CacheStatsResult{
		Hits:  atomic.LoadInt64(&goCaptureCacheStats.hits),
		Miss:  atomic.LoadInt64(&goCaptureCacheStats.miss),
	}
}

// ResetGoCaptureCacheStats resets both counters to zero.
func ResetGoCaptureCacheStats() {
	atomic.StoreInt64(&goCaptureCacheStats.hits, 0)
	atomic.StoreInt64(&goCaptureCacheStats.miss, 0)
}