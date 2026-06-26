package cpp

// C++ Constraint Filter — filter overload candidates by concept satisfaction.
//
// After extracting constraints, this module filters candidate overloads
// based on whether the callsite's argument types satisfy the constraints.
// This is used by the ConstraintCompatibility hook.
// Ported from TS languages/cpp/constraint-filter.ts.

import (
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CppConstraintCompatibilityHook implements the ScopeResolver.ConstraintCompatibility hook.
// Returns ArityCompatible if the callsite's argument types satisfy the
// definition's constraints, ArityIncompatible if they violate, or
// ArityUnknown if constraint checking is inconclusive.
// TODO: full implementation — wire to constraint satisfaction checking.
func CppConstraintCompatibilityHook(
	callsite shared.ReferenceSite,
	def shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) scope_resolution.ArityVerdict {
	// TODO: check extracted constraints against callsite argument type classes
	return scope_resolution.ArityUnknown
}

// FilterCppOverloadsByConstraint filters a set of candidate definitions
// by removing those whose constraints are not satisfied by the callsite.
// TODO: full implementation
func FilterCppOverloadsByConstraint(
	callsite shared.ReferenceSite,
	candidates []shared.SymbolDefinition,
	ctx shared.ParameterTypeClass,
) []shared.SymbolDefinition {
	return candidates
}