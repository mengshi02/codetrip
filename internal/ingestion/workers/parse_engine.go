// Parse engine — concurrent parse orchestration.
//
// This is the core business logic that wires GoroutinePool with
// the actual parsing function. It corresponds to the TS parse-worker.ts
// main loop, but replaces Worker Thread messaging with goroutine dispatch.
//
// Flow:
//  1. ParseFileSet reads files from disk
//  2. GoroutinePool.Dispatch fans out sub-batches to goroutines
//  3. Each goroutine calls parseSingleFile via parseFn
//  4. Results are merged via MergeResults
//  5. Quarantined files are collected into SkippedPaths
//
// Ported from gitnexus workers/parse-worker.ts (103KB → ~200 lines).
package workers

import (
	"fmt"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ── ParseFileSet ─────────────────────────────────────────────────────────

// ParseFileSet runs concurrent parsing on a set of files using a GoroutinePool.
// This is the main entry point called by the pipeline's parse phase.
//
// It replaces the TS pattern of:
//
//	WorkerPool.dispatch(files) → worker.postMessage → worker.onmessage
//
// with:
//
//	GoroutinePool.Dispatch(files, parseFn) → channel → MergeResults
func ParseFileSet(
	pool *GoroutinePool,
	entries []FileEntry,
	repoPath string,
	languages []string, // language filter (empty = all)
	onProgress func(completed int, message string),
) (*ParseFileSetResult, []PoolError) {
	// Filter by language if specified
	filtered := filterByLanguage(entries, languages)
	if len(filtered) == 0 {
		empty := &ParseFileSetResult{
			ExportedTypeMap: make(map[string]map[string]string),
			SkippedPaths:    make(map[string]string),
		}
		return empty, nil
	}

	// Dispatch to goroutine pool
	results, errors := pool.Dispatch(filtered, func(batch []FileEntry) ParseFileSetResult {
		return parseSubBatch(batch, repoPath)
	}, onProgress)

	// Merge all sub-batch results
	if len(results) == 0 {
		empty := &ParseFileSetResult{
			ExportedTypeMap: make(map[string]map[string]string),
			SkippedPaths:    make(map[string]string),
		}
		// Collect quarantined paths
		empty.SkippedPaths = CollectSkippedPaths(empty.SkippedPaths, pool.Quarantine())
		return empty, errors
	}

	merged := MergeResults(results)

	// Collect quarantined paths into skipped
	merged.SkippedPaths = CollectSkippedPaths(merged.SkippedPaths, pool.Quarantine())

	return &merged, errors
}

// ── Sub-batch parse ──────────────────────────────────────────────────────

// parseSubBatch parses a sub-batch of files sequentially within a goroutine.
// This corresponds to TS parse-worker.ts's message handler.
//
// gotreesitter is pure Go — no CGo, no thread-safety concerns.
// Each goroutine can safely create and use its own Parser instance.
func parseSubBatch(files []FileEntry, repoPath string) ParseFileSetResult {
	result := ParseFileSetResult{
		ExportedTypeMap: make(map[string]map[string]string),
		SkippedPaths:    make(map[string]string),
		FileCount:       0,
	}

	for _, f := range files {
		singleResult := parseSingleFile(f, repoPath)
		if singleResult == nil {
			result.SkippedPaths[f.Path] = "parse returned nil"
			continue
		}

		// Accumulate into sub-batch result
		result.ImportEdges = append(result.ImportEdges, singleResult.ImportEdges...)
		result.Definitions = append(result.Definitions, singleResult.Definitions...)
		result.BindingRefs = append(result.BindingRefs, singleResult.BindingRefs...)
		result.ParsedFiles = append(result.ParsedFiles, singleResult.ParsedFiles...)
		result.AllFetchCalls = append(result.AllFetchCalls, singleResult.AllFetchCalls...)
		result.AllExtractedRoutes = append(result.AllExtractedRoutes, singleResult.AllExtractedRoutes...)
		result.AllDecoratorRoutes = append(result.AllDecoratorRoutes, singleResult.AllDecoratorRoutes...)
		result.AllToolDefs = append(result.AllToolDefs, singleResult.AllToolDefs...)
		result.AllORMQueries = append(result.AllORMQueries, singleResult.AllORMQueries...)
		result.AllFetchWrapperDefs = append(result.AllFetchWrapperDefs, singleResult.AllFetchWrapperDefs...)
		MergeExportedTypeMaps(result.ExportedTypeMap, singleResult.ExportedTypeMap)
		result.FileCount++
	}

	return result
}

// ── Single file parse ────────────────────────────────────────────────────

// parseSingleFile parses a single source file.
//
// This is the core per-file parsing pipeline that replaces TS parse-worker.ts's
// per-file processing. It:
//   1. Determines the file's language
//   2. Parses via gotreesitter (pure Go, goroutine-safe)
//   3. Runs language-specific extractors (imports, calls, heritage, routes, tools, ORM)
//   4. Builds ParsedFile artifact
//   5. Accumulates bindings
//
// Current status: reads file content, detects language, produces empty result.
// TODO(Phase 2): Full parsing pipeline with gotreesitter + extractors.
func parseSingleFile(entry FileEntry, repoPath string) *ParseFileSetResult {
	// Determine language
	lang := shared.GetLanguageFromFilename(entry.Path)
	if lang == "" {
		return nil // not a parseable file
	}

	// Read file content if not already loaded
	content := entry.Content
	if content == "" {
		contents := core.ReadFileContents(repoPath, []string{entry.Path})
		var ok bool
		content, ok = contents[entry.Path]
		if !ok {
			return nil // file read failed
		}
	}

	// TODO(Phase 2): Full parsing pipeline
	//   1. Get LanguageProvider for this language
	//   2. Create gotreesitter Parser for this language
	//   3. Parse file → Tree → root Node
	//   4. Run language-specific scope queries → ParsedFile
	//   5. Run extractors:
	//      - Import extraction → ImportEdge[]
	//      - Call/heritage extraction → BindingRef[]
	//      - Route extraction (route_extractors) → ExtractedRoute[]
	//      - Tool definition extraction → ToolDef[]
	//      - ORM query extraction → ORMQuery[]
	//      - Fetch call extraction → FetchCall[]
	//   6. Build ParsedFile artifact with Scopes, Imports, References
	//   7. Accumulate bindings via core.BindingAccumulator

	_ = lang    // suppress unused warning — will be used for provider lookup
	_ = content // suppress unused warning — will be used for parsing

	result := &ParseFileSetResult{
		ExportedTypeMap: make(map[string]map[string]string),
		SkippedPaths:    make(map[string]string),
		FileCount:       1,
	}

	return result
}

// ── Language filter ──────────────────────────────────────────────────────

// filterByLanguage filters file entries by supported languages.
// If languages is empty, all files are included.
func filterByLanguage(entries []FileEntry, languages []string) []FileEntry {
	if len(languages) == 0 {
		return entries
	}

	langSet := make(map[shared.SupportedLanguage]bool, len(languages))
	for _, l := range languages {
		langSet[shared.SupportedLanguage(l)] = true
	}

	filtered := make([]FileEntry, 0, len(entries))
	for _, e := range entries {
		lang := shared.GetLanguageFromFilename(e.Path)
		if lang == "" {
			continue
		}
		if langSet[lang] {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

// ── Progress helper ──────────────────────────────────────────────────────

// FormatProgress creates a progress message for the parse phase.
func FormatProgress(completed int, total int) string {
	if total <= 0 {
		return fmt.Sprintf("parsed %d files", completed)
	}
	pct := completed * 100 / total
	return fmt.Sprintf("parsed %d/%d files (%d%%)", completed, total, pct)
}