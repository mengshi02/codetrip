// Result merge — aggregate ParseFileSetResult from multiple goroutines.
//
// Replaces TS result-merge.ts. In TS, results from Worker threads
// arrive via postMessage and must be serialized/deserialized across
// the V8 structured clone boundary. In Go, goroutines share memory
// directly — results are sent through typed channels and merged
// without serialization overhead.
//
// Thread safety: MergeResults is called from the goroutine that
// collects results from the channel — single-writer, no lock needed.
package workers

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ── Merge functions ──────────────────────────────────────────────────────

// MergeResults aggregates multiple ParseFileSetResult into one.
// Corresponds to TS result-merge.ts mergeWorkerResults.
//
// The merge is purely additive — all slices/maps are concatenated/merged.
// Order is preserved within each result, but across results the order
// follows the order they arrive from the channel.
func MergeResults(results []ParseFileSetResult) ParseFileSetResult {
	var merged ParseFileSetResult

	// Pre-allocate for the common case
	totalFiles := 0
	for _, r := range results {
		totalFiles += r.FileCount
	}

	merged.ImportEdges = make([]shared.ImportEdge, 0)
	merged.Definitions = make([]shared.SymbolDefinition, 0)
	merged.BindingRefs = make([]shared.BindingRef, 0)
	merged.AllFetchCalls = make([]interface{}, 0)
	merged.AllExtractedRoutes = make([]interface{}, 0)
	merged.AllDecoratorRoutes = make([]interface{}, 0)
	merged.AllToolDefs = make([]interface{}, 0)
	merged.AllORMQueries = make([]interface{}, 0)
	merged.AllFetchWrapperDefs = make([]interface{}, 0)
	merged.ParsedFiles = make([]shared.ParsedFile, 0)
	merged.ExportedTypeMap = make(map[string]map[string]string)
	merged.SkippedPaths = make(map[string]string)
	merged.FileCount = 0

	for _, r := range results {
		// Slices — append
		merged.ImportEdges = append(merged.ImportEdges, r.ImportEdges...)
		merged.Definitions = append(merged.Definitions, r.Definitions...)
		merged.BindingRefs = append(merged.BindingRefs, r.BindingRefs...)
		merged.AllFetchCalls = append(merged.AllFetchCalls, r.AllFetchCalls...)
		merged.AllExtractedRoutes = append(merged.AllExtractedRoutes, r.AllExtractedRoutes...)
		merged.AllDecoratorRoutes = append(merged.AllDecoratorRoutes, r.AllDecoratorRoutes...)
		merged.AllToolDefs = append(merged.AllToolDefs, r.AllToolDefs...)
		merged.AllORMQueries = append(merged.AllORMQueries, r.AllORMQueries...)
		merged.AllFetchWrapperDefs = append(merged.AllFetchWrapperDefs, r.AllFetchWrapperDefs...)
		merged.ParsedFiles = append(merged.ParsedFiles, r.ParsedFiles...)

		// Map — merge per-file entries
		MergeExportedTypeMaps(merged.ExportedTypeMap, r.ExportedTypeMap)

		// Skipped paths — merge (no overwrite — first reason wins)
		for path, reason := range r.SkippedPaths {
			if _, exists := merged.SkippedPaths[path]; !exists {
				merged.SkippedPaths[path] = reason
			}
		}

		merged.FileCount += r.FileCount
	}

	return merged
}

// MergeExportedTypeMaps merges src into dst.
// For each file in src, if the file key doesn't exist in dst, the
// entire inner map is copied. If it does exist, individual symbol
// entries are added (existing entries are not overwritten).
func MergeExportedTypeMaps(
	dst map[string]map[string]string,
	src map[string]map[string]string,
) {
	for filePath, srcSymbols := range src {
		dstSymbols, exists := dst[filePath]
		if !exists {
			// Copy the entire inner map
			dst[filePath] = make(map[string]string, len(srcSymbols))
			for k, v := range srcSymbols {
				dst[filePath][k] = v
			}
			continue
		}
		// Merge individual entries — existing entries win
		for k, v := range srcSymbols {
			if _, exists := dstSymbols[k]; !exists {
				dstSymbols[k] = v
			}
		}
	}
}

// CollectSkippedPaths merges quarantine entries into the SkippedPaths map.
// This is called after all goroutines complete to include quarantined
// files in the final result.
func CollectSkippedPaths(
	skipped map[string]string,
	quarantine *QuarantineTracker,
) map[string]string {
	if quarantine == nil {
		return skipped
	}
	if skipped == nil {
		skipped = make(map[string]string)
	}
	for path, reason := range quarantine.All() {
		if _, exists := skipped[path]; !exists {
			skipped[path] = reason
		}
	}
	return skipped
}