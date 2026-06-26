// Package typescript — TypeScript tree-sitter scope query.
// Defines the tree-sitter query pattern used to extract scopes,
// definitions, imports, type bindings, and references from TypeScript
// and TSX source files.
//
// TypeScript specifics that shape this query:
//   - Dual grammar selection: .ts files use the typescript grammar;
//     .tsx files use the tsx grammar (JSX superset).
//   - Namespaces use internal_module node type.
//   - this/super are named nodes (not anonymous tokens like C#).
//   - Dynamic imports: call_expression with (import) function node.
//   - Function overloads: function_signature + function_declaration.
//   - Parameter properties: constructor(public name: string).
//   - Enum: dual type+value, emits @scope.class + @declaration.enum.
//
// Ported from TS languages/typescript/query.ts (1112 lines).
package typescript

// TypeScriptScopeQuery is the tree-sitter query string for TypeScript
// scope extraction. The full query captures:
//   - Scopes: module, namespace, class, function
//   - Declarations: class, interface, enum, type, namespace, function,
//     method, constructor, property, variable
//   - Imports: import statements, dynamic imports, re-exports
//   - Type bindings: parameter annotations, variable annotations,
//     return types, constructor inference
//   - References: call sites, member writes
//
// TODO: full implementation — currently placeholder.
const TypeScriptScopeQuery = `
(program) @scope.module
; TODO: fill with full scope query (~1000 lines)
`

// TSXJSXQuerySuffix contains the JSX-specific patterns that are appended
// to TypeScriptScopeQuery when parsing .tsx files. These patterns capture
// JSX element/component scopes that would be invalid in the plain TS grammar.
// TODO: full implementation — currently placeholder.
const TSXJSXQuerySuffix = `
; TODO: fill with JSX patterns
`

// GetTsScopeQuery returns the appropriate tree-sitter query for the given
// file path. .tsx files get the TSX grammar query (TypeScriptScopeQuery +
// TSXJSXQuerySuffix); .ts files get the plain TypeScript grammar query.
//
// Mirrors TS getTsScopeQuery(filePath).
// TODO: full implementation — currently returns empty query.
func GetTsScopeQuery(filePath string) interface{} {
	// TODO: lazy-init parser/query singletons; select grammar by extension.
	// If filePath ends with .tsx, use TSX grammar; otherwise use TS grammar.
	return nil
}

// GetTsParser returns a tree-sitter Parser configured with the appropriate
// TypeScript/TSX grammar for the given file path.
//
// Mirrors TS getTsParser(filePath).
// TODO: full implementation — currently returns nil.
func GetTsParser(filePath string) interface{} {
	// TODO: lazy-init parser singleton; select grammar by extension.
	return nil
}

// TsCachedTreeMatchesGrammar validates that a cached tree-sitter Tree was
// produced by the grammar matching the given file path (TSX vs TypeScript).
// Returns true if the tree's language matches, or if the tree's language
// cannot be determined (backwards-compatible fallback).
//
// Mirrors TS tsCachedTreeMatchesGrammar(tree, filePath).
func TsCachedTreeMatchesGrammar(tree interface{}, filePath string) bool {
	// TODO: compare tree.getLanguage() against TS/TSX grammar objects.
	return true // fallback: assume compatible
}