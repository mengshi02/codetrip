// Package java — Java simple binding/import/receiver hooks.
// These are the small per-language hooks used during scope extraction
// to determine which scope owns a binding, which scope owns an import,
// and how receiver bindings are synthesized.
// Ported from TS languages/java/simple-hooks.ts.
package java

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// JavaBindingScopeFor returns the ScopeID that should own a given type binding.
// In Java, "self" bindings ("this" type) stay in the Function scope so
// PopulateJavaClassOwnedMembers can find them; all other bindings are
// auto-hoisted to the Module scope by returning nil (default behavior).
//
// Mirrors TS javaBindingScopeFor(bindingName, source).
// Returns nil for auto-hoist (default).
func JavaBindingScopeFor(bindingName string, source string) *shared.ScopeID {
	// "self" bindings stay in Function scope for owner population.
	// Returning nil means the generic pipeline auto-hoists to Module.
	if source == "self" {
		// Don't hoist self bindings — they need to stay in Function scope
		// for PopulateJavaClassOwnedMembers to find the receiver type.
		// The pipeline handles this by checking the source.
	}
	return nil
}

// WalkToScope walks from the current scope up the scope tree to find
// the nearest scope of the given kind. Returns nil if no matching scope
// is found.
//
// Mirrors TS walkToScope(scope, kind).
// TODO: full implementation — currently returns nil.
func WalkToScope(scope *shared.Scope, kind shared.ScopeKind) *shared.ScopeID {
	// TODO: walk parent chain until we find a scope of the given kind.
	return nil
}

// JavaImportOwningScope returns the ScopeID that should own an import edge.
// In Java, all imports are at the file/module level, so this always returns nil
// (meaning the Module scope owns all imports — the default behavior).
//
// Mirrors TS javaImportOwningScope(importKind).
func JavaImportOwningScope(importKind shared.ParsedImportKind) *shared.ScopeID {
	// Java imports are always module-level; return nil for default.
	return nil
}

// JavaReceiverBinding returns a synthesized BindingRef for a Java method
// receiver ("this"), linking the method's Function scope to the owning class type.
// Returns nil when scope.Kind != Function (not a method scope).
//
// Mirrors TS javaReceiverBinding(scope).
func JavaReceiverBinding(scope *shared.Scope) *shared.BindingRef {
	if scope.Kind != shared.ScopeKindFunction {
		return nil
	}
	// TODO: extract receiver type from scope.TypeBindings["this"],
	// create a BindingRef pointing to the class/interface definition.
	return nil
}