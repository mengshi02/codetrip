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
func (trip *Trip) GetMetrics() MetricsSnapshot {
	return MetricsSnapshot{
		IndexRepoTotal:   trip.metrics.IndexRepoTotal.Load(),
		IndexRepoSuccess: trip.metrics.IndexRepoSuccess.Load(),
		IndexRepoFail:    trip.metrics.IndexRepoFail.Load(),
		DropIndexTotal:   trip.metrics.DropIndexTotal.Load(),
		ReIndexTotal:     trip.metrics.ReIndexTotal.Load(),
		ReIndexSuccess:   trip.metrics.ReIndexSuccess.Load(),
		ReIndexFail:      trip.metrics.ReIndexFail.Load(),
		ImpactQueries:    trip.metrics.ImpactQueries.Load(),
		SearchQueries:    trip.metrics.SearchQueries.Load(),
		ContextQueries:   trip.metrics.ContextQueries.Load(),
		RenameQueries:    trip.metrics.RenameQueries.Load(),
		NodeReads:        trip.metrics.NodeReads.Load(),
		EdgeReads:        trip.metrics.EdgeReads.Load(),
		NodeWrites:       trip.metrics.NodeWrites.Load(),
		EdgeWrites:       trip.metrics.EdgeWrites.Load(),
		NodeCacheHits:    trip.metrics.NodeCacheHits.Load(),
		NodeCacheMisses:  trip.metrics.NodeCacheMisses.Load(),
		Errors:           trip.metrics.Errors.Load(),
	}
}