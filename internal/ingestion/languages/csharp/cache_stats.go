// Package csharp — C# capture cache statistics.
// Tracks hit/miss counters for the C# tree-sitter capture pass,
// useful for profiling and debugging scope extraction throughput.
package csharp

import "sync/atomic"

// csharpCaptureCacheStats holds atomic counters for C# capture cache hits/misses.
var csharpCaptureCacheStats struct {
	hits int64
	miss int64
}

// RecordCsharpCacheHit increments the C# capture cache hit counter.
func RecordCsharpCacheHit() {
	atomic.AddInt64(&csharpCaptureCacheStats.hits, 1)
}

// RecordCsharpCacheMiss increments the C# capture cache miss counter.
func RecordCsharpCacheMiss() {
	atomic.AddInt64(&csharpCaptureCacheStats.miss, 1)
}

// CsharpCacheStatsResult holds snapshot of cache hit/miss counters.
type CsharpCacheStatsResult struct {
	Hits  int64
	Miss  int64
}

// GetCsharpCaptureCacheStats returns a snapshot of current cache hit/miss counters.
func GetCsharpCaptureCacheStats() CsharpCacheStatsResult {
	return CsharpCacheStatsResult{
		Hits:  atomic.LoadInt64(&csharpCaptureCacheStats.hits),
		Miss:  atomic.LoadInt64(&csharpCaptureCacheStats.miss),
	}
}

// ResetCsharpCaptureCacheStats resets both counters to zero.
func ResetCsharpCaptureCacheStats() {
	atomic.StoreInt64(&csharpCaptureCacheStats.hits, 0)
	atomic.StoreInt64(&csharpCaptureCacheStats.miss, 0)
}