// Package python — Tree-sitter query for Python scope captures (RFC §5.1).
// Exposes the Python scope query string and lazy Parser/Query singletons.
//
// Ported from TS languages/python/query.ts.
package python

// pythonScopeQuery is the tree-sitter scope query for Python.
// Mirrors TS PYTHON_SCOPE_QUERY constant.
// TODO: verify this query against tree-sitter-python grammar; currently
// carries the same capture names as the TS version.
const pythonScopeQuery = `
;; Scopes
(module) @scope.module
(class_definition) @scope.class
(function_definition) @scope.function
(lambda) @scope.function

;; Declarations
(class_definition
  name: (identifier) @declaration.name) @declaration.class

(function_definition
  name: (identifier) @declaration.name) @declaration.function

(assignment
  left: (identifier) @declaration.name) @declaration.variable

;; Declarations: for-loop target
(for_statement
  left: (identifier) @declaration.name) @declaration.variable

;; Imports — single anchor per statement; interpretImport decomposes
(import_statement) @import.statement
(import_from_statement) @import.statement

;; Type bindings (parameter annotations)
(typed_parameter
  (identifier) @type-binding.name
  type: (type) @type-binding.type) @type-binding.parameter

(typed_default_parameter
  (identifier) @type-binding.name
  type: (type) @type-binding.type) @type-binding.parameter

;; Type bindings (return annotations)
(function_definition
  return_type: (type) @type-binding.type) @type-binding.return

;; Type bindings (variable annotations)
(assignment
  left: (identifier) @type-binding.name
  type: (type) @type-binding.type) @type-binding.annotation

;; References
(call
  function: (identifier) @reference.name) @reference.call.free

(call
  function: (attribute
    attribute: (identifier) @reference.name
    object: (_) @reference.receiver)) @reference.call.member
`

// GetPythonScopeQuery returns the Python scope query object.
// In TS this returns a tree-sitter Query object; in Go we return nil
// as the tree-sitter integration is deferred.
//
// Mirrors TS getPythonScopeQuery().
// TODO: return actual tree-sitter Query when parser integration is complete.
func GetPythonScopeQuery() interface{} {
	// TODO: create and return a tree-sitter Query from pythonScopeQuery.
	// Requires tree-sitter Go bindings and Python grammar.
	return nil
}