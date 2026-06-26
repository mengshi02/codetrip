package rust

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// RustArityMetadata holds extracted parameter metadata for a Rust function/method.
type RustArityMetadata struct {
	MinArgs   int    // minimum required arguments (excluding self)
	MaxArgs   *int   // nil = no upper bound (variadic)
	HasSelf   bool   // first param is self/Self
	IsVariadic bool  // last param is .. or variadic
	SelfKind  string // "", "&self", "&mut self", "self"
}

// ComputeRustArityMetadata extracts arity metadata from a Rust symbol definition.
// TODO: full implementation — currently returns zero-value metadata.
func ComputeRustArityMetadata(def shared.SymbolDefinition) RustArityMetadata {
	// TODO: parse Rust function signatures to extract arity metadata.
	// Rust supports: self params, generic params, no default values.
	return RustArityMetadata{}
}