// Package typescript — TypeScript capture cache statistics.
// Tracks hit/miss counters for the TypeScript tree-sitter capture pass,
// useful for profiling and debugging scope extraction throughput.
// Mirrors the Go provider's cache_stats.go pattern.
//
// Ported from TS languages/typescript/cache-stats.ts.
package typescript

import "sync/atomic"

// tsCaptureCacheStats holds atomic counters for TypeScript capture cache hits/misses.
var tsCaptureCacheStats struct {
	hits int64
	miss int64
}

// RecordTsCacheHit increments the TypeScript capture cache hit counter.
func RecordTsCacheHit() {
	atomic.AddInt64(&tsCaptureCacheStats.hits, 1)
}

// RecordTsCacheMiss increments the TypeScript capture cache miss counter.
func RecordTsCacheMiss() {
	atomic.AddInt64(&tsCaptureCacheStats.miss, 1)
}

// TsCacheStatsResult holds snapshot of cache hit/miss counters.
type TsCacheStatsResult struct {
	Hits  int64
	Miss  int64
}

// GetTsCaptureCacheStats returns a snapshot of current cache hit/miss counters.
func GetTsCaptureCacheStats() TsCacheStatsResult {
	return TsCacheStatsResult{
		Hits:  atomic.LoadInt64(&tsCaptureCacheStats.hits),
		Miss:  atomic.LoadInt64(&tsCaptureCacheStats.miss),
	}
}

// ResetTsCaptureCacheStats resets both counters to zero.
func ResetTsCaptureCacheStats() {
	atomic.StoreInt64(&tsCaptureCacheStats.hits, 0)
	atomic.StoreInt64(&tsCaptureCacheStats.miss, 0)
}