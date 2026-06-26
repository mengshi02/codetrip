package rust

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// RustBindingScopeFor returns the scope kind that a Rust binding should be placed in.
// Rust uses module-level scoping for most declarations.
// TODO: full implementation — currently returns empty string.
func RustBindingScopeFor(nodeType string) string {
	// TODO: implement Rust binding scope classification.
	switch nodeType {
	case "function_item", "struct_item", "enum_item", "trait_item":
		return "Module"
	case "impl_item":
		return "Impl"
	default:
		return ""
	}
}

// WalkToScope returns the nearest enclosing scope ID for a Rust AST node.
// TODO: full implementation — currently returns empty ScopeID.
func WalkToScope(nodeType string, scopes []shared.Scope) shared.ScopeID {
	// TODO: implement Rust scope walking.
	return ""
}

// RustImportOwningScope returns the scope that owns a Rust import/use declaration.
// Rust imports are owned by the enclosing module scope.
// TODO: full implementation — currently returns empty ScopeID.
func RustImportOwningScope(filePath string, scopeTree *shared.ScopeTree) shared.ScopeID {
	// TODO: implement Rust import owning scope lookup.
	return ""
}

// RustReceiverBinding returns the receiver binding for a Rust method call.
// Rust methods have explicit self/Self receivers.
// TODO: full implementation — currently returns zero-value.
func RustReceiverBinding(def shared.SymbolDefinition, scopeID shared.ScopeID) shared.BindingRef {
	return SynthesizeRustReceiverBinding(def, scopeID)
}