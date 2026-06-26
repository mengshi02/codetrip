// Local Symbol Pruner — removes non-exported, non-referenced symbols from the graph.
//
// Mirrors TS local-symbol-pruner.ts, skeleton for codetrip.
// After scope resolution completes, this prunes symbols that:
//   1. Are not exported (isExported=false)
//   2. Have no incoming cross-file references
//   3. Are not part of any heritage/implements chain
// This reduces graph size by ~60-80% on large repos.
// Deferred to Phase 3 when scope resolution populates reference data.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// PruneLocalSymbolsOptions controls pruning behavior.
type PruneLocalSymbolsOptions struct {
	// Keep symbols that are exported even if unreferenced
	KeepExported bool
	// Keep symbols that participate in heritage chains
	KeepHeritage bool
	// Labels that should never be pruned (e.g., File, Folder)
	ProtectedLabels []shared.NodeLabel
}

// PruneLocalSymbols removes non-exported, unreferenced symbols from the graph.
// Returns the count of removed nodes.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func PruneLocalSymbols(graph shared.KnowledgeGraph, opts PruneLocalSymbolsOptions) int {
	// TODO(Phase 3): iterate graph nodes, check export/reference status, remove
	return 0
}