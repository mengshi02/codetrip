// Package java — Java arity metadata: declaration-level arity computation.
// Ported from TS languages/java/arity-metadata.ts.
package java

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// JavaArityMetadata holds computed arity information for a Java method/constructor
// declaration, including total parameter count and required (non-varargs) count.
type JavaArityMetadata struct {
	ParameterCount        int
	RequiredParameterCount int
	IsVarargs             bool
}

// ComputeJavaArityMetadata inspects the captures of a Java method/constructor
// declaration and computes the total and required parameter counts.
// Varargs parameters (Type...) contribute to ParameterCount but not
// RequiredParameterCount; they also set IsVarargs = true.
//
// Mirrors TS computeJavaArityMetadata(captures).
// TODO: full implementation — currently returns zero-value struct.
func ComputeJavaArityMetadata(captures shared.CaptureMatch) JavaArityMetadata {
	// TODO: walk @parameter captures, count total vs required,
	// detect varargs parameters via "..." suffix on last parameter type.
	return JavaArityMetadata{}
}