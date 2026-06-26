package c

import (
	"sort"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// cBindingTier returns the priority tier for a binding origin.
// Lower tier = higher priority.
func cBindingTier(origin shared.BindingOrigin) int {
	switch origin {
	case shared.OriginLocal:
		return 0
	case shared.OriginNamespace:
		return 1
	case shared.OriginImport:
		return 2
	case shared.OriginReexport:
		return 3
	case shared.OriginWildcard:
		return 4
	default:
		return 99
	}
}

// CMergeBindings merges existing and incoming bindings with simple first-wins by tier.
// C has no namespaces or reexports, but the tiers are defined for compatibility.
// Ported from GitNexus c/merge-bindings.ts.
func CMergeBindings(existing, incoming []shared.BindingRef) []shared.BindingRef {
	all := make([]shared.BindingRef, 0, len(existing)+len(incoming))
	all = append(all, existing...)
	all = append(all, incoming...)

	// Sort by tier, then by Def.NodeID for deterministic ordering
	sort.SliceStable(all, func(i, j int) bool {
		ti := cBindingTier(all[i].Origin)
		tj := cBindingTier(all[j].Origin)
		if ti != tj {
			return ti < tj
		}
		return all[i].Def.NodeID < all[j].Def.NodeID
	})

	// Deduplicate by Def.NodeID
	seen := make(map[string]struct{})
	result := make([]shared.BindingRef, 0, len(all))
	for _, b := range all {
		if _, ok := seen[b.Def.NodeID]; ok {
			continue
		}
		seen[b.Def.NodeID] = struct{}{}
		result = append(result, b)
	}
	return result
}