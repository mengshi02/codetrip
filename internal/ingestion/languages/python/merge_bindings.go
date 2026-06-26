// Package python — Python LEGB binding merge precedence.
// Implements per-scope binding merge for Python using LEGB rule:
//   - Tier 0: local — x = …, def x, class x in this scope
//   - Tier 1: import / namespace / reexport — from m import x, import m
//   - Tier 2: wildcard — from m import *
//
// Within a surviving tier, de-dup by DefId (last-write-wins).
// Ported from TS languages/python/merge-bindings.ts.
package python

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

const (
	tierLocal     = 0
	tierImport    = 1 // import / namespace / reexport all at same tier
	tierWildcard  = 2
	tierUnknown   = 3
)

// tierOf maps BindingOrigin to its LEGB merge precedence tier.
// Lower tier = higher precedence (local wins over import).
func tierOf(origin shared.BindingOrigin) int {
	switch origin {
	case shared.OriginLocal:
		return tierLocal
	case shared.OriginImport, shared.OriginNamespace, shared.OriginReexport:
		return tierImport
	case shared.OriginWildcard:
		return tierWildcard
	default:
		return tierUnknown
	}
}

// PythonMergeBindings merges binding sets using Python LEGB precedence.
// It keeps only bindings at the best (lowest) tier and de-duplicates by DefId.
//
// Mirrors TS pythonMergeBindings(bindings).
func PythonMergeBindings(bindings []shared.BindingRef) []shared.BindingRef {
	if len(bindings) == 0 {
		return bindings
	}

	// Find the best (lowest) tier across all bindings.
	bestTier := tierUnknown
	for _, b := range bindings {
		t := tierOf(b.Origin)
		if t < bestTier {
			bestTier = t
		}
	}

	// Keep only bindings at the best tier.
	var survivors []shared.BindingRef
	for _, b := range bindings {
		if tierOf(b.Origin) == bestTier {
			survivors = append(survivors, b)
		}
	}

	// De-duplicate by DefId (last-write-wins for Python semantics).
	seen := make(map[string]shared.BindingRef)
	for _, b := range survivors {
		seen[b.Def.NodeID] = b
	}

	result := make([]shared.BindingRef, 0, len(seen))
	for _, b := range seen {
		result = append(result, b)
	}
	return result
}