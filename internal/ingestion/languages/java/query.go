// Package java — Java tree-sitter scope query.
// Defines the tree-sitter query pattern used to extract scopes,
// definitions, imports, and type bindings from Java source files.
// Ported from TS languages/java/query.ts.
package java

// JavaScopeQuery is the tree-sitter query string for Java scope extraction.
// It captures:
//   - class_declaration, interface_declaration, enum_declaration → Class scopes
//   - method_declaration, constructor_declaration → Function scopes
//   - import_declaration → import edges
//   - field_declaration, local_variable_declaration → type bindings
//   - return types, parameter types → type bindings
//
// TODO: full implementation — currently placeholder.
const JavaScopeQuery = `
(class_declaration
  name: (identifier) @definition.class
  body: (class_body) @scope.class)

(interface_declaration
  name: (identifier) @definition.interface
  body: (interface_body) @scope.class)

(method_declaration
  name: (identifier) @definition.method
  body: (block) @scope.function)

(import_declaration
  (import_spec
    path: (scoped_identifier) @import-path))

(local_variable_declaration
  (variable_declarator
    name: (identifier) @type-binding.local
    type: (_) @type-binding.type))
`

// GetJavaScopeQuery returns the Java tree-sitter query string.
func GetJavaScopeQuery() string {
	return JavaScopeQuery
}