package rust

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// InterpretRustImport interprets a Rust use/import capture as an ImportEdge.
// Rust imports: `use crate::module::item`, `use super::item`, `use self::item`.
// TODO: full implementation — currently returns zero-value.
func InterpretRustImport(capture shared.CaptureMatch) shared.ImportEdge {
	// TODO: implement Rust import interpretation.
	return shared.ImportEdge{}
}

// InterpretRustTypeBinding interprets a Rust type annotation capture as a TypeRef.
// Rust type bindings: struct fields, function return types, let bindings.
// TODO: full implementation — currently returns zero-value.
func InterpretRustTypeBinding(capture shared.CaptureMatch) shared.TypeRef {
	// TODO: implement Rust type binding interpretation.
	return shared.TypeRef{}
}