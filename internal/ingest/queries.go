package ingest

import "strings"

// Tree-sitter S-expression queries for extracting code definitions, imports, calls, and heritage.

// LanguageQueries returns the tree-sitter query string for a given language ID.
func LanguageQueries(lang string) string {
	switch lang {
	case "typescript":
		return TypeScriptQueries
	case "tsx":
		return TypeScriptQueries
	case "javascript":
		return JavaScriptQueries
	case "python":
		return PythonQueries
	case "java":
		return JavaQueries
	case "c":
		return CQueries
	case "go":
		return GoQueries
	case "cpp":
		return enhancedCPPQueries()
	case "csharp":
		return CSharpQueries
	case "rust":
		return RustQueries
	case "php":
		return PHPQueries
	case "kotlin":
		return KotlinQueries
	case "swift":
		return SwiftQueries
	default:
		return ""
	}
}

// enhancedCPPQueries captures the exact inline function definition and the
// field-initializer call forms used by the validated C++ analyzer.
func enhancedCPPQueries() string {
	query := CPPQueries
	old := `(field_declaration_list
  (function_definition
    declarator: (function_declarator
      declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name))) @definition.method`
	new := `(field_declaration_list
  (function_definition
    declarator: (function_declarator
      declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name)) @definition.method)`
	query = strings.Replace(query, old, new, 1)
	return query + `
(field_initializer (field_identifier) @call.name (argument_list)) @call
(field_initializer (qualified_identifier name: (identifier) @call.name) (argument_list)) @call
`
}

const TypeScriptQueries = `
(class_declaration
  name: (type_identifier) @name) @definition.class

(interface_declaration
  name: (type_identifier) @name) @definition.interface

(function_declaration
  name: (identifier) @name) @definition.function


(method_definition
  name: (property_identifier) @name) @definition.method

(method_signature
  name: (property_identifier) @name) @definition.method

(lexical_declaration
  (variable_declarator
    name: (identifier) @name
    value: (arrow_function))) @definition.function

(lexical_declaration
  (variable_declarator
    name: (identifier) @name
    value: (function_expression))) @definition.function

(export_statement
  declaration: (lexical_declaration
    (variable_declarator
      name: (identifier) @name
      value: (arrow_function)))) @definition.function

(export_statement
  declaration: (lexical_declaration
    (variable_declarator
      name: (identifier) @name
      value: (function_expression)))) @definition.function

(import_statement
  source: (string) @import.source) @import

(export_statement
  source: (string) @import.source) @import

(call_expression
  function: (identifier) @call.name) @call

(call_expression
  function: (member_expression
    property: (property_identifier) @call.name)) @call

(new_expression
  constructor: (identifier) @call.name) @call

(class_declaration
  name: (type_identifier) @heritage.class
  (class_heritage
    (extends_clause
      value: (identifier) @heritage.extends))) @heritage

(class_declaration
  name: (type_identifier) @heritage.class
  (class_heritage
    (implements_clause
      (type_identifier) @heritage.implements))) @heritage.impl
`

const JavaScriptQueries = `
(class_declaration
  name: (identifier) @name) @definition.class

(function_declaration
  name: (identifier) @name) @definition.function


(method_definition
  name: (property_identifier) @name) @definition.method

(lexical_declaration
  (variable_declarator
    name: (identifier) @name
    value: (arrow_function))) @definition.function

(lexical_declaration
  (variable_declarator
    name: (identifier) @name
    value: (function_expression))) @definition.function

(export_statement
  declaration: (lexical_declaration
    (variable_declarator
      name: (identifier) @name
      value: (arrow_function)))) @definition.function

(export_statement
  declaration: (lexical_declaration
    (variable_declarator
      name: (identifier) @name
      value: (function_expression)))) @definition.function

(import_statement
  source: (string) @import.source) @import

(export_statement
  source: (string) @import.source) @import

(call_expression
  function: (identifier) @call.name) @call

(call_expression
  function: (member_expression
    property: (property_identifier) @call.name)) @call

(new_expression
  constructor: (identifier) @call.name) @call

(class_declaration
  name: (identifier) @heritage.class
  (class_heritage
    (identifier) @heritage.extends)) @heritage
`

const PythonQueries = `
(class_definition
  name: (identifier) @name) @definition.class

(function_definition
  name: (identifier) @name) @definition.function

(import_statement
  name: (dotted_name) @import.source) @import

(import_from_statement
  module_name: [(dotted_name) (relative_import)] @import.source
  name: (dotted_name
    (identifier) @import.exported_name @import.name)) @import

(import_from_statement
  module_name: [(dotted_name) (relative_import)] @import.source
  name: (aliased_import
    name: (dotted_name (identifier) @import.exported_name)
    alias: (identifier) @import.name)) @import

(import_from_statement
  module_name: [(dotted_name) (relative_import)] @import.source
  (wildcard_import)) @import

(call
  function: (identifier) @call.name) @call

(call
  function: (attribute
    attribute: (identifier) @call.name)) @call

(class_definition
  name: (identifier) @heritage.class
  superclasses: (argument_list
    (identifier) @heritage.extends)) @heritage

(class_definition
  name: (identifier) @heritage.class
  superclasses: (argument_list
    (attribute
      attribute: (identifier) @heritage.extends))) @heritage
`

const JavaQueries = `
(class_declaration name: (identifier) @name) @definition.class
(interface_declaration name: (identifier) @name) @definition.interface
(enum_declaration name: (identifier) @name) @definition.enum
(annotation_type_declaration name: (identifier) @name) @definition.annotation

(method_declaration name: (identifier) @name) @definition.method
(constructor_declaration name: (identifier) @name) @definition.constructor

(import_declaration (_) @import.source) @import

(method_invocation name: (identifier) @call.name) @call
(method_invocation object: (_) name: (identifier) @call.name) @call

(object_creation_expression type: (type_identifier) @call.name) @call

(class_declaration name: (identifier) @heritage.class
  (superclass (type_identifier) @heritage.extends)) @heritage

(class_declaration name: (identifier) @heritage.class
  (super_interfaces (type_list (type_identifier) @heritage.implements))) @heritage.impl
`

const CQueries = `
(function_definition declarator: (function_declarator declarator: (identifier) @name)) @definition.function
(declaration declarator: (function_declarator declarator: (identifier) @name)) @definition.function

(function_definition declarator: (pointer_declarator declarator: (function_declarator declarator: (identifier) @name))) @definition.function
(declaration declarator: (pointer_declarator declarator: (function_declarator declarator: (identifier) @name))) @definition.function

(function_definition declarator: (pointer_declarator declarator: (pointer_declarator declarator: (function_declarator declarator: (identifier) @name)))) @definition.function

(struct_specifier name: (type_identifier) @name) @definition.struct
(union_specifier name: (type_identifier) @name) @definition.union
(enum_specifier name: (type_identifier) @name) @definition.enum
(type_definition declarator: (type_identifier) @name) @definition.typedef

(preproc_function_def name: (identifier) @name) @definition.macro
(preproc_def name: (identifier) @name) @definition.macro

(preproc_include path: (_) @import.source) @import

(call_expression function: (identifier) @call.name) @call
(call_expression function: (field_expression field: (field_identifier) @call.name)) @call
`

const GoQueries = `
(function_declaration name: (identifier) @name) @definition.function
(method_declaration name: (field_identifier) @name) @definition.method

(type_declaration (type_spec name: (type_identifier) @name type: (struct_type))) @definition.struct
(type_declaration (type_spec name: (type_identifier) @name type: (interface_type))) @definition.interface

(import_declaration (import_spec path: (interpreted_string_literal) @import.source)) @import
(import_declaration (import_spec_list (import_spec path: (interpreted_string_literal) @import.source))) @import

(type_declaration
  (type_spec
    name: (type_identifier) @heritage.class
    type: (struct_type
        (field_declaration_list
        (field_declaration
          !name
          type: (type_identifier) @heritage.extends))))) @definition.struct

(call_expression function: (identifier) @call.name) @call
(call_expression function: (selector_expression field: (field_identifier) @call.name)) @call

(composite_literal type: (type_identifier) @call.name) @call
`

const CPPQueries = `
(class_specifier name: (type_identifier) @name) @definition.class
(struct_specifier name: (type_identifier) @name) @definition.struct
(namespace_definition name: (namespace_identifier) @name) @definition.namespace
(enum_specifier name: (type_identifier) @name) @definition.enum

(type_definition declarator: (type_identifier) @name) @definition.typedef
(alias_declaration name: (type_identifier) @name) @definition.type
(union_specifier name: (type_identifier) @name) @definition.union

(preproc_function_def name: (identifier) @name) @definition.macro
(preproc_def name: (identifier) @name) @definition.macro

(function_definition declarator: (function_declarator declarator: (identifier) @name)) @definition.function
(function_definition declarator: (function_declarator declarator: (qualified_identifier name: (identifier) @name))) @definition.method

(function_definition declarator: (pointer_declarator declarator: (function_declarator declarator: (identifier) @name))) @definition.function
(function_definition declarator: (pointer_declarator declarator: (function_declarator declarator: (qualified_identifier name: (identifier) @name)))) @definition.method

(function_definition declarator: (pointer_declarator declarator: (pointer_declarator declarator: (function_declarator declarator: (identifier) @name)))) @definition.function
(function_definition declarator: (pointer_declarator declarator: (pointer_declarator declarator: (function_declarator declarator: (qualified_identifier name: (identifier) @name))))) @definition.method

(function_definition declarator: (reference_declarator (function_declarator declarator: (identifier) @name))) @definition.function
(function_definition declarator: (reference_declarator (function_declarator declarator: (qualified_identifier name: (identifier) @name)))) @definition.method

(function_definition declarator: (function_declarator declarator: (qualified_identifier name: (destructor_name) @name))) @definition.method

(declaration declarator: (function_declarator declarator: (identifier) @name)) @definition.function
(declaration declarator: (pointer_declarator declarator: (function_declarator declarator: (identifier) @name))) @definition.function

(field_declaration
  declarator: (function_declarator
    declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name)) @definition.method
(field_declaration
  declarator: (pointer_declarator
    declarator: (function_declarator
      declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name))) @definition.method
(field_declaration
  declarator: (reference_declarator
    (function_declarator
      declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name))) @definition.method

(field_declaration_list
  (function_definition
    declarator: (function_declarator
      declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name))) @definition.method
(field_declaration_list
  (function_definition
    declarator: (pointer_declarator
      declarator: (function_declarator
        declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name)))) @definition.method
(field_declaration_list
  (preproc_ifdef
    (function_definition
      declarator: (function_declarator
        declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name)) @definition.method))
(field_declaration_list
  (preproc_if
    (function_definition
      declarator: (function_declarator
        declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name)) @definition.method))
(field_declaration_list
  (preproc_ifdef
    (function_definition
      declarator: (reference_declarator
        (function_declarator
          declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name))) @definition.method))
(field_declaration_list
  (preproc_if
    (function_definition
      declarator: (reference_declarator
        (function_declarator
          declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name))) @definition.method))
(field_declaration_list
  (function_definition
    declarator: (reference_declarator
      (function_declarator
        declarator: [(field_identifier) (identifier) (operator_name) (destructor_name)] @name)))) @definition.method

(template_declaration (class_specifier name: (type_identifier) @name)) @definition.template
(template_declaration (function_definition declarator: (function_declarator declarator: (identifier) @name))) @definition.template

(preproc_include path: (_) @import.source) @import

(call_expression function: (identifier) @call.name) @call
(call_expression function: (field_expression field: (field_identifier) @call.name)) @call
(call_expression function: (qualified_identifier name: (identifier) @call.name)) @call
(call_expression function: (template_function name: (identifier) @call.name)) @call

(new_expression type: (type_identifier) @call.name) @call

(class_specifier name: (type_identifier) @heritage.class
  (base_class_clause (type_identifier) @heritage.extends)) @heritage
(class_specifier name: (type_identifier) @heritage.class
  (base_class_clause (access_specifier) (type_identifier) @heritage.extends)) @heritage
`

const CSharpQueries = `
(class_declaration name: (identifier) @name) @definition.class
(interface_declaration name: (identifier) @name) @definition.interface
(struct_declaration name: (identifier) @name) @definition.struct
(enum_declaration name: (identifier) @name) @definition.enum
(record_declaration name: (identifier) @name) @definition.record
(delegate_declaration name: (identifier) @name) @definition.delegate

(namespace_declaration name: (identifier) @name) @definition.namespace
(namespace_declaration name: (qualified_name) @name) @definition.namespace
(file_scoped_namespace_declaration name: (identifier) @name) @definition.namespace
(file_scoped_namespace_declaration name: (qualified_name) @name) @definition.namespace

(method_declaration name: (identifier) @name) @definition.method
(local_function_statement name: (identifier) @name) @definition.function
(constructor_declaration name: (identifier) @name) @definition.constructor
(property_declaration name: (identifier) @name) @definition.property

(class_declaration name: (identifier) @name (parameter_list) @definition.constructor)
(record_declaration name: (identifier) @name (parameter_list) @definition.constructor)

(using_directive (qualified_name) @import.source) @import
(using_directive (identifier) @import.source) @import

(invocation_expression function: (identifier) @call.name) @call
(invocation_expression function: (member_access_expression name: (identifier) @call.name)) @call

(object_creation_expression type: (identifier) @call.name) @call
(object_creation_expression type: (generic_name (identifier) @call.name)) @call

(variable_declaration type: (identifier) @call.name (variable_declarator (implicit_object_creation_expression) @call))

(class_declaration name: (identifier) @heritage.class
  (base_list (identifier) @heritage.extends)) @heritage
(class_declaration name: (identifier) @heritage.class
  (base_list (generic_name (identifier) @heritage.extends))) @heritage
`

const RustQueries = `
(function_item name: (identifier) @name) @definition.function
(trait_item body: (declaration_list
  (function_signature_item name: (identifier) @name) @definition.method))
(struct_item name: (type_identifier) @name) @definition.struct
(enum_item name: (type_identifier) @name) @definition.enum
(trait_item name: (type_identifier) @name) @definition.trait
(impl_item type: (type_identifier) @name !trait) @definition.impl
(impl_item type: (generic_type type: (type_identifier) @name) !trait) @definition.impl
(mod_item name: (identifier) @name) @definition.module

(type_item name: (type_identifier) @name) @definition.type
(const_item name: (identifier) @name) @definition.const
(static_item name: (identifier) @name) @definition.static
(macro_definition name: (identifier) @name) @definition.macro

(use_declaration argument: (_) @import.source) @import

(call_expression function: (identifier) @call.name) @call
(call_expression function: (field_expression field: (field_identifier) @call.name)) @call
(call_expression function: (scoped_identifier name: (identifier) @call.name)) @call
(call_expression function: (generic_function function: (identifier) @call.name)) @call

(struct_expression name: (type_identifier) @call.name) @call

(impl_item trait: (type_identifier) @heritage.trait type: (type_identifier) @heritage.class) @heritage
(impl_item trait: (generic_type type: (type_identifier) @heritage.trait) type: (type_identifier) @heritage.class) @heritage
(impl_item trait: (type_identifier) @heritage.trait type: (generic_type type: (type_identifier) @heritage.class)) @heritage
(impl_item trait: (generic_type type: (type_identifier) @heritage.trait) type: (generic_type type: (type_identifier) @heritage.class)) @heritage
`

const PHPQueries = `
(namespace_definition
  name: (namespace_name) @name) @definition.namespace

(class_declaration
  name: (name) @name) @definition.class

(interface_declaration
  name: (name) @name) @definition.interface

(trait_declaration
  name: (name) @name) @definition.trait

(enum_declaration
  name: (name) @name) @definition.enum

(function_definition
  name: (name) @name) @definition.function

(method_declaration
  name: (name) @name) @definition.method

(property_declaration
  (property_element
    (variable_name
      (name) @name))) @definition.property

(namespace_use_declaration
  (namespace_use_clause
    (qualified_name) @import.source)) @import

(function_call_expression
  function: (name) @call.name) @call

(member_call_expression
  name: (name) @call.name) @call

(nullsafe_member_call_expression
  name: (name) @call.name) @call

(scoped_call_expression
  name: (name) @call.name) @call

(object_creation_expression (name) @call.name) @call

(class_declaration
  name: (name) @heritage.class
  (base_clause
    [(name) (qualified_name)] @heritage.extends)) @heritage

(class_declaration
  name: (name) @heritage.class
  (class_interface_clause
    [(name) (qualified_name)] @heritage.implements)) @heritage.impl

(class_declaration
  name: (name) @heritage.class
  body: (declaration_list
    (use_declaration
      [(name) (qualified_name)] @heritage.trait))) @heritage
`

const KotlinQueries = `
(class_declaration
  "interface"
  (type_identifier) @name) @definition.interface

(class_declaration
  "class"
  (type_identifier) @name) @definition.class

(object_declaration
  (type_identifier) @name) @definition.class

(companion_object
  (type_identifier) @name) @definition.class

(function_declaration
  (simple_identifier) @name) @definition.function

(property_declaration
  (variable_declaration
    (simple_identifier) @name)) @definition.property

(enum_entry
  (simple_identifier) @name) @definition.enum

(type_alias
  (type_identifier) @name) @definition.type

(import_header
  (identifier) @import.source) @import

(call_expression
  (simple_identifier) @call.name) @call

(call_expression
  (navigation_expression
    (navigation_suffix
      (simple_identifier) @call.name))) @call

(constructor_invocation
  (user_type
    (type_identifier) @call.name)) @call

(infix_expression
  (_)
  (simple_identifier) @call.name) @call

(class_declaration
  (type_identifier) @heritage.class
  (delegation_specifier
    (user_type (type_identifier) @heritage.extends))) @heritage

(class_declaration
  (type_identifier) @heritage.class
  (delegation_specifier
    (constructor_invocation
      (user_type (type_identifier) @heritage.extends)))) @heritage
`

const SwiftQueries = `
(class_declaration "class" name: (type_identifier) @name) @definition.class

(class_declaration "struct" name: (type_identifier) @name) @definition.struct

(class_declaration "enum" name: (type_identifier) @name) @definition.enum

(class_declaration "extension" name: (user_type (type_identifier) @name)) @definition.class

(class_declaration "actor" name: (type_identifier) @name) @definition.class

(protocol_declaration name: (type_identifier) @name) @definition.interface

(typealias_declaration name: (type_identifier) @name) @definition.type

(function_declaration name: (simple_identifier) @name) @definition.function

(protocol_function_declaration name: (simple_identifier) @name) @definition.method

(init_declaration) @definition.constructor

(property_declaration (pattern (simple_identifier) @name)) @definition.property

(import_declaration (identifier (simple_identifier) @import.source)) @import

(call_expression (simple_identifier) @call.name) @call

(call_expression (navigation_expression (navigation_suffix (simple_identifier) @call.name))) @call

(class_declaration name: (type_identifier) @heritage.class
  (inheritance_specifier inherits_from: (user_type (type_identifier) @heritage.extends))) @heritage

(protocol_declaration name: (type_identifier) @heritage.class
  (inheritance_specifier inherits_from: (user_type (type_identifier) @heritage.extends))) @heritage

(class_declaration "extension" name: (user_type (type_identifier) @heritage.class)
  (inheritance_specifier inherits_from: (user_type (type_identifier) @heritage.extends))) @heritage
`

// ─────────────────────────────────────────────────────────────────────────────
// Import-only queries — focused on extracting import path, name, and exported_name.
// Used by import_processor.go ProcessImports.
// ─────────────────────────────────────────────────────────────────────────────

const TypeScriptImportQuery = `
(import_statement
  source: (string) @import.path) @import

(import_statement
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @import.exported_name
        alias: (identifier)? @import.name)))) @import.named

(export_statement
  source: (string) @import.path) @import
`

const JavaScriptImportQuery = `
(import_statement
  source: (string) @import.path) @import

(import_statement
  (import_clause
    (named_imports
      (import_specifier
        name: (identifier) @import.exported_name
        alias: (identifier)? @import.name)))) @import.named

(export_statement
  source: (string) @import.path) @import
`

const PythonImportQuery = `
(import_statement
  name: (dotted_name) @import.path) @import

(import_from_statement
  module_name: [(dotted_name) (relative_import)] @import.path
  name: (dotted_name
    (identifier) @import.exported_name @import.name)) @import.named

(import_from_statement
  module_name: [(dotted_name) (relative_import)] @import.path
  name: (aliased_import
    name: (dotted_name (identifier) @import.exported_name)
    alias: (identifier) @import.name)) @import.named

(import_from_statement
  module_name: [(dotted_name) (relative_import)] @import.path
  (wildcard_import)) @import
`

const GoImportQuery = `
(import_declaration
  (import_spec
    path: (interpreted_string_literal) @import.path)) @import

(import_declaration
  (import_spec_list
    (import_spec
      path: (interpreted_string_literal) @import.path))) @import
`

const JavaImportQuery = `
(import_declaration
  (_) @import.path) @import
`

const KotlinImportQuery = `
(import_header
  (identifier) @import.path) @import
`

const CSharpImportQuery = `
(using_directive
  (qualified_name) @import.path) @import

(using_directive
  (identifier) @import.path) @import
`

const PHPImportQuery = `
(namespace_use_declaration
  (namespace_use_clause
    (qualified_name) @import.path)) @import
`

const RustImportQuery = `
(use_declaration
  argument: (_) @import.path) @import
`

const SwiftImportQuery = `
(import_declaration
  (identifier
    (simple_identifier) @import.path)) @import
`

const CImportQuery = `
(preproc_include
  path: (_) @import.path) @import
`
