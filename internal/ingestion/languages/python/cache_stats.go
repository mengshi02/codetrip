// Package python — Python capture cache statistics.
// Tracks hit/miss counters for the Python tree-sitter capture pass,
// useful for profiling and debugging scope extraction throughput.
//
// Ported from TS languages/python/cache-stats.ts.
package python

import "sync/atomic"

// pythonCaptureCacheStats holds atomic counters for Python capture cache hits/misses.
var pythonCaptureCacheStats struct {
	hits  int64
	miss  int64
}

// RecordCacheHit increments the Python capture cache hit counter.
func RecordCacheHit() {
	atomic.AddInt64(&pythonCaptureCacheStats.hits, 1)
}

// RecordCacheMiss increments the Python capture cache miss counter.
func RecordCacheMiss() {
	atomic.AddInt64(&pythonCaptureCacheStats.miss, 1)
}

// CacheStatsResult holds a snapshot of cache hit/miss counters.
type CacheStatsResult struct {
	Hits  int64
	Miss  int64
}

// GetPythonCaptureCacheStats returns a snapshot of current cache hit/miss counters.
func GetPythonCaptureCacheStats() (hits int, misses int) {
	return int(atomic.LoadInt64(&pythonCaptureCacheStats.hits)),
		int(atomic.LoadInt64(&pythonCaptureCacheStats.miss))
}

// ResetPythonCaptureCacheStats resets both counters to zero.
func ResetPythonCaptureCacheStats() {
	atomic.StoreInt64(&pythonCaptureCacheStats.hits, 0)
	atomic.StoreInt64(&pythonCaptureCacheStats.miss, 0)
}