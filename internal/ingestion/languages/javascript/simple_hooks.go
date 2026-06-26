// Package javascript — Simple hooks for the JavaScript scope-resolution provider.
// jsBindingScopeFor wraps tsBindingScopeFor and adds the JS-only
// @type-binding.class-field hoisting rule. The other two hooks
// (jsImportOwningScope, jsReceiverBinding) are identical to their
// TypeScript counterparts and are re-exported directly.
//
// @type-binding.class-field hoisting lives here (not in tsBindingScopeFor)
// because it is emitted exclusively by synthesizeConstructorFieldBindings
// in captures.go, which is a JavaScript-only synthesis pass. TypeScript uses
// @type-binding.parameter-property for constructor parameter properties instead.
//
// Ported from TS languages/javascript/simple-hooks.ts.
package javascript

import (
	typescript "github.com/mengshi02/codetrip/internal/ingestion/languages/typescript"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JsBindingScopeFor wraps tsBindingScopeFor and additionally hoists
// @type-binding.class-field captures to the enclosing Class scope.
// @type-binding.class-field is anchored inside the constructor body
// (by synthesizeConstructorFieldBindings) so that WalkToScope can walk
// up from the Function (constructor) scope to the Class scope.
func JsBindingScopeFor(decl shared.CaptureMatch, innermost *shared.Scope) *shared.ScopeID {
	if _, ok := decl["@type-binding.class-field"]; ok {
		return typescript.WalkToScope(innermost, shared.ScopeKindClass)
	}
	return typescript.TsBindingScopeFor(decl, innermost)
}

// JsImportOwningScope delegates to TsImportOwningScope (identical behavior).
func JsImportOwningScope(imp shared.ParsedImport) *shared.ScopeID {
	return typescript.TsImportOwningScope(imp)
}

// JsReceiverBinding delegates to TsReceiverBinding (identical behavior).
func JsReceiverBinding(functionScope *shared.Scope) *shared.TypeRef {
	return typescript.TsReceiverBinding(functionScope)
}