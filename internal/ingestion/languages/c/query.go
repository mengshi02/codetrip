package c

import (
	"sync"

	"github.com/odvcencio/gotreesitter"
	"github.com/odvcencio/gotreesitter/grammars"
)

// CScopeQuery is the unified tree-sitter S-expression query for C scope extraction.
// It combines all extraction dimensions (scope, declaration, import, type-binding, reference)
// into a single query, matching GitNexus's approach.
const CScopeQuery = `
;; Scopes
(translation_unit) @scope.module
(struct_specifier) @scope.class
(union_specifier) @scope.class
(function_definition) @scope.function
(compound_statement) @scope.block
(if_statement) @scope.block
(for_statement) @scope.block
(while_statement) @scope.block
(do_statement) @scope.block
(switch_statement) @scope.block
(case_statement) @scope.block

;; Declarations — struct (named)
(struct_specifier
  name: (type_identifier) @declaration.name
  body: (field_declaration_list)) @declaration.struct

;; Declarations — struct (typedef struct { ... } Name)
(type_definition
  type: (struct_specifier
    body: (field_declaration_list))
  declarator: (type_identifier) @declaration.name) @declaration.struct

;; Declarations — union (named)
(union_specifier
  name: (type_identifier) @declaration.name
  body: (field_declaration_list)) @declaration.union

;; Declarations — union (typedef union { ... } Name)
(type_definition
  type: (union_specifier
    body: (field_declaration_list))
  declarator: (type_identifier) @declaration.name) @declaration.union

;; Declarations — enum
(enum_specifier
  name: (type_identifier) @declaration.name) @declaration.enum

;; Declarations — enum (typedef enum { ... } Name)
(type_definition
  type: (enum_specifier
    body: (enumerator_list))
  declarator: (type_identifier) @declaration.name) @declaration.enum

;; Declarations — function definition
(function_definition
  declarator: (function_declarator
    declarator: (identifier) @declaration.name)) @declaration.function

;; Declarations — function definition with pointer return
(function_definition
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: (identifier) @declaration.name))) @declaration.function

;; Declarations — function declaration (prototype)
(declaration
  declarator: (function_declarator
    declarator: (identifier) @declaration.name)) @declaration.function

;; Declarations — function declaration with pointer return (prototype)
(declaration
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: (identifier) @declaration.name))) @declaration.function

;; Declarations — typedef
(type_definition
  declarator: (type_identifier) @declaration.name) @declaration.typedef

;; Declarations — typedef for function pointers
(type_definition
  declarator: (function_declarator
    declarator: (parenthesized_declarator
      (pointer_declarator
        declarator: (type_identifier) @declaration.name)))) @declaration.typedef

;; Declarations — struct fields
(field_declaration
  declarator: (field_identifier) @declaration.name) @declaration.field

;; Declarations — struct fields (pointer)
(field_declaration
  declarator: (pointer_declarator
    declarator: (field_identifier) @declaration.name)) @declaration.field

;; Declarations — variables (with initializer)
(declaration
  declarator: (init_declarator
    declarator: (identifier) @declaration.name)) @declaration.variable

;; Declarations — variables (without initializer)
(declaration
  declarator: (identifier) @declaration.name) @declaration.variable

(declaration
  declarator: (pointer_declarator
    declarator: (identifier) @declaration.name)) @declaration.variable

;; Declarations — macro definitions
(preproc_def
  name: (identifier) @declaration.name) @declaration.macro

(preproc_function_def
  name: (identifier) @declaration.name) @declaration.macro

;; Declarations — enum constants
(enumerator
  name: (identifier) @declaration.name) @declaration.const

;; Imports
(preproc_include) @import.statement

;; Type bindings — parameter annotations
(parameter_declaration
  type: (_) @type-binding.type
  declarator: (identifier) @type-binding.name) @type-binding.parameter

;; Type bindings — variable with type (init_declarator)
(declaration
  type: (_) @type-binding.type
  declarator: (init_declarator
    declarator: (identifier) @type-binding.name)) @type-binding.assignment

;; References — free calls
(call_expression
  function: (identifier) @reference.name) @reference.call.free

;; References — member calls via pointer (ptr->func())
(call_expression
  function: (field_expression
    argument: (_) @reference.receiver
    field: (field_identifier) @reference.name)) @reference.call.member

;; References — field reads
(field_expression
  argument: (_) @reference.receiver
  field: (field_identifier) @reference.name) @reference.read

;; References — field writes (assignment)
(assignment_expression
  left: (field_expression
    argument: (_) @reference.receiver
    field: (field_identifier) @reference.name)) @reference.write
`

var (
	cScopeQueryOnce     sync.Once
	cScopeQueryCompiled *gotreesitter.Query
	cScopeQueryErr      error
)

// CScopeQueryCompiled returns a pre-compiled tree-sitter query for C scope extraction.
// The query is compiled on first call and cached for subsequent calls.
func CScopeQueryCompiled() *gotreesitter.Query {
	cScopeQueryOnce.Do(func() {
		lang := grammars.CLanguage()
		q, err := gotreesitter.NewQuery(CScopeQuery, lang)
		if err != nil {
			cScopeQueryErr = err
			return
		}
		cScopeQueryCompiled = q
	})
	if cScopeQueryErr != nil {
		return nil
	}
	return cScopeQueryCompiled
}