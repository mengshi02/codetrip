// Emit References — drains ReferenceIndex into the KnowledgeGraph as edges.
//
// Mirrors TS emit-references.ts, skeleton for codetrip.
// The full implementation will iterate ReferenceIndex entries and emit
// CALLS, INHERITS, METHOD_OVERRIDES, etc. edges with evidence traces.
// Deferred to Phase 3 (scope resolution engine completes the ReferenceIndex).

package core

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// EmitReferencesOptions controls how references are emitted into the graph.
type EmitReferencesOptions struct {
	// Whether to attach evidence traces to emitted edges
	IncludeEvidence bool
	// Minimum confidence threshold for emitting an edge
	MinConfidence float64
}

// EmitReferences drains the ReferenceIndex into the KnowledgeGraph.
// For each reference, it creates an appropriate edge (CALLS, INHERITS, etc.)
// with confidence scores and optional evidence traces.
//
// Current status: skeleton — full implementation deferred to Phase 3
// when the scope resolution engine produces a populated ReferenceIndex.
func EmitReferences(graph shared.KnowledgeGraph, refIndex *shared.ReferenceIndex, opts EmitReferencesOptions) int {
	// TODO(Phase 3): iterate refIndex entries and emit edges
	return 0
}