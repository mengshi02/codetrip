// Package java — Java method receiver binding synthesis.
// Java methods don't have explicit receiver parameters like Go, but the
// implicit "this" reference creates a self-type binding in each method's
// Function scope that is used by PopulateJavaClassOwnedMembers to stamp
// ownerId on Method defs.
// Ported from TS languages/java/receiver-binding.ts.
package java

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// SynthesizeJavaReceiverBinding creates a self-type BindingRef for the
// implicit receiver ("this") of a Java method scope. This binding links
// the method's Function scope to the class type that owns it.
//
// Returns nil when the scope is not a Function scope or has no
// self type binding.
//
// Mirrors TS synthesizeJavaReceiverBinding(scope).
// TODO: full implementation — currently returns nil.
func SynthesizeJavaReceiverBinding(scope *shared.Scope) *shared.BindingRef {
	if scope.Kind != shared.ScopeKindFunction {
		return nil
	}
	// TODO: extract receiver type from scope.TypeBindings["this"],
	// create a BindingRef pointing to the class/interface definition.
	return nil
}

// PopulateJavaClassOwnedMembers stamps ownerId on method/field definitions
// within a Java class scope, using the self-type binding to identify the
// owning class.
//
// Mirrors TS populateClassOwnedMembers(parsed).
// TODO: full implementation — currently no-op.
func PopulateJavaClassOwnedMembers(parsed *shared.ParsedFile) {
	// TODO: iterate scopes, for each Class scope stamp ownerId on
	// nested Function (method) defs using the class's QualifiedName.
}