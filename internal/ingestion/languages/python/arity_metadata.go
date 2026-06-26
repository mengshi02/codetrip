// Package python — Python arity metadata extraction.
// Extracts parameter count, required count, and type list from a
// function_definition tree-sitter node for use by pythonArityCompatibility.
//
// Mirrors TS languages/python/arity-metadata.ts (computePythonArityMetadata).
package python

// PythonArityMetadata holds the extracted arity metadata for a Python function.
// Ported from TS PythonArityMetadata interface.
type PythonArityMetadata struct {
	ParameterCount        *int     // nil when variadic (*args / **kwargs present)
	RequiredParameterCount *int    // nil when variadic; total - optionalCount otherwise
	ParameterTypes        []string // populated with real type text; nil when empty
}

// ComputePythonArityMetadata extracts arity metadata from a function_definition
// tree-sitter node.
//
// Python specifics:
//   - self / cls are stripped (consumed by extractPythonParameters).
//   - Defaulted params contribute to optionalCount, flipping
//     requiredParameterCount = total − optionalCount.
//   - Variadic (*args / **kwargs) collapses parameterCount to nil,
//     which pythonArityCompatibility then treats as 'unknown'.
//
// Mirrors TS computePythonArityMetadata(fnNode).
// TODO: wire to pythonMethodConfig.extractParameters when tree-sitter is integrated.
func ComputePythonArityMetadata(fnNode interface{}) PythonArityMetadata {
	// TODO: extract parameters from fnNode using tree-sitter Python parser.
	// This requires pythonMethodConfig.extractParameters to be wired.
	//
	// Skeleton logic (to be filled when tree-sitter integration is complete):
	//   params := pythonMethodConfig.extractParameters(fnNode)
	//   hasVariadic := false; optionalCount := 0; types := []
	//   for p in params:
	//     if p.isVariadic: hasVariadic = true
	//     elif p.isOptional: optionalCount++
	//     if p.type != nil: types.append(p.type)
	//   total := len(params)
	//   parameterCount = hasVariadic ? nil : total
	//   requiredParameterCount = hasVariadic ? nil : total - optionalCount
	//   parameterTypes = len(types) > 0 ? types : nil

	return PythonArityMetadata{
		ParameterCount:         nil,
		RequiredParameterCount: nil,
		ParameterTypes:         nil,
	}
}