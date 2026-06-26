// Package golang — Go simple binding/import/receiver hooks.
// These are the small per-language hooks used during scope extraction
// to determine which scope owns a binding, which scope owns an import,
// and how receiver bindings are synthesized.
// Ported from TS languages/go/simple-hooks.ts.
package golang

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// GoBindingScopeFor returns the ScopeID that should own a given type binding.
// In Go, "self" bindings (method receiver types) stay in the Function scope
// so PopulateGoOwners can find them; all other bindings are auto-hoisted
// to the Module scope by returning nil (default behavior).
//
// Mirrors TS goBindingScopeFor(bindingName, source).
func GoBindingScopeFor(bindingName string, source string) *shared.ScopeID {
	// "self" bindings stay in Function scope for owner population.
	// Returning nil means the generic pipeline auto-hoists to Module.
	if source == "self" {
		// We need to signal "don't hoist" but the hook contract only
		// supports returning nil (auto-hoist) or a concrete ScopeID.
		// The scope_extractor checks source=="self" directly before
		// hoisting, so returning nil here is correct — the pipeline
		// knows not to hoist self bindings for Go.
		return nil
	}
	return nil
}

// GoImportOwningScope returns the ScopeID that should own an import edge.
// In Go, all imports are at the file/module level, so this always returns nil
// (meaning the Module scope owns all imports — the default behavior).
//
// Mirrors TS goImportOwningScope(importKind).
func GoImportOwningScope(importKind shared.ParsedImportKind) *shared.ScopeID {
	// Go imports are always module-level; return nil for default.
	return nil
}

// GoReceiverBinding returns a synthesized BindingRef for a Go method
// receiver, linking the method's Function scope to the owning struct type.
// Returns nil when scope.Kind != Function (not a method scope) or when
// no self type binding is found.
//
// Mirrors TS goReceiverBinding(scope).
func GoReceiverBinding(scope *shared.Scope) *shared.BindingRef {
	if scope == nil || scope.Kind != shared.ScopeKindFunction {
		return nil
	}

	// Look for the "self" type binding in this Function scope.
	typeBinding, ok := scope.TypeBindings["self"]
	if !ok {
		return nil
	}

	rawName := typeBinding.RawName
	if rawName == "" {
		return nil
	}

	// Create a BindingRef pointing to the receiver type's definition.
	// The qualified name matches what NormalizeGoTypeName would produce.
	normalized := NormalizeGoTypeName(rawName)
	qname := normalized
	return &shared.BindingRef{
		Def: shared.SymbolDefinition{
			NodeID:        string(scope.ID) + ":receiver:" + normalized,
			Type:          shared.LabelType,
			QualifiedName: &qname,
		},
		Origin: shared.OriginLocal,
	}
}