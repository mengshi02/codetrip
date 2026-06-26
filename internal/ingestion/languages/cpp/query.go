package cpp

import (
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// CppScopeQuery is the unified tree-sitter S-expression query for C++ scope extraction.
// It combines all extraction dimensions (scope, declaration, import, type-binding, reference)
// into a single query, matching the approach used in GitNexus.
//
// C++ specifics beyond C:
//   - class/struct with access specifiers
//   - namespace definitions and namespace aliases
//   - using declarations and using-directives
//   - template declarations, template specializations
//   - constructor/destructor declarations
//   - virtual/override/final specifiers
//   - auto type deduction
//   - Range-based for loops
//   - Lambda expressions
//   - Static/constexpr/inline specifiers
//   - ADL (argument-dependent lookup) references
const CppScopeQuery = `
;; Scopes
(translation_unit) @scope.module
(namespace_definition) @scope.namespace
(class_specifier body: (field_declaration_list)) @scope.class
(struct_specifier body: (field_declaration_list)) @scope.class
(union_specifier body: (field_declaration_list)) @scope.class
(function_definition) @scope.function
(lambda_expression) @scope.function
(compound_statement) @scope.block

;; Declarations — class/struct
(class_specifier
  name: (type_identifier) @declaration.name
  body: (field_declaration_list)) @declaration.class

(struct_specifier
  name: (type_identifier) @declaration.name
  body: (field_declaration_list)) @declaration.class

;; Declarations — typedef struct/union/enum
(type_definition
  type: (struct_specifier body: (field_declaration_list))
  declarator: (type_identifier) @declaration.name) @declaration.struct

(type_definition
  type: (union_specifier body: (field_declaration_list))
  declarator: (type_identifier) @declaration.name) @declaration.union

;; Declarations — enum
(enum_specifier
  name: (type_identifier) @declaration.name) @declaration.enum

(type_definition
  type: (enum_specifier body: (enumerator_list))
  declarator: (type_identifier) @declaration.name) @declaration.enum

;; Declarations — enum class
(enum_specifier
  name: (type_identifier) @declaration.name
  base: (_)) @declaration.enum

;; Declarations — function definition
(function_definition
  declarator: (function_declarator
    declarator: (identifier) @declaration.name)) @declaration.function

(function_definition
  declarator: (function_declarator
    declarator: (qualified_identifier
      name: (identifier) @declaration.name))) @declaration.function

(function_definition
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: (identifier) @declaration.name))) @declaration.function

;; Declarations — method definition
(function_definition
  declarator: (function_declarator
    declarator: (qualified_identifier
      scope: (namespace_identifier) @type-binding.self
      name: (identifier) @declaration.name))) @declaration.method

(function_definition
  declarator: (function_declarator
    declarator: (field_identifier) @declaration.name)) @declaration.method

;; Declarations — constructor
(function_definition
  declarator: (function_declarator
    declarator: (type_identifier) @declaration.name)) @declaration.constructor

;; Declarations — destructor
(function_definition
  declarator: (function_declarator
    declarator: (destructor_name
      name: (identifier) @declaration.name))) @declaration.method

;; Declarations — function declaration (prototype)
(declaration
  declarator: (function_declarator
    declarator: (identifier) @declaration.name)) @declaration.function

(declaration
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: (identifier) @declaration.name))) @declaration.function

;; Declarations — typedef
(type_definition
  declarator: (type_identifier) @declaration.name) @declaration.typedef

;; Declarations — template
(template_declaration
  (function_definition
    declarator: (function_declarator
      declarator: (identifier) @declaration.name))) @declaration.function

(template_declaration
  (class_specifier
    name: (type_identifier) @declaration.name
    body: (field_declaration_list))) @declaration.class

;; Declarations — struct fields
(field_declaration
  declarator: (field_identifier) @declaration.name) @declaration.field

(field_declaration
  declarator: (pointer_declarator
    declarator: (field_identifier) @declaration.name)) @declaration.field

;; Declarations — variables
(declaration
  declarator: (init_declarator
    declarator: (identifier) @declaration.name)) @declaration.variable

(declaration
  declarator: (identifier) @declaration.name) @declaration.variable

(declaration
  declarator: (pointer_declarator
    declarator: (identifier) @declaration.name)) @declaration.variable

(declaration
  declarator: (reference_declarator
    declarator: (identifier) @declaration.name)) @declaration.variable

;; Declarations — using declaration
(using_declaration
  name: (identifier) @declaration.name) @declaration.variable

;; Declarations — namespace alias
(namespace_alias_definition
  name: (identifier) @declaration.name
  value: (_) @declaration.alias-target) @declaration.namespace

;; Declarations — enum constants
(enumerator
  name: (identifier) @declaration.name) @declaration.const

;; Imports — #include
(preproc_include) @import.statement

;; Imports — using namespace
(using_declaration) @import.statement

(namespace_definition
  name: (namespace_identifier) @declaration.name) @declaration.namespace

;; Type bindings — parameter annotations
(parameter_declaration
  type: (_) @type-binding.type
  declarator: (identifier) @type-binding.name) @type-binding.parameter

(parameter_declaration
  type: (_) @type-binding.type
  declarator: (pointer_declarator
    declarator: (identifier) @type-binding.name)) @type-binding.parameter

(parameter_declaration
  type: (_) @type-binding.type
  declarator: (reference_declarator
    declarator: (identifier) @type-binding.name)) @type-binding.parameter

;; Type bindings — variable with type
(declaration
  type: (_) @type-binding.type
  declarator: (init_declarator
    declarator: (identifier) @type-binding.name)) @type-binding.assignment

;; Type bindings — auto
(declaration
  type: (auto) @type-binding.auto
  declarator: (init_declarator
    declarator: (identifier) @type-binding.name)) @type-binding.assignment

;; References — free calls
(call_expression
  function: (identifier) @reference.name) @reference.call.free

;; References — member calls
(call_expression
  function: (field_expression
    argument: (_) @reference.receiver
    field: (field_identifier) @reference.name)) @reference.call.member

;; References — qualified calls
(call_expression
  function: (qualified_identifier
    name: (identifier) @reference.name)) @reference.call.free

;; References — field reads
(field_expression
  argument: (_) @reference.receiver
  field: (field_identifier) @reference.name) @reference.read

;; References — field writes
(assignment_expression
  left: (field_expression
    argument: (_) @reference.receiver
    field: (field_identifier) @reference.name)) @reference.write
`

var (
	cppScopeQueryOnce     sync.Once
	cppScopeQueryCompiled *gotreesitter.Query
	cppScopeQueryErr      error
)

// CppScopeQueryCompiled returns a pre-compiled tree-sitter query for C++ scope extraction.
func CppScopeQueryCompiled() *gotreesitter.Query {
	cppScopeQueryOnce.Do(func() {
		lang := grammars.CppLanguage()
		q, err := gotreesitter.NewQuery(CppScopeQuery, lang)
		if err != nil {
			cppScopeQueryErr = err
			return
		}
		cppScopeQueryCompiled = q
	})
	if cppScopeQueryErr != nil {
		return nil
	}
	return cppScopeQueryCompiled
}