package cpp

// C++ Two-Phase Lookup — template name lookup semantics.
//
// C++ template name lookup has two phases:
//   Phase 1: At template definition time, non-dependent names are resolved.
//   Phase 2: At template instantiation time, dependent names are resolved.
//
// Dependent names are those whose resolution depends on template parameters
// (e.g. T::method, where T is a template parameter). They cannot be resolved
// until the template is instantiated with concrete types.
//
// This module provides lookup primitives for two-phase name resolution.
// Ported from TS languages/cpp/two-phase-lookup.ts.

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CppLookupPhase discriminates the two template lookup phases.
type CppLookupPhase int

const (
	PhaseDefinition      CppLookupPhase = 0 // non-dependent name lookup at definition time
	PhaseInstantiation   CppLookupPhase = 1 // dependent name lookup at instantiation time
)

// IsCppDependentName checks whether a name is dependent on template parameters.
// Dependent names include member access through a dependent type (T::x, t.member)
// and calls to dependent functions.
// TODO: full implementation — parse template parameter references.
func IsCppDependentName(name string, templateParams []string) bool {
	return false // stub
}

// ResolveCppNonDependentName resolves non-dependent names at template definition time.
// These names are resolved using the usual scope lookup rules.
// TODO: full implementation
func ResolveCppNonDependentName(name string, scopeID shared.ScopeID, parsedFiles []*shared.ParsedFile) []string {
	return nil
}

// ResolveCppDependentName resolves dependent names at template instantiation time.
// These names are resolved using ADL + MRO chain + dependent base class lookup.
// TODO: full implementation
func ResolveCppDependentName(name string, instantiatedTypes []shared.SymbolDefinition) []string {
	return nil
}