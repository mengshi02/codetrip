// Scope Extractor Bridge — bridges language-specific scope extraction to the core pipeline.
//
// Mirrors TS scope-extractor-bridge.ts, skeleton for codetrip.
// This module sits between the language provider (which knows how to
// extract scopes from a specific language's source) and the core pipeline
// (which processes scopes uniformly). It dispatches to the correct
// LanguageProvider based on file language.
// Deferred to Phase 6 when language providers are implemented.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// ScopeExtractorBridgeResult holds extracted scopes for all files.
type ScopeExtractorBridgeResult struct {
	Scopes  map[string][]ScopeInfo // filePath → scopes
	Stats   map[string]int         // language → scope count
	Errors  map[string]string      // filePath → error message (if any)
}

// ExtractScopesForFile dispatches scope extraction to the correct language provider.
//
// Current status: skeleton — full implementation deferred to Phase 6.
func ExtractScopesForFile(filePath string, source []byte, provider LanguageProvider) ([]ScopeInfo, error) {
	// TODO(Phase 6): call provider.ExtractScopes
	return []ScopeInfo{}, nil
}

// RunScopeExtractorBridge extracts scopes for all files using their providers.
//
// Current status: skeleton — full implementation deferred to Phase 6.
func RunScopeExtractorBridge(files map[string][]byte, registry *LanguageProviderRegistry) (*ScopeExtractorBridgeResult, error) {
	// TODO(Phase 6): for each file, lookup provider, call ExtractScopes
	return &ScopeExtractorBridgeResult{
		Scopes: map[string][]ScopeInfo{},
		Stats:  map[string]int{},
		Errors: map[string]string{},
	}, nil
}

// AddScopesToGraph writes scope nodes and parent-child edges into KnowledgeGraph.
//
// Current status: skeleton — full implementation deferred to Phase 6.
func AddScopesToGraph(graph shared.KnowledgeGraph, result *ScopeExtractorBridgeResult) error {
	// TODO(Phase 6): create scope nodes + CONTAINS edges
	return nil
}