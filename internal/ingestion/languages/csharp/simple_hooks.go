// Package csharp — C# simple binding/import/receiver hooks.
// These are the small per-language hooks used during scope extraction
// to determine which scope owns a binding, which scope owns an import,
// and how receiver bindings are synthesized.
// Ported from TS languages/csharp/simple-hooks.ts.
package csharp

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// CsharpBindingScopeFor returns the ScopeID that should own a given type
// binding. In C#, "self" bindings (this/base receiver types) stay in the
// Function scope for owner population; all other bindings are auto-hoisted
// to the Module scope by returning nil (default behavior).
//
// Mirrors TS csharpBindingScopeFor(bindingName, source).
// Returns nil for auto-hoist (default).
func CsharpBindingScopeFor(bindingName string, source string) *shared.ScopeID {
	// "self" bindings stay in Function scope for owner population.
	// Returning nil means the generic pipeline auto-hoists to Module.
	if source == "self" {
		// Don't hoist self bindings — they need to stay in Function scope.
		// The pipeline handles this by checking the source.
		return nil
	}
	return nil
}

// WalkToScope determines the target scope for walking up the scope chain
// when resolving a binding. In C#, namespace imports should walk to
// the Namespace scope, not the Module scope.
//
// Mirrors TS walkToScope(scopeID, kind).
// TODO: full implementation — currently returns nil (use parent scope).
func WalkToScope(scopeID shared.ScopeID, kind shared.ScopeKind) *shared.ScopeID {
	// TODO: implement C#-specific scope chain walking.
	// Namespace imports should walk to the Namespace scope.
	return nil
}

// CsharpImportOwningScope returns the ScopeID that should own an import edge.
// In C#, "using" namespace imports are owned by the file's Module scope,
// while "using static" imports are also at Module scope level.
// Returns nil for default (Module scope owns all imports).
//
// Mirrors TS csharpImportOwningScope(importKind).
func CsharpImportOwningScope(importKind shared.ParsedImportKind) *shared.ScopeID {
	// C# imports are always module-level; return nil for default.
	return nil
}

// CsharpReceiverBinding returns a synthesized BindingRef for a C# method
// receiver, linking the method's Function scope to the owning class type.
// Returns nil when scope.Kind != Function (not a method scope).
//
// Mirrors TS csharpReceiverBinding(scope).
func CsharpReceiverBinding(scope *shared.Scope) *shared.BindingRef {
	return SynthesizeCsharpReceiverBinding(scope)
}