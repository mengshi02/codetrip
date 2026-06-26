// Package golang — Go range-variable type binding population.
// Go range clauses (for x := range expr) infer the type of the
// iteration variable from the collection expression. This hook
// populates those inferred type bindings in the loop scope.
// Ported from TS languages/go/range-binding.ts.
package golang

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// PopulateGoRangeBindings inspects Go range-loop scopes and populates
// type bindings for the iteration variables (key/value) based on the
// type of the range expression.
//
// Mirrors TS populateGoRangeBindings(parsed).
func PopulateGoRangeBindings(parsed *shared.ParsedFile) {
	if parsed == nil {
		return
	}

	for _, scope := range parsed.Scopes {
		// Only Block scopes can be range-loop bodies.
		if scope.Kind != shared.ScopeKindBlock {
			continue
		}

		// Look for type bindings that indicate range iteration.
		// In the capture phase, @type-binding.range-key and
		// @type-binding.range-value captures mark these.
		// If the scope already has these bindings, skip.
		if len(scope.TypeBindings) == 0 {
			continue
		}

		// Check for range-key and range-value type bindings.
		// These are synthesized by the capture phase from for_range
		// constructs and stored as assignment-inferred source.
		rangeKey, hasKey := scope.TypeBindings["range-key"]
		rangeValue, hasValue := scope.TypeBindings["range-value"]

		if !hasKey && !hasValue {
			continue
		}

		// Normalize the type names if present.
		if hasKey && rangeKey.RawName != "" {
			rangeKey.RawName = NormalizeGoTypeName(rangeKey.RawName)
			scope.TypeBindings["range-key"] = rangeKey
		}
		if hasValue && rangeValue.RawName != "" {
			rangeValue.RawName = NormalizeGoTypeName(rangeValue.RawName)
			scope.TypeBindings["range-value"] = rangeValue
		}
	}
}