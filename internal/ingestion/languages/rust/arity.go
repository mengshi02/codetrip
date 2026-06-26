package rust

import (
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// RustArityCompatibility checks whether a callsite's argument count is compatible
// with a candidate definition's parameter signature.
// Note: argument order is (def, callsite) — the legacy convention.
// TODO: full implementation — currently returns ArityUnknown.
func RustArityCompatibility(def shared.SymbolDefinition, callsite shared.Callsite) scope_resolution.ArityVerdict {
	// TODO: implement Rust arity compatibility checking.
	// Rust supports: regular params, self params, default (no), rest/catch-all (no).
	return scope_resolution.ArityUnknown
}