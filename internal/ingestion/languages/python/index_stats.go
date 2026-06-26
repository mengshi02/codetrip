// Package python — Build counter for the per-file-set Python import-resolution index.
// A "build" is a WeakMap cache MISS that materializes a fresh PythonFileIndex (O(files)).
//
// Unlike cache-stats.ts (which gates its counters behind PROF_SCOPE_RESOLUTION
// because they sit on the per-capture hot path), this counter is always live:
// an index build happens at most once per resolution run, so the single increment
// is negligible.
//
// Ported from TS languages/python/index-stats.ts.
package python

import "sync/atomic"

// pythonIndexBuildCounter holds the atomic counter for Python file index builds.
var pythonIndexBuildCounter int64

// RecordPythonFileIndexBuild increments the file index build counter.
func RecordPythonFileIndexBuild() {
	atomic.AddInt64(&pythonIndexBuildCounter, 1)
}

// GetPythonFileIndexBuildCount returns the current file index build count.
func GetPythonFileIndexBuildCount() int {
	return int(atomic.LoadInt64(&pythonIndexBuildCounter))
}

// ResetPythonFileIndexBuildCount resets the file index build counter to zero.
func ResetPythonFileIndexBuildCount() {
	atomic.StoreInt64(&pythonIndexBuildCounter, 0)
}