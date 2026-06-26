// MRO (Method Resolution Order) Processor — computes linearizations for type hierarchies.
//
// Mirrors TS mro-processor.ts, skeleton for codetrip.
// After cross-file resolution establishes IMPLEMENTS/EXTENDS edges,
// MRO computes C3-linearization order for each type so method resolution
// can be deterministic. Deferred to Phase 5 when type hierarchies are complete.

package core

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// MROEntry represents one step in a type's method resolution order.
type MROEntry struct {
	ScopeID string
	Depth   int // distance from the root type (0 = root)
}

// MROProcessorResult holds computed MRO for all types with heritage chains.
type MROProcessorResult struct {
	// Map of type ScopeID → ordered list of ancestors
	Linearizations map[string][]MROEntry
	// Types that had cyclic heritage (broken by removing duplicates)
	CyclicTypes []string
}

// ComputeMRO computes C3-linearization for all types in the graph.
//
// Current status: skeleton — full implementation deferred to Phase 5.
func ComputeMRO(graph shared.KnowledgeGraph) (*MROProcessorResult, error) {
	// TODO(Phase 5): C3-linearization algorithm
	//   1. Find all types with IMPLEMENTS/EXTENDS edges
	//   2. Build ancestor lists
	//   3. Merge using C3 algorithm
	//   4. Detect and break cycles
	return &MROProcessorResult{
		Linearizations: map[string][]MROEntry{},
		CyclicTypes:    []string{},
	}, nil
}

// AddMROToGraph writes MRO metadata into graph node properties.
//
// Current status: skeleton — full implementation deferred to Phase 5.
func AddMROToGraph(graph shared.KnowledgeGraph, result *MROProcessorResult) error {
	// TODO(Phase 5): set mro property on each type node
	return nil
}

// C3Merge performs the C3-linearization merge of multiple parent MROs.
// This is the core algorithm: merge lists by always taking the head of
// the first list that is not in the tail of any other list.
//
// Current status: skeleton — full implementation deferred to Phase 5.
func C3Merge(parentLists [][]MROEntry) []MROEntry {
	// TODO(Phase 5): implement C3 merge algorithm
	return []MROEntry{}
}