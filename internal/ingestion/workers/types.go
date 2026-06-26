// Workers — Go-native concurrent parse engine.
//
// This package provides the goroutine-based concurrent parsing infrastructure
// that replaces the TS Worker Threads model. Core advantages:
//
//   - goroutine: lightweight (2KB stack), no slot/respawn mechanism needed
//   - channel: native communication, no postMessage/transferList
//   - recover: handles panics, no quarantine/circuit-breaker
//   - shared memory: no V8 structured clone boundary
//
// Ported from gitnexus workers/ (6 files → 5 files, 3 TS mechanisms dropped).
package workers

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ── Result types ─────────────────────────────────────────────────────────

// ParseFileSetResult is the result of parsing a single sub-batch of files.
// Corresponds to TS ParseWorkerResult, but drops Worker-Threads-specific fields
// (workerId, transferList, serialized buffers).
type ParseFileSetResult struct {
	// ── Import edges ──
	ImportEdges []shared.ImportEdge

	// ── Symbol definitions ──
	Definitions []shared.SymbolDefinition

	// ── Binding refs (calls, heritage, etc.) ──
	BindingRefs []shared.BindingRef

	// ── Extracted artifacts (typed later — currently interface{}) ──
	AllFetchCalls       []interface{} // ExtractedFetchCall
	AllExtractedRoutes  []interface{} // ExtractedRoute
	AllDecoratorRoutes  []interface{} // ExtractedDecoratorRoute
	AllToolDefs         []interface{} // ExtractedToolDef
	AllORMQueries       []interface{} // ExtractedORMQuery
	AllFetchWrapperDefs []interface{} // FetchWrapperDef

	// ── Parsed files ──
	ParsedFiles []shared.ParsedFile

	// ── Exported type map (filePath → symbolName → qualifiedName) ──
	ExportedTypeMap map[string]map[string]string

	// ── Metadata ──
	SkippedPaths map[string]string // path → reason
	FileCount    int
	ChunkIndex   int
	SubBatchIdx  int
}

// FileScopeBinding holds file-scoped binding entries.
// Corresponds to TS parse-worker.ts fileScopeBindings.
type FileScopeBinding struct {
	FilePath string
	Bindings []core.BindingEntry
}

// ── Error/stats types ───────────────────────────────────────────────────

// PoolError represents an error from a single goroutine in the pool.
// Corresponds to TS WorkerError, but uses Go recover instead of
// Node.js error events.
type PoolError struct {
	FilePath  string // file being processed when the error occurred
	Err       error  // underlying error
	Recovered bool   // true if captured via recover (goroutine panic)
}

// PoolStats holds runtime statistics for the goroutine pool.
// Corresponds to TS WorkerPoolStats.
type PoolStats struct {
	MaxWorkers    int // configured concurrency limit
	ActiveWorkers int // currently running goroutines
	Quarantined   int // files skipped due to prior errors
	Errors        int // total errors encountered
	Completed     int // files successfully parsed
}

// ── Configuration ────────────────────────────────────────────────────────

const (
	// DefaultSubBatchBytes is the default byte budget per sub-batch.
	// 256KB balances overhead vs. parallelism — smaller sub-batches
	// increase scheduling overhead, larger ones reduce parallelism.
	DefaultSubBatchBytes = 256 * 1024

	// DefaultConcurrency is the default number of concurrent goroutines.
	// Set to runtime.NumCPU() at init time (see goroutine_pool.go).
	DefaultConcurrency = 0 // 0 means "use NumCPU"
)