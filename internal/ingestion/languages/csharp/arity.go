// Package csharp — C# arity compatibility check.
// C# is statically typed; arity compatibility is strict unless
// the method has optional or params (variadic) parameters.
// Ported from TS languages/csharp/arity.ts.
package csharp

import (
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CSharpArityCompatibility checks whether a callsite's arity matches a
// definition's parameter signature. C# optional parameters (with default
// values) and params arrays (variadic) are handled specially.
//
// Mirrors TS csharpArityCompatibility(callsite, def).
// TODO: full implementation — currently returns ArityUnknown.
func CSharpArityCompatibility(callsite shared.Callsite, def shared.SymbolDefinition) scope_resolution.ArityVerdict {
	max := def.ParameterCount
	min := def.RequiredParameterCount
	if max == nil && min == nil {
		return scope_resolution.ArityUnknown
	}
	if callsite.Arity == nil {
		return scope_resolution.ArityUnknown
	}
	arity := *callsite.Arity

	// Check minimum required parameters
	if min != nil && arity < *min {
		return scope_resolution.ArityIncompatible
	}
	// Check maximum parameters (params/variadic may exceed max)
	if max != nil && arity > *max {
		// TODO: check if def has a "params" (variadic) last parameter.
		return scope_resolution.ArityIncompatible
	}
	return scope_resolution.ArityCompatible
}