// Package csharp — C# binding merge precedence.
// Implements per-scope binding merge for C#: local bindings take precedence
// over using namespace bindings, which take precedence over using static
// bindings. This mirrors C#'s actual shadowing rules:
//
//	local > using (namespace) > using static
//
// When two bindings have the same name, the lower-tier (higher-number) origin
// is dropped.
// Ported from TS languages/csharp/merge-bindings.ts.
package csharp

import (
	"sort"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// csharpOriginTier maps BindingOrigin to its merge precedence tier.
// C# shadowing order: local > using (namespace) > using static.
// Lower tier = higher precedence (local wins over using).
func csharpOriginTier(origin shared.BindingOrigin) int {
	switch origin {
	case shared.OriginLocal:
		return 0
	case shared.OriginImport:
		return 1
	case shared.OriginNamespace:
		return 2
	case shared.OriginWildcard:
		return 3 // using static = wildcard-like
	case shared.OriginReexport:
		return 4
	default:
		return 5
	}
}

// CSharpMergeBindings merges two binding sets for a C# scope, keeping the
// highest-precedence (lowest-tier) binding for each name.
// C# shadowing: local > using > using static.
// existing bindings are preferred over incoming when tiers are equal.
//
// Mirrors TS csharpMergeBindings(existing, incoming, scopeID).
// TODO: full implementation — currently concatenates and deduplicates by tier.
func CSharpMergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
	if len(existing) == 0 {
		return incoming
	}
	if len(incoming) == 0 {
		return existing
	}

	// Merge all bindings, sorted by origin tier, then deduplicate by name
	// keeping the first (lowest-tier) occurrence.
	all := append(existing, incoming...)
	sort.SliceStable(all, func(i, j int) bool {
		return csharpOriginTier(all[i].Origin) < csharpOriginTier(all[j].Origin)
	})

	seen := map[string]bool{}
	result := make([]shared.BindingRef, 0, len(all))
	for _, b := range all {
		name := b.Def.QualifiedName
		key := ""
		if name != nil {
			key = *name
		} else {
			key = b.Def.NodeID
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, b)
	}
	return result
}