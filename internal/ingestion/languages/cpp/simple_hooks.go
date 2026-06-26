package cpp

// CppBindingScopeFor returns the scope ID for a C++ binding declaration.
// For return-type bindings, hoists to Module scope so propagateImportedReturnTypes
// can mirror them across files.
// Ported from GitNexus cpp/simple-hooks.ts.
func CppBindingScopeFor(hasReturnBinding bool) string {
	// If this is a return type binding, indicate it should be hoisted to module scope.
	// The caller (scope resolution orchestrator) will handle the actual hoisting.
	if hasReturnBinding {
		return "module" // signal to hoist to module scope
	}
	return "" // default auto-hoist
}

// CppImportOwningScope returns the owning scope for a C++ import.
// #include and using declarations are file-scoped in C++.
// Ported from GitNexus cpp/simple-hooks.ts.
func CppImportOwningScope() string {
	return "" // default
}

// CppReceiverBinding returns the receiver binding for a C++ function scope.
// C++ receiver resolution is handled through populateOwners + MRO chain,
// not through this hook.
// Ported from GitNexus cpp/simple-hooks.ts.
func CppReceiverBinding() string {
	return "" // no receiver through this hook
}