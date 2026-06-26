// Parse implementation — chunked parse + resolve loop.
//
// This is the core parsing engine of the ingestion pipeline. It reads
// source files in byte-budget chunks, dispatches each chunk to a
// GoroutinePool for concurrent parsing via gotreesitter (pure Go,
// goroutine-safe), and merges results across chunks.
//
// The TS version uses Node.js Worker Threads; codetrip uses Go's native
// goroutine+channel model via the workers.GoroutinePool:
//   - goroutine: lightweight (2KB stack), no slot/respawn needed
//   - channel: native communication, no postMessage/transferList
//   - recover: handles panics, no quarantine/circuit-breaker
//   - shared memory: no V8 structured clone boundary
//
// Dependencies (Phase 2 modules to be implemented):
//   - binding-accumulator → BindingAccumulator
//   - parsing-processor → processParsing, mergeChunkResults
//   - call-processor → processCalls, buildExportedTypeMapFromGraph
//   - import-processor → processImports, buildImportResolutionContext
//   - heritage-processor → processHeritage, buildHeritageMap
//   - filesystem-walker → readFileContents
//   - framework-detection → detectFrameworks
//   - language-config → LanguageConfig
//   - finalize-orchestrator → runFinalize
//
// Current status: skeleton with GoroutinePool integration.
// TODO markers indicate where Phase 2 module integration is needed.

package pipeline

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/mengshi02/codetrip/internal/ingestion/workers"
)

// ─── Constants ────────────────────────────────────────────────

const (
	// DefaultChunkByteBudget is the default byte budget per parse chunk.
	// 2MB gives a useful cache-invalidation floor (~1/N chunks on a
	// multi-MB repo) while keeping overhead low on cold runs.
	DefaultChunkByteBudget = 2 * 1024 * 1024

	// ReadConcurrency controls how many file reads are batched together.
	ReadConcurrency = 32
)

// ─── Types ──────────────────────────────────────────────────

// FileEntry represents a file with its path and content loaded.
// Kept here for backward compatibility; workers.FileEntry is the canonical type.
type FileEntry = workers.FileEntry

// ChunkResult holds the results from parsing a single chunk.
// Will be populated as Phase 2 modules are integrated.
type ChunkResult struct {
	// Import edges extracted from this chunk
	ImportEdges []shared.ImportEdge
	// Symbol definitions extracted from this chunk
	Definitions []shared.SymbolDefinition
	// Binding refs (calls, heritage, etc.) extracted from this chunk
	BindingRefs []shared.BindingRef
	// Parsed files produced by this chunk
	ParsedFiles []shared.ParsedFile
	// Whether this chunk was parsed from fresh content (not cached)
	WasFresh bool
}

// ParseImplResult holds the aggregate results of the full parse + resolve loop.
type ParseImplResult struct {
	// Exported type map: filePath → (symbolName → qualifiedName)
	ExportedTypeMap map[string]map[string]string
	// All fetch calls extracted across all chunks
	AllFetchCalls []interface{} // TODO: typed as ExtractedFetchCall
	// All fetch wrapper definitions across all chunks
	AllFetchWrapperDefs []interface{} // TODO: typed as FetchWrapperDef
	// All extracted routes across all chunks
	AllExtractedRoutes []interface{} // TODO: typed as ExtractedRoute
	// All decorator routes across all chunks
	AllDecoratorRoutes []interface{} // TODO: typed as ExtractedDecoratorRoute
	// All tool definitions across all chunks
	AllToolDefs []interface{} // TODO: typed as ExtractedToolDef
	// All ORM queries across all chunks
	AllORMQueries []interface{} // TODO: typed as ExtractedORMQuery
	// Binding accumulator (aggregate across all chunks)
	BindingAccumulator interface{} // TODO: typed as *BindingAccumulator
	// Resolution context for cross-file propagation
	ResolutionContext interface{} // TODO: typed as ResolutionContext
	// Whether a worker pool was used
	UsedWorkerPool bool
	// ParsedFile artifacts for scope-resolution re-extract cache
	ParsedFiles []shared.ParsedFile
	// All paths that were actually parseable
	AllPaths []string
	// Total files scanned (including non-parseable)
	TotalFiles int
}

// ─── Chunk builder ──────────────────────────────────────────

// BuildChunks splits scanned files into byte-budget chunks for parsing.
// Files are sorted alphabetically for stable chunk membership across runs
// (macOS APFS directory order can change after modifications — different
// chunks → different cache hashes → false invalidation).
func BuildChunks(scanned []ScannedFile, budget int64) [][]string {
	// Sort by path for stable membership
	sorted := make([]ScannedFile, len(scanned))
	copy(sorted, scanned)
	for i := 0; i < len(sorted)-1; i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Path > sorted[j].Path {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	var chunks [][]string
	var current []string
	var currentBytes int64

	for _, f := range sorted {
		if len(current) > 0 && currentBytes+f.Size > budget {
			chunks = append(chunks, current)
			current = nil
			currentBytes = 0
		}
		current = append(current, f.Path)
		currentBytes += f.Size
	}
	if len(current) > 0 {
		chunks = append(chunks, current)
	}
	return chunks
}

// ─── Main parse + resolve function ──────────────────────────

// RunChunkedParseAndResolve is the main entry point for the parse phase.
// It reads source files in byte-budget chunks, dispatches each chunk to
// a GoroutinePool for concurrent parsing, and merges results across chunks.
//
// Current implementation: skeleton — builds chunks, creates pool, returns
// empty results.
// TODO: integrate Phase 2 modules (parsing-processor, call-processor,
// import-processor, heritage-processor, binding-accumulator, etc.)
func RunChunkedParseAndResolve(
	graph interface{}, // TODO: typed as shared.KnowledgeGraph
	scannedFiles []ScannedFile,
	allPaths []string,
	totalFiles int,
	repoPath string,
	budget int64,
) (*ParseImplResult, error) {
	// Filter to parseable files (known language + available parser)
	parseable := make([]ScannedFile, 0, len(scannedFiles))
	for _, f := range scannedFiles {
		lang := shared.GetLanguageFromFilename(f.Path)
		if lang != "" {
			parseable = append(parseable, f)
		}
	}

	if len(parseable) == 0 {
		return &ParseImplResult{
			ExportedTypeMap: make(map[string]map[string]string),
			UsedWorkerPool:  false,
			AllPaths:        allPaths,
			TotalFiles:      totalFiles,
		}, nil
	}

	// Build byte-budget chunks
	chunks := BuildChunks(parseable, budget)
	if len(chunks) == 0 {
		return &ParseImplResult{
			ExportedTypeMap: make(map[string]map[string]string),
			UsedWorkerPool:  false,
			AllPaths:        allPaths,
			TotalFiles:      totalFiles,
		}, nil
	}

	// Create goroutine pool for concurrent parsing
	pool := workers.NewGoroutinePool(0, 0) // defaults: NumCPU, 256KB sub-batch
	_ = pool                                // used below in Phase 2

	// TODO(Phase 2): For each chunk:
	//   1. Read file contents (filesystem-walker.readFileContents)
	//   2. Convert ScannedFile → workers.FileEntry
	//   3. Call workers.ParseFileSet(pool, entries, repoPath, languages, onProgress)
	//   4. Resolve imports (import-processor.processImports)
	//   5. Resolve calls (call-processor.processCalls)
	//   6. Resolve heritage (heritage-processor.processHeritage)
	//   7. Synthesize wildcard bindings
	//   8. Merge chunk results into aggregate

	// Placeholder: return empty aggregate
	result := &ParseImplResult{
		ExportedTypeMap: make(map[string]map[string]string),
		UsedWorkerPool:  true, // GoroutinePool is always used
		AllPaths:        allPaths,
		TotalFiles:      totalFiles,
	}

	fmt.Printf("parse: %d parseable files in %d chunks (%dMB budget), pool concurrency=%d\n",
		len(parseable), len(chunks), budget/(1024*1024), pool.Stats().MaxWorkers)

	return result, nil
}

// ResolveChunkByteBudget determines the byte budget per chunk.
// Priority: explicit option > environment variable > default.
func ResolveChunkByteBudget(optBudget int64) int64 {
	if optBudget > 0 {
		return optBudget
	}
	return DefaultChunkByteBudget
}