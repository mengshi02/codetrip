// Package typescript — TypeScript import/type-binding interpretation hooks.
// These functions translate raw tree-sitter captures into ParsedImport
// and ParsedTypeBinding records that the scope-resolution pipeline consumes.
//
// interpretTsImport handles 10 import kinds:
//
//	default, named, named-alias, namespace, reexport, reexport-alias,
//	reexport-wildcard, reexport-namespace, dynamic, side-effect
//
// interpretTsTypeBinding handles type-binding captures:
//
//	annotation, parameter-annotation, return-annotation, self,
//	assignment-inferred, constructor-inferred, receiver-propagated
//
// Ported from TS languages/typescript/interpret.ts.
package typescript

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// InterpretTsImport converts a TypeScript import capture into a ParsedImport.
// Handles 10 import kinds dispatched by @import.kind marker:
//   - default          → ParsedImportNamed with localName from alias
//   - named            → ParsedImportNamed
//   - named-alias      → ParsedImportAlias
//   - namespace        → ParsedImportNamespace
//   - reexport         → ParsedImportReexport
//   - reexport-alias   → ParsedImportReexport with alias
//   - reexport-wildcard → ParsedImportWildcard
//   - reexport-namespace → ParsedImportReexport with namespace
//   - dynamic          → ParsedImportDynamicResolved or DynamicUnresolved
//   - side-effect      → ParsedImportSideEffect
//
// Mirrors TS interpretTsImport(captures): ParsedImport | null.
// TODO: full implementation — currently returns zero-value ParsedImport.
func InterpretTsImport(captures shared.CaptureMatch) *shared.ParsedImport {
	// TODO: extract @import.kind, @import.name, @import.alias, @import.source,
	// dispatch on kind to produce appropriate ParsedImport.
	return nil
}

// InterpretTsTypeBinding converts a TypeScript type-binding capture into
// a ParsedTypeBinding. Handles annotations, parameter annotations, return
// annotations, self bindings, assignment-inferred types, constructor-
// inferred types, and receiver-propagated bindings.
//
// Mirrors TS interpretTsTypeBinding(captures): ParsedTypeBinding | null.
// TODO: full implementation — currently returns nil.
func InterpretTsTypeBinding(captures shared.CaptureMatch) *shared.ParsedTypeBinding {
	// TODO: extract @type-binding-name and @type-binding-type captures,
	// determine Source (annotation/parameter-annotation/return-annotation/
	// self/assignment-inferred/constructor-inferred/receiver-propagated),
	// fill BoundName and RawTypeName.
	return nil
}

// StripGenericsAndArraySuffix removes generic type arguments (<T>) and
// array suffixes ([]) from a TypeScript type name, producing the base
// type name used for arity/parameter-type matching.
//
// Examples:
//   "Array<number>" → "Array"
//   "Promise<string>" → "Promise"
//   "string[]" → "string"
//   "Map<string, number>" → "Map"
//
// Mirrors TS stripGenericsAndArraySuffix(raw): string.
// TODO: full implementation — currently does basic trimming.
func StripGenericsAndArraySuffix(raw string) string {
	// TODO: strip generic args (<...>), array suffix ([]),
	// handle nested generics properly.
	return raw
}