// Package typescript — TypeScript simple binding/import/receiver hooks.
// These are the small per-language hooks used during scope extraction
// to determine which scope owns a binding, which scope owns an import,
// and how receiver bindings are synthesized.
//
// Key behaviors:
//   - tsBindingScopeFor: method return types hoisted to Module,
//     parameter properties hoisted to Class, var declarations hoisted
//     to nearest Function/Module.
//   - tsImportOwningScope: returns nil (imports are module-level by default)
//   - tsReceiverBinding: looks up 'this' typeBinding on Function scopes
//
// Ported from TS languages/typescript/simple-hooks.ts.
package typescript

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// TsBindingScopeFor returns the ScopeID that should own a given type binding.
// TypeScript-specific scope overrides:
//   - @type-binding.return → hoist to Module scope (method return types)
//   - @type-binding.parameter-property → hoist to Class scope
//   - var declarations → hoist to nearest Function or Module scope
//   - Everything else → nil (default auto-hoist behavior)
//
// Mirrors TS tsBindingScopeFor(decl, innermost, tree): ScopeId | null.
func TsBindingScopeFor(decl shared.CaptureMatch, innermost *shared.Scope) *shared.ScopeID {
	// Method return type: hoist to Module.
	if _, ok := decl["@type-binding.return"]; ok {
		return WalkToScope(innermost, shared.ScopeKindModule)
	}

	// Parameter property: hoist to Class.
	if _, ok := decl["@type-binding.parameter-property"]; ok {
		return WalkToScope(innermost, shared.ScopeKindClass)
	}

	// var declarations: hoist to nearest Function or Module.
	if variable, ok := decl["@declaration.variable"]; ok {
		if isVarDeclaration(variable.Text) {
			fnScope := WalkToScope(innermost, shared.ScopeKindFunction)
			if fnScope != nil {
				return fnScope
			}
			return WalkToScope(innermost, shared.ScopeKindModule)
		}
	}

	return nil
}

// WalkToScope walks up the scope chain from `from` to find the first scope
// whose Kind matches any of `kinds`. Returns the matching scope's ID,
// or nil when no ancestor matches.
//
// Exported so language-specific hook wrappers (e.g. jsBindingScopeFor)
// can reuse it without duplicating the traversal logic.
//
// Mirrors TS walkToScope(from, tree, ...kinds): ScopeId | null.
func WalkToScope(from *shared.Scope, kinds ...shared.ScopeKind) *shared.ScopeID {
	cur := from
	for cur != nil {
		for _, k := range kinds {
			if cur.Kind == k {
				return &cur.ID
			}
		}
		if cur.Parent == nil {
			break
		}
		// TODO: need a ScopeTree to resolve parent scopes.
		// For skeleton, break immediately.
		break
	}
	return nil
}

// isVarDeclaration checks if a capture text starts with "var" keyword
// (vs "let" or "const").
func isVarDeclaration(captureText string) bool {
	return strings.HasPrefix(captureText, "var ") ||
		strings.HasPrefix(captureText, "var\t") ||
		strings.HasPrefix(captureText, "var\n")
}

// TsImportOwningScope returns the ScopeID that should own an import edge.
// TypeScript imports are syntactically top-level; returning nil delegates
// to the central default (nearest Module/Namespace scope).
//
// Mirrors TS tsImportOwningScope(_imp, _innermost, _tree): ScopeId | null.
func TsImportOwningScope(_imp shared.ParsedImport) *shared.ScopeID {
	return nil
}

// TsReceiverBinding looks up `this` on the function scope's type bindings.
// Returns nil for static methods, free functions, non-Function scopes.
// For instance methods, returns the TypeRef for the synthesized `this` binding.
//
// Mirrors TS tsReceiverBinding(functionScope): TypeRef | null.
func TsReceiverBinding(functionScope *shared.Scope) *shared.TypeRef {
	if functionScope.Kind != shared.ScopeKindFunction {
		return nil
	}
	// TODO: look up "this" in functionScope.TypeBindings.
	// return functionScope.TypeBindings["this"] or nil
	return nil
}