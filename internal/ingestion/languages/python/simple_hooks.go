// Package python — Python simple binding/import/receiver hooks.
// These are the small per-language hooks used during scope extraction
// to determine which scope owns a binding, which scope owns an import,
// and how receiver bindings are synthesized.
//
// Each hook exists to make the provider's choice explicit (rather than
// relying on "absence == default") so reviewers don't have to re-derive
// the analysis.
//
// Ported from TS languages/python/simple-hooks.ts.
package python

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// PythonFunctionDefinitionLabel determines the NodeLabel for a Python
// function definition. If the function is defined inside a class body,
// it should be labeled 'Method'; otherwise 'Function'.
//
// In TS, this checks whether the function's parent (ignoring decorated_definition
// wrapper) is a class_definition node. In Go, the CaptureMatch carries the
// declaration context from tree-sitter.
//
// Mirrors TS pythonFunctionDefinitionLabel(functionNode, defaultLabel).
// TODO: full implementation — requires tree-sitter node traversal via CaptureMatch.
func PythonFunctionDefinitionLabel(decl shared.CaptureMatch) shared.NodeLabel {
	// TODO: check if function is inside a class_definition body.
	// If so, return shared.LabelMethod; otherwise return shared.LabelFunction.
	// The CaptureMatch should carry enough context to determine the parent scope.
	return shared.LabelFunction
}

// PythonBindingScopeFor returns the ScopeID that should own a given type binding.
// Python has no block scope, so the central extractor's "innermost enclosing scope"
// default is already correct: `for x in …` creates `x` in the enclosing
// function/module scope (because we never emit a @scope.block for the for-loop body),
// comprehension variables stay in their expression context, etc.
// Returns nil to delegate to the default behavior.
//
// Mirrors TS pythonBindingScopeFor(_decl, _innermost, _tree).
func PythonBindingScopeFor(decl shared.CaptureMatch, innermost *shared.Scope, tree *shared.ScopeTree) *shared.ScopeID {
	// Python has no block scope — innermost enclosing scope default is correct.
	// Return nil to delegate to the central extractor's default.
	return nil
}

// PythonImportOwningScope returns the ScopeID that should own an import edge.
// Function-local `from x import Y` should attach the binding to the function
// scope, not the module. Class-body imports (rare but legal —
// `class A: import x` makes `x` a class attribute) attach to the class.
// Module-level imports delegate to the central default.
//
// Mirrors TS pythonImportOwningScope(_imp, innermost, _tree).
func PythonImportOwningScope(imp shared.ParsedImport, innermost *shared.Scope, tree *shared.ScopeTree) *shared.ScopeID {
	// Function-local and class-body imports attach to the innermost scope.
	if innermost != nil && (innermost.Kind == shared.ScopeKindFunction || innermost.Kind == shared.ScopeKindClass) {
		return &innermost.ID
	}
	// Module-level imports delegate to the central default.
	return nil
}

// PythonReceiverBinding looks up `self` or `cls` in the function scope's
// type bindings. Returns nil for free functions (no self/cls) and for
// non-Function scopes.
//
// Mirrors TS pythonReceiverBinding(functionScope).
func PythonReceiverBinding(functionScope *shared.Scope) *shared.TypeRef {
	if functionScope == nil || functionScope.Kind != shared.ScopeKindFunction {
		return nil
	}
	// Look up self or cls in the function scope's type bindings.
	if tb, ok := functionScope.TypeBindings["self"]; ok {
		return &tb
	}
	if tb, ok := functionScope.TypeBindings["cls"]; ok {
		return &tb
	}
	return nil
}