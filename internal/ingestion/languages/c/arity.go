// Package c — C arity compatibility check.
// C has no overloading; variadic functions detected via "..." in parameter types.
// Ported from TS languages/c/arity.ts.
package c

import (
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CArityCompatibility checks whether a callsite's arity matches a definition's
// parameter signature. C variadic functions (with "..." parameter type) are
// handled specially — callsites with more args than the def's max are still
// compatible when the def is variadic.
//
// Mirrors TS cArityCompatibility(callsite, def).
func CArityCompatibility(callsite shared.Callsite, def shared.SymbolDefinition) scope_resolution.ArityVerdict {
	max := def.ParameterCount
	min := def.RequiredParameterCount

	if max == nil && min == nil {
		return scope_resolution.ArityUnknown
	}

	callArity := 0
	if callsite.Arity != nil {
		callArity = *callsite.Arity
	}

	variadic := false
	for _, t := range def.ParameterTypes {
		if t == "..." {
			variadic = true
			break
		}
	}

	if min != nil && callArity < *min {
		return scope_resolution.ArityIncompatible
	}
	if max != nil && callArity > *max && !variadic {
		return scope_resolution.ArityIncompatible
	}
	return scope_resolution.ArityCompatible
}