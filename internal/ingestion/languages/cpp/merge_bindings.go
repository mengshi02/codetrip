package cpp

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// bindingTier returns the priority tier for a binding origin.
// C++ tier precedence: local(0) > namespace(1) > import(2) > reexport(3) > wildcard(4).
func bindingTier(origin shared.BindingOrigin) int {
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

// CppMergeBindings merges existing and incoming bindings with first-wins by tier.
// C++ tier precedence: local(0) > namespace(1) > import(2) > reexport(3) > wildcard(4).
// Deduplicates by Def.NodeID.
// Ported from GitNexus cpp/merge-bindings.ts.
func CppMergeBindings(existing, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	all := make([]shared.BindingRef, 0, len(existing)+len(incoming))
	all = append(all, existing...)
	all = append(all, incoming...)

	// Sort by tier, then by Def.NodeID for deterministic ordering
	for i := 1; i < len(all); i++ {
		for j := i; j > 0; j-- {
			if bindingTier(all[j].Origin) < bindingTier(all[j-1].Origin) ||
				(bindingTier(all[j].Origin) == bindingTier(all[j-1].Origin) &&
					all[j].Def.NodeID < all[j-1].Def.NodeID) {
				all[j], all[j-1] = all[j-1], all[j]
			}
		}
	}

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