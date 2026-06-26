package cpp

import (
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// ArityResult represents the result of an arity compatibility check.
type ArityResult int

const (
	ArityCompatible   ArityResult = iota
	ArityUnknown
	ArityIncompatible
)

// CppArityCompatibility checks arity compatibility between a call site and target definition.
// C++ supports overloading and default parameters.
// Ported from GitNexus cpp/arity.ts.
func CppArityCompatibility(defParamCount, defRequiredCount int, defParameterTypes []string, callArity int) ArityResult {
	hasMax := defParamCount > 0
	hasMin := defRequiredCount > 0

	if !hasMax && !hasMin {
		return ArityUnknown
	}
	if callArity < 0 {
		return ArityUnknown
	}

	variadic := false
	for _, t := range defParameterTypes {
		if t == "..." {
			variadic = true
			break
		}
	}

	if hasMin && callArity < defRequiredCount {
		return ArityIncompatible
	}
	if hasMax && callArity > defParamCount && !variadic {
		return ArityIncompatible
	}
	return ArityCompatible
}

// CppScopeArityCompatibility adapts the scope-resolution ArityCompatibility contract
// (callsite, def) → ArityVerdict to the existing CppArityCompatibility function.
// Ported from TS languages/cpp/arity.ts.
func CppScopeArityCompatibility(callsite shared.Callsite, def shared.SymbolDefinition) scope_resolution.ArityVerdict {
	callArity := -1
	if callsite.Arity != nil {
		callArity = *callsite.Arity
	}

	defParamCount := 0
	defRequiredCount := 0
	var defParameterTypes []string
	if len(def.ParameterTypes) > 0 {
		defParameterTypes = def.ParameterTypes
		defParamCount = len(def.ParameterTypes)
		defRequiredCount = *def.RequiredParameterCount
	}

	result := CppArityCompatibility(defParamCount, defRequiredCount, defParameterTypes, callArity)
	switch result {
	case ArityCompatible:
		return scope_resolution.ArityCompatible
	case ArityIncompatible:
		return scope_resolution.ArityIncompatible
	default:
		return scope_resolution.ArityUnknown
	}
}