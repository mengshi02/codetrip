package rust

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// SynthesizeRustReceiverBinding creates a receiver binding for a Rust method.
// Rust methods have explicit self parameters: &self, &mut self, self.
// The receiver binding maps the self parameter to the enclosing struct/enum type.
// TODO: full implementation — currently returns zero-value.
func SynthesizeRustReceiverBinding(
	methodDef shared.SymbolDefinition,
	scopeID shared.ScopeID,
) shared.BindingRef {
	// TODO: implement Rust receiver binding synthesis.
	// For `impl S { fn foo(&self) }` → bind self → S
	return shared.BindingRef{}
}