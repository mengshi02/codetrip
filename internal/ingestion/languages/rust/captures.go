package rust

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// EmitRustScopeCaptures produces the raw scope-capture stream for a Rust source file.
// Rust captures: use/import statements, impl blocks (impl T for S), trait definitions,
// struct/enum definitions, function/method signatures, lifetime annotations.
// TODO: full implementation — currently returns nil.
func EmitRustScopeCaptures(source []byte, filePath string) []shared.CaptureMatch {
	// TODO: implement Rust scope capture emission.
	// Key Rust capture types:
	//   - use_item → import capture
	//   - impl_item → trait-impl capture (synthesizes @reference.inherits sites)
	//   - struct_item / enum_item → class-like captures
	//   - function_item / method_item → method captures with self receiver
	//   - mod_item → module scope capture
	return nil
}