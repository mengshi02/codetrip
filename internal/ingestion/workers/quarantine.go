// QuarantineTracker — fault isolation for problematic file paths.
//
// Replaces TS quarantine.ts (Set<string> wrapper). In Go, goroutine
// panics are captured via recover, and the offending file path is
// marked as quarantined — subsequent chunks will skip it.
//
// Key difference from TS: no circuit-breaker or slot-respawn mechanism.
// Go goroutines are cheap (2KB stack), so there's no need to "respawn"
// a worker slot after a panic. The goroutine simply exits, and the
// semaphore allows a new one to start.
package workers

import (
	"fmt"
	"sync"
)

// ── QuarantineTracker ────────────────────────────────────────────────────

// QuarantineTracker tracks file paths that have caused panics or errors
// during parsing, preventing them from being retried in subsequent chunks.
type QuarantineTracker struct {
	mu       sync.RWMutex
	paths    map[string]string // path → reason
	attempts map[string]int    // path → attempt count
	maxRetry int              // maximum retry attempts (default 0 — no retry in Go)
}

// NewQuarantineTracker creates a new QuarantineTracker.
func NewQuarantineTracker() *QuarantineTracker {
	return &QuarantineTracker{
		paths:    make(map[string]string),
		attempts: make(map[string]int),
		maxRetry: 0, // Go goroutines don't need respawn — no retry
	}
}

// Mark records a file path as quarantined with the given reason.
func (q *QuarantineTracker) Mark(path string, reason string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.paths[path] = reason
}

// MarkFromError records a file path as quarantined from an error.
// The reason is derived from the error message.
func (q *QuarantineTracker) MarkFromError(path string, err error) {
	reason := "unknown error"
	if err != nil {
		reason = err.Error()
	}
	q.Mark(path, reason)
}

// IsQuarantined returns true if the path has been quarantined.
func (q *QuarantineTracker) IsQuarantined(path string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	_, ok := q.paths[path]
	return ok
}

// GetReason returns the quarantine reason for a path, or empty string.
func (q *QuarantineTracker) GetReason(path string) string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.paths[path]
}

// Count returns the number of quarantined paths.
func (q *QuarantineTracker) Count() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.paths)
}

// All returns a copy of all quarantined paths and their reasons.
func (q *QuarantineTracker) All() map[string]string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	out := make(map[string]string, len(q.paths))
	for k, v := range q.paths {
		out[k] = v
	}
	return out
}

// RecordAttempt increments the attempt counter for a path.
// Returns true if the path should be retried (attempts < maxRetry).
func (q *QuarantineTracker) RecordAttempt(path string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.attempts[path]++
	return q.attempts[path] <= q.maxRetry
}

// Summary returns a human-readable summary of quarantine state.
func (q *QuarantineTracker) Summary() string {
	q.mu.RLock()
	defer q.mu.RUnlock()
	if len(q.paths) == 0 {
		return "quarantine: no quarantined files"
	}
	return fmt.Sprintf("quarantine: %d file(s) quarantined", len(q.paths))
}