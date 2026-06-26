// Package javascript — JS scope query string + lazy parser/query singletons.
// Subset of the TypeScript scope query compiled against tree-sitter-javascript.
// TypeScript-only node types (interface_declaration, type_alias_declaration,
// enum_declaration, etc.) are dropped because the JS grammar doesn't define them.
//
// Ported from TS languages/javascript/query.ts.
package javascript

import "strings"

// JavaScriptScopeQuery is the tree-sitter scope query for JavaScript.
// It is a subset of the TypeScript query — TypeScript-only node types are dropped.
// Placeholder: the actual query string will be compiled from tree-sitter-javascript.
const JavaScriptScopeQuery = `
;; placeholder — JS scope query compiled against tree-sitter-javascript
(program) @scope.module
`

// JSJSXQuerySuffix is appended when compiling against the JSX grammar for .jsx files.
const JSJSXQuerySuffix = `
;; placeholder — JSX-specific scope patterns
`

// GetJsParser returns a lazy-initialized JS tree-sitter Parser singleton.
// filePath is optional; .jsx files use the JSX grammar variant.
// Returns interface{} placeholder for tree-sitter Parser — full implementation
// will use typed Parser when the binding is available.
func GetJsParser(filePath string) interface{} {
	// TODO: full implementation — lazy Parser singleton for JS grammar
	return nil
}

// GetJsScopeQuery returns a lazy-initialized JS tree-sitter Query singleton.
// filePath is optional; .jsx files append the JSX suffix.
// Returns interface{} placeholder for tree-sitter Query — full implementation
// will use typed Query when the binding is available.
func GetJsScopeQuery(filePath string) interface{} {
	// TODO: full implementation — lazy Query singleton for JS grammar
	return nil
}

// JsCachedTreeMatchesGrammar validates that a cached Tree was produced by
// the JS grammar. Returns true when the tree's language matches the JS grammar.
func JsCachedTreeMatchesGrammar(tree interface{}) bool {
	// TODO: full implementation — check tree.getLanguage() == JS_GRAMMAR
	return true
}

// IsJsxFile returns true when the file should be parsed with the JSX-extended query.
func IsJsxFile(filePath string) bool {
	return strings.HasSuffix(filePath, ".jsx")
}