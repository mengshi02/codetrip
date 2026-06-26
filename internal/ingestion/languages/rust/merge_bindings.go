package rust

import (
	"sort"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// RustMergeBindings implements Rust's binding-merge precedence.
// Rust shadowing: local > import/use > wildcard (use::*).
// The scopeID is used for scope-aware merge decisions.
// TODO: full implementation — currently returns concatenated bindings sorted by origin tier.
func RustMergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	combined := make([]shared.BindingRef, 0, len(existing)+len(incoming))
	combined = append(combined, existing...)
	combined = append(combined, incoming...)

	// Sort by origin priority: local > import > namespace > reexport > wildcard
	sort.SliceStable(combined, func(i, j int) bool {
		return originTier(combined[i].Origin) < originTier(combined[j].Origin)
	})

	// Deduplicate by name, keeping the highest-priority binding.
	seen := make(map[string]bool, len(combined))
	result := combined[:0]
	for _, b := range combined {
		name := b.Def.NodeID
		if seen[name] {
			continue
		}
		seen[name] = true
		result = append(result, b)
	}
	return result
}

// originTier returns a numeric priority for binding origin.
// Lower = higher priority.
func originTier(origin shared.BindingOrigin) int {
	switch origin {
	case shared.OriginLocal:
		return 0
	case shared.OriginImport:
		return 1
	case shared.OriginNamespace:
		return 2
	case shared.OriginReexport:
		return 3
	case shared.OriginWildcard:
		return 4
	default:
		return 5
	}
}