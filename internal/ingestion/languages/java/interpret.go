// Package java — Java import/type-binding interpretation hooks.
// These functions translate raw tree-sitter captures into ParsedImport
// and ParsedTypeBinding records that the scope-resolution pipeline consumes.
// Ported from TS languages/java/interpret.ts.
package java

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// InterpretJavaImport converts a Java import capture into a ParsedImport.
// Handles named imports (import com.foo.Bar), wildcard imports (import com.foo.*),
// and static imports (import static com.foo.Bar.method / import static com.foo.Bar.*).
//
// Mirrors TS interpretJavaImport(captures).
// TODO: full implementation — currently returns zero-value ParsedImport.
func InterpretJavaImport(captures shared.CaptureMatch) shared.ParsedImport {
	// TODO: extract @import-path, @import-name captures,
	// determine Kind (named / wildcard / static-named / static-wildcard),
	// fill ImportPath and ImportedName fields.
	return shared.ParsedImport{}
}

// InterpretJavaTypeBinding converts a Java type-binding capture into a
// ParsedTypeBinding. Handles field type annotations, method return-type
// annotations, variable type annotations, and generic type parameters.
//
// Mirrors TS interpretJavaTypeBinding(captures).
// TODO: full implementation — currently returns zero-value ParsedTypeBinding.
func InterpretJavaTypeBinding(captures shared.CaptureMatch) shared.ParsedTypeBinding {
	// TODO: extract @type-binding-name and @type-binding-type captures,
	// determine Source (annotation / return-annotation / field-annotation),
	// fill BoundName and RawTypeName fields.
	return shared.ParsedTypeBinding{}
}