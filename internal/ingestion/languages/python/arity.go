// Package python — Python arity compatibility check.
// Accommodates *args, **kwargs, and default parameters.
//
// Verdicts:
//   - 'compatible'   — requiredParameterCount <= argCount <= parameterCount,
//     OR the def takes *args (then any argCount >= required is ok).
//   - 'incompatible' — argCount is below required, OR above max with no *args.
//   - 'unknown'      — def metadata is absent / incomplete.
//
// Ported from TS languages/python/arity.ts (pythonArityCompatibility).
package python

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PythonArityCompatibility checks whether a callsite's arity matches a
// Python definition's parameter signature.
//
// Python specifics:
//   - *args / **kwargs detected from parameterTypes (stored as '*args', '**kwargs').
//   - When variadic, any argCount >= required is compatible.
//   - Without variadic, argCount must be between required and parameterCount.
//
// Note: this function uses (def, callsite) order, matching the TS convention.
// The ScopeResolver interface uses (callsite, def) — the adapter swaps arguments.
//
// Mirrors TS pythonArityCompatibility(def, callsite).
func PythonArityCompatibility(def shared.SymbolDefinition, callsite shared.Callsite) scope_resolution.ArityVerdict {
	max := def.ParameterCount
	min := def.RequiredParameterCount
	if max == nil && min == nil {
		return scope_resolution.ArityUnknown
	}

	argCount := -1
	if callsite.Arity != nil {
		argCount = *callsite.Arity
	}
	if argCount < 0 {
		return scope_resolution.ArityUnknown
	}

	// Detect varargs/kwargs from parameterTypes — the Python method extractor
	// stores '*args'/'**kwargs' in this list.
	hasVarArgs := false
	for _, t := range def.ParameterTypes {
		if strings.HasPrefix(t, "*") {
			hasVarArgs = true
			break
		}
	}

	if min != nil && argCount < *min {
		return scope_resolution.ArityIncompatible
	}
	if max != nil && argCount > *max && !hasVarArgs {
		return scope_resolution.ArityIncompatible
	}

	return scope_resolution.ArityCompatible
}