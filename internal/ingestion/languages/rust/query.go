package rust

// RustScopeQuery is the tree-sitter query string for Rust scope extraction.
// Captures: scopes (module/function/struct/enum/trait/impl), definitions,
// imports (use declarations), type bindings, and receiver references.
// TODO: full implementation — currently placeholder.
const RustScopeQuery = `
; Rust scope extraction query
; TODO: port full query from TS RUST_QUERIES

(function_item
  name: (identifier) @definition.function)

(struct_item
  name: (type_identifier) @definition.class)

(enum_item
  name: (type_identifier) @definition.class)

(trait_item
  name: (type_identifier) @definition.interface)

(impl_item
  type: (type_identifier) @reference.class)

(use_declaration
  argument: (scoped_identifier) @import)

(mod_item
  name: (identifier) @scope.module)
`

// GetRustScopeQuery returns the Rust tree-sitter scope query.
func GetRustScopeQuery() string { return RustScopeQuery }