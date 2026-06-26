// Parsing Processor — orchestrates tree-sitter parsing for all scanned files.
//
// Mirrors TS parsing-processor.ts, skeleton for codetrip.
// This is the main entry point for Phase 2 parsing: it takes the scanned
// file list, chunks them, runs tree-sitter queries, and produces SymbolDefinition
// nodes + IMPORT edges for each file.
// Deferred to Phase 6 when language providers are fully implemented.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// ParsingProcessorResult holds the output of parsing all files.
type ParsingProcessorResult struct {
	// All SymbolDefinition nodes produced
	Symbols []shared.SymbolDefinition
	// All ImportEdge relationships found
	Imports []ImportEdge
	// Per-file stats
	FileStats map[string]int // filePath → symbol count
}

// ImportEdge represents an import relationship found during parsing.
type ImportEdge struct {
	SourceFile string     // file containing the import
	ImportPath string     // raw import path text
	ResolvedTo string     // resolved target file (empty if unresolved)
	Kind       ImportKind // direct/package/sideEffect etc.
	Line       int
}

// ParsingProcessorOptions controls parsing behavior.
type ParsingProcessorOptions struct {
	ChunkByteBudget int64
	ReadConcurrency int
	LanguageConfigs []LanguageConfig
}

// RunParsingProcessor parses all scanned files and produces symbol/import data.
//
// Current status: skeleton — full implementation deferred to Phase 6.
func RunParsingProcessor(files []ScannedFile, graph shared.KnowledgeGraph, opts ParsingProcessorOptions) (*ParsingProcessorResult, error) {
	// TODO(Phase 6): for each file, run tree-sitter queries → extract symbols + imports
	return &ParsingProcessorResult{
		Symbols:   []shared.SymbolDefinition{},
		Imports:   []ImportEdge{},
		FileStats: map[string]int{},
	}, nil
}

// AddParsedSymbolsToGraph writes parsed symbols and imports into the KnowledgeGraph.
//
// Current status: skeleton — full implementation deferred to Phase 6.
func AddParsedSymbolsToGraph(graph shared.KnowledgeGraph, result *ParsingProcessorResult) error {
	// TODO(Phase 6): create SymbolDefinition nodes + IMPORTS edges
	return nil
}
