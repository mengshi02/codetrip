// Package csharp — C# arity metadata: declaration-level arity computation.
// C# methods may have optional parameters (with default values) and
// params arrays (variadic). This computes ParameterCount and
// RequiredParameterCount from captured parameter nodes.
// Ported from TS languages/csharp/arity-metadata.ts.
package csharp

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// CsharpArityMetadata is the struct for computing declaration-level arity
// information from C# captures. C# methods may have optional parameters
// and params (variadic) arrays.
type CsharpArityMetadata struct {
	ParameterCount        int
	RequiredParameterCount int
	HasParamsArray        bool // true if the last parameter is a "params" array
}

// ComputeCsharpArityMetadata inspects the captures of a C# method/constructor
// declaration and computes the total and required parameter counts.
// Optional parameters (with default values) contribute to total but not required.
// Params arrays (variadic) contribute 1 to total and 0 to required.
//
// Mirrors TS computeCsharpArityMetadata(captures).
// TODO: full implementation — currently returns zero-value struct.
func ComputeCsharpArityMetadata(captures shared.CaptureMatch) CsharpArityMetadata {
	// TODO: walk @parameter captures, count total vs required,
	// detect optional parameters (with default values) and params arrays.
	return CsharpArityMetadata{}
}