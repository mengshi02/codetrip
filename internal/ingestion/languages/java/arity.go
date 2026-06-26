// Package java — Java arity compatibility check.
// Java methods support overloading and varargs; this hook determines
// whether a callsite's arity matches a candidate definition.
// Ported from TS languages/java/arity.ts.
package java

import (
	"github.com/mengshi02/codetrip/internal/ingestion/scope_resolution"
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JavaArityCompatibility checks whether a callsite's arity matches a
// definition's parameter signature. Java varargs (type ending with "...")
// allow callsites with more args than the def's max.
//
// Mirrors TS javaArityCompatibility(def, callsite).
// TODO: full implementation — currently returns ArityUnknown.
func JavaArityCompatibility(def shared.SymbolDefinition, callsite shared.Callsite) scope_resolution.ArityVerdict {
	// TODO: implement Java arity compatibility logic:
	// - Check ParameterCount and RequiredParameterCount
	// - Handle varargs (last param type ending with "...")
	// - Compare callsite.Arity against min/max bounds
	return scope_resolution.ArityUnknown
}