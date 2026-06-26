// Resolve References — resolves cross-file references by matching import paths to symbols.
//
// Mirrors TS resolve-references.ts, skeleton for codetrip.
// After parsing produces local symbols + import edges, this step resolves
// each reference: for every USES edge, it determines which external symbol
// is being referenced and creates a resolved cross-file edge.
// Deferred to Phase 3 when import resolution is implemented.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// UnresolvedReference is a reference that needs cross-file resolution.
type UnresolvedReference struct {
	SourceFile   string // file containing the reference
	SymbolName   string // local name used in the reference
	ImportSource string // the import path from which it might originate
	Line         int
	ScopeID      string // scope of the referencing symbol
}

// ResolvedReference maps a local reference to a target symbol across files.
type ResolvedReference struct {
	SourceScopeID string // the symbol making the reference
	TargetScopeID string // the symbol being referenced
	TargetFile    string // file where the target symbol lives
	RefKind       string // "uses", "calls", "inherits", "implements"
	Evidence      []shared.Evidence
}

// ResolveReferencesResult holds resolved and unresolved references.
type ResolveReferencesResult struct {
	Resolved   []ResolvedReference
	Unresolved []UnresolvedReference
	Stats      map[string]int // "resolved", "unresolved" → counts
}

// ResolveReferencesOptions controls resolution behavior.
type ResolveReferencesOptions struct {
	Adapter   ImportTargetAdapter
	Bindings  map[string]map[string][]shared.BindingRef // scopeID → varName → bindings
	Workspace shared.WorkspaceIndex
}

// ResolveReferences resolves all cross-file references in the graph.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func ResolveReferences(graph shared.KnowledgeGraph, opts ResolveReferencesOptions) (*ResolveReferencesResult, error) {
	// TODO(Phase 3): for each USES/IMPORTS edge, find matching target symbol
	return &ResolveReferencesResult{
		Resolved:   []ResolvedReference{},
		Unresolved: []UnresolvedReference{},
		Stats:      map[string]int{"resolved": 0, "unresolved": 0},
	}, nil
}

// AddResolvedReferencesToGraph writes resolved reference edges into the KnowledgeGraph.
//
// Current status: skeleton — full implementation deferred to Phase 3.
func AddResolvedReferencesToGraph(graph shared.KnowledgeGraph, result *ResolveReferencesResult) error {
	// TODO(Phase 3): create USES/CALLS/INHERITS edges with evidence
	return nil
}
