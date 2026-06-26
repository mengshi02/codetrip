// Package csharp — C# tree-sitter scope query.
// Defines the tree-sitter query pattern used to extract scopes,
// definitions, imports, and type bindings from C# source files.
// Ported from TS languages/csharp/query.ts.
package csharp

// CsharpScopeQuery is the tree-sitter query string for C# scope extraction.
// It captures:
//   - namespace_declaration → Namespace scopes
//   - class_declaration, struct_declaration, interface_declaration → Class scopes
//   - method_declaration, constructor_declaration → Function scopes
//   - using_directive → import edges
//   - variable_declaration, field_declaration → type bindings
//   - this/base → self type binding
//   - return type → return-annotation type binding
//
// TODO: full implementation — currently placeholder.
const CsharpScopeQuery = `
(namespace_declaration
  name: (identifier) @definition.namespace
  body: (declaration_list) @scope.namespace)

(class_declaration
  name: (identifier) @definition.class
  body: (declaration_list) @scope.class)

(struct_declaration
  name: (identifier) @definition.struct
  body: (declaration_list) @scope.class)

(interface_declaration
  name: (identifier) @definition.interface
  body: (declaration_list) @scope.class))

(method_declaration
  name: (identifier) @definition.method
  body: (block) @scope.function)

(constructor_declaration
  body: (block) @scope.function)

(using_directive
  name: (identifier) @import-path)
`

// GetCsharpScopeQuery returns the C# tree-sitter query string.
func GetCsharpScopeQuery() string {
	return CsharpScopeQuery
}