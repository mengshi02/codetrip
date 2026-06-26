// Package java — Java capture cache statistics.
// Tracks hit/miss counters for the Java tree-sitter capture pass,
// useful for profiling and debugging scope extraction throughput.
package java

import "sync/atomic"

// javaCaptureCacheStats holds atomic counters for Java capture cache hits/misses.
var javaCaptureCacheStats struct {
	hits  int64
	miss  int64
}

// RecordJavaCacheHit increments the Java capture cache hit counter.
func RecordJavaCacheHit() {
	atomic.AddInt64(&javaCaptureCacheStats.hits, 1)
}

// RecordJavaCacheMiss increments the Java capture cache miss counter.
func RecordJavaCacheMiss() {
	atomic.AddInt64(&javaCaptureCacheStats.miss, 1)
}

// JavaCacheStatsResult holds snapshot of cache hit/miss counters.
type JavaCacheStatsResult struct {
	Hits  int64
	Miss  int64
}

// GetJavaCaptureCacheStats returns a snapshot of current cache hit/miss counters.
func GetJavaCaptureCacheStats() JavaCacheStatsResult {
	return JavaCacheStatsResult{
		Hits:  atomic.LoadInt64(&javaCaptureCacheStats.hits),
		Miss:  atomic.LoadInt64(&javaCaptureCacheStats.miss),
	}
}

// ResetJavaCaptureCacheStats resets both counters to zero.
func ResetJavaCaptureCacheStats() {
	atomic.StoreInt64(&javaCaptureCacheStats.hits, 0)
	atomic.StoreInt64(&javaCaptureCacheStats.miss, 0)
}