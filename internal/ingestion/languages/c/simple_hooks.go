package c

// CBindingScopeFor returns the scope ID for a C binding declaration.
// C has no self/receiver bindings that need special scoping — always use default auto-hoist.
// Ported from GitNexus c/simple-hooks.ts.
func CBindingScopeFor() string {
	return "" // empty means default auto-hoist
}

// CImportOwningScope returns the owning scope for a C import.
// C has no namespace scoping for imports — always use default.
// Ported from GitNexus c/simple-hooks.ts.
func CImportOwningScope() string {
	return "" // empty means default
}

// CReceiverBinding returns the receiver binding for a C function scope.
// C has no methods or receivers — always returns nil.
// Ported from GitNexus c/simple-hooks.ts.
func CReceiverBinding() string {
	return "" // empty means no receiver
}