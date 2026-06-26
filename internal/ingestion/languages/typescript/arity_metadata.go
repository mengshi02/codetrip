// Package typescript — TypeScript arity metadata: declaration-level
// arity computation from function/method captures.
//
// Computes ParameterCount, RequiredParameterCount, and ParameterTypes
// from the captured parameter list. TypeScript-specific semantics:
//   - Rest parameters (...args) make parameterCount undefined (max unknown)
//   - Optional parameters (p?: T) contribute to optionalCount
//   - Defaulted parameters (p: T = expr) also contribute to optionalCount
//   - A 'params' marker is pushed onto ParameterTypes for rest params
//   - Generics and array suffixes are stripped from parameter type names
//
// Ported from TS languages/typescript/arity-metadata.ts.
package typescript

// TsArityMetadata holds the computed arity information for a function
// declaration.
type TsArityMetadata struct {
	ParameterCount        *int     // nil when rest parameter present (unknown max)
	RequiredParameterCount *int    // excludes optional/defaulted params
	ParameterTypes        []string // includes 'params' marker for rest
}

// ComputeTsArityMetadata inspects the captures of a TypeScript function/method
// declaration and computes arity metadata (parameter count, required count,
// parameter type names). Rest parameters push a 'params' marker and make
// ParameterCount nil.
//
// Mirrors TS computeTsArityMetadata(fnNode): TsArityMetadata.
// TODO: full implementation — currently returns zero-value metadata.
func ComputeTsArityMetadata(fnNode interface{}) TsArityMetadata {
	// TODO: extract parameters from fnNode, count total/required/optional,
	// detect rest parameters, compute ParameterTypes with 'params' marker.
	return TsArityMetadata{}
}