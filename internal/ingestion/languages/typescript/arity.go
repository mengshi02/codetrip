// Package typescript — TypeScript arity compatibility check.
// Accommodates rest parameters (...args), optional (p?: T),
// and defaulted (p: T = …) parameters.
// Ported from TS languages/typescript/arity.ts.
package typescript

import (
	"strings"

	scope_resolution "github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// TypeScriptArityCompatibility checks whether a callsite's arity matches
// a definition's parameter signature. TypeScript-specific semantics:
//
//   - Rest parameters (...args) make parameterCount undefined (max unknown).
//   - Optional/defaulted parameters contribute to optionalCount; requiredParameterCount
//     excludes them.
//   - A literal 'params' marker in parameterTypes signals variadic dispatch.
//
// Mirrors TS typescriptArityCompatibility(def, callsite).
func TypeScriptArityCompatibility(def shared.SymbolDefinition, callsite shared.Callsite) scope_resolution.ArityVerdict {
	max := def.ParameterCount
	min := def.RequiredParameterCount
	if max == nil && min == nil {
		return scope_resolution.ArityUnknown
	}

	if callsite.Arity != nil {
		argCount := *callsite.Arity
		if argCount < 0 {
			return scope_resolution.ArityUnknown
		}

		// Variadic detection: 'params' marker in parameterTypes
		hasRest := false
		for _, t := range def.ParameterTypes {
			if t == "params" || strings.HasPrefix(t, "params ") {
				hasRest = true
				break
			}
		}

		if min != nil && argCount < *min {
			return scope_resolution.ArityIncompatible
		}
		if max != nil && argCount > *max && !hasRest {
			return scope_resolution.ArityIncompatible
		}
	}

	return scope_resolution.ArityCompatible
}