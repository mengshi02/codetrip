// Package java — Java binding merge precedence.
// Implements per-scope binding merge for Java: local bindings take precedence
// over import bindings, which take precedence over wildcard/reexport bindings.
// When two bindings have the same name, the lower-tier (higher-number) origin
// is dropped.
// Ported from TS languages/java/merge-bindings.ts.
package java

import (
	"sort"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// javaOriginTier maps BindingOrigin to its merge precedence tier for Java.
// Lower tier = higher precedence (local wins over import, import wins over wildcard).
func javaOriginTier(origin shared.BindingOrigin) int {
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

// JavaMergeBindings merges two binding sets for a Java scope, keeping the
// highest-precedence (lowest-tier) binding for each name.
// existing bindings are preferred over incoming when tiers are equal.
// Merge precedence: local > import > namespace > reexport > wildcard.
//
// Mirrors TS javaMergeBindings(existing, incoming, scopeID).
// TODO: full implementation — currently concatenates and deduplicates by tier.
func JavaMergeBindings(existing []shared.BindingRef, incoming []shared.BindingRef, scopeID string) []shared.BindingRef {
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
		return javaOriginTier(all[i].Origin) < javaOriginTier(all[j].Origin)
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