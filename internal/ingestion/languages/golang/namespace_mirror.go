// Package golang — Go namespace type-binding mirroring.
// In Go, the module (file) scope mirrors type bindings from its
// child Class/Function scopes so that cross-file type lookups
// can find struct/interface types at the module level.
// Ported from TS languages/go/namespace-mirror.ts.
package golang

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// MirrorGoNamespaceTypeBindings hoists type bindings from child scopes
// up to the module scope so that cross-file type lookups can find them.
// This is needed because Go struct/interface method receiver types
// are stored in Function scopes, but cross-file resolution needs them
// at the Module level.
//
// Mirrors TS mirrorGoNamespaceTypeBindings(parsed).
func MirrorGoNamespaceTypeBindings(parsed *shared.ParsedFile) {
	if parsed == nil {
		return
	}

	// Find the module scope (root scope).
	var moduleScope *shared.Scope
	for _, scope := range parsed.Scopes {
		if scope.Kind == shared.ScopeKindModule {
			moduleScope = scope
			break
		}
	}
	if moduleScope == nil {
		return
	}

	// Ensure TypeBindings map is initialized.
	if moduleScope.TypeBindings == nil {
		moduleScope.TypeBindings = make(map[string]shared.TypeRef)
	}

	// Walk child scopes and hoist typeBindings with source=self/annotation
	// up to the ModuleScope. Only hoist if the module scope doesn't already
	// have a binding for that name (child bindings take precedence by recency,
	// but module-level should not be overwritten).
	for _, scope := range parsed.Scopes {
		// Skip the module scope itself.
		if scope.Kind == shared.ScopeKindModule {
			continue
		}
		// Only hoist from Class and Function scopes.
		if scope.Kind != shared.ScopeKindClass && scope.Kind != shared.ScopeKindFunction {
			continue
		}
		for name, tr := range scope.TypeBindings {
			// Only hoist self (receiver) and annotation (declared type) bindings.
			if tr.Source != shared.TypeRefSourceSelf &&
				tr.Source != shared.TypeRefSourceAnnotation &&
				tr.Source != shared.TypeRefSourceReturnAnnotation {
				continue
			}
			// Don't overwrite existing module-level binding.
			if _, exists := moduleScope.TypeBindings[name]; exists {
				continue
			}
			moduleScope.TypeBindings[name] = tr
		}
	}
}