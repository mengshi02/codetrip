package codetrip

import "sync/atomic"

// Metrics holds lightweight operation counters for observability.
// All fields use atomic operations for lock-free updates with minimal performance impact.
type Metrics struct {
	// Index operations
	IndexRepoTotal   atomic.Int64 // total IndexRepo calls
	IndexRepoSuccess atomic.Int64 // successful IndexRepo calls
	IndexRepoFail    atomic.Int64 // failed IndexRepo calls
	DropIndexTotal   atomic.Int64 // total DropIndex calls
	ReIndexTotal     atomic.Int64 // total ReIndex calls
	ReIndexSuccess   atomic.Int64 // successful ReIndex calls
	ReIndexFail      atomic.Int64 // failed ReIndex calls

	// Query operations
	ImpactQueries    atomic.Int64 // impact analysis queries
	SearchQueries    atomic.Int64 // search queries
	ContextQueries   atomic.Int64 // context queries
	RenameQueries    atomic.Int64 // rename queries

	// Graph operations
	NodeReads  atomic.Int64 // total node read operations
	EdgeReads  atomic.Int64 // total edge read operations
	NodeWrites atomic.Int64 // total node write operations
	EdgeWrites atomic.Int64 // total edge write operations

	// Cache operations
	NodeCacheHits   atomic.Int64 // node cache hit count
	NodeCacheMisses atomic.Int64 // node cache miss count

	// Error tracking
	Errors atomic.Int64 // total error count
}

// MetricsSnapshot represents a point-in-time snapshot of metrics
type MetricsSnapshot struct {
	IndexRepoTotal   int64
	IndexRepoSuccess int64
	IndexRepoFail    int64
	DropIndexTotal   int64
	ReIndexTotal     int64
	ReIndexSuccess   int64
	ReIndexFail      int64
	ImpactQueries    int64
	SearchQueries    int64
	ContextQueries   int64
	RenameQueries    int64
	NodeReads        int64
	EdgeReads        int64
	NodeWrites       int64
	EdgeWrites       int64
	NodeCacheHits    int64
	NodeCacheMisses  int64
	Errors           int64
}

// GetMetrics returns a snapshot of current metrics
func (e *Engine) GetMetrics() MetricsSnapshot {
	return MetricsSnapshot{
		IndexRepoTotal:   e.metrics.IndexRepoTotal.Load(),
		IndexRepoSuccess: e.metrics.IndexRepoSuccess.Load(),
		IndexRepoFail:    e.metrics.IndexRepoFail.Load(),
		DropIndexTotal:   e.metrics.DropIndexTotal.Load(),
		ReIndexTotal:     e.metrics.ReIndexTotal.Load(),
		ReIndexSuccess:   e.metrics.ReIndexSuccess.Load(),
		ReIndexFail:      e.metrics.ReIndexFail.Load(),
		ImpactQueries:    e.metrics.ImpactQueries.Load(),
		SearchQueries:    e.metrics.SearchQueries.Load(),
		ContextQueries:   e.metrics.ContextQueries.Load(),
		RenameQueries:    e.metrics.RenameQueries.Load(),
		NodeReads:        e.metrics.NodeReads.Load(),
		EdgeReads:        e.metrics.EdgeReads.Load(),
		NodeWrites:       e.metrics.NodeWrites.Load(),
		EdgeWrites:       e.metrics.EdgeWrites.Load(),
		NodeCacheHits:    e.metrics.NodeCacheHits.Load(),
		NodeCacheMisses:  e.metrics.NodeCacheMisses.Load(),
		Errors:           e.metrics.Errors.Load(),
	}
}