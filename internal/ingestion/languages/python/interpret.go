// Package python — Capture-match → semantic-shape interpreters for Python.
// Two pure functions, both consumed by the central scope extractor:
//   - InterpretPythonImport     → ParsedImport
//   - InterpretPythonTypeBinding → TypeRef
//
// The matches arrive pre-decomposed by emitPythonScopeCaptures
// (one imported name per match; synthesized self/cls markers already attached)
// so these functions are straight-line tag readers.
//
// Ported from TS languages/python/interpret.ts.
package python

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// InterpretPythonImport interprets a CaptureMatch into a ParsedImport.
//
// Markers attached by splitImportStatement (import-decomposer.ts):
//   - @import.kind:  'plain' | 'aliased' | 'from' | 'from-alias' | 'wildcard' | 'dynamic'
//   - @import.name:  the imported symbol name (or module name for plain imports)
//   - @import.alias: the local alias name (for 'as' forms)
//   - @import.source: the module path (always present except for dynamic)
//
// Mirrors TS interpretPythonImport(captures).
// TODO: full implementation — requires Capture struct field access.
func InterpretPythonImport(captures shared.CaptureMatch) *shared.ParsedImport {
	// TODO: implement when CaptureMatch processing is ready.
	// Steps (from TS):
	//   1. Read @import.kind capture to determine import variant
	//   2. Based on kind, construct the appropriate ParsedImport:
	//      - plain: kind=namespace, localName=source first segment
	//      - aliased: kind=namespace, localName=alias
	//      - from: kind=named, localName=name, targetRaw=source
	//      - from-alias: kind=alias, localName=alias, importedName=name
	//      - wildcard: kind=wildcard, targetRaw=source
	//      - dynamic: kind=dynamic-unresolved
	//   3. Return nil if @import.kind is missing or unknown
	return nil
}

// InterpretPythonTypeBinding interprets a CaptureMatch into a TypeRef.
//
// Synthesized self/cls captures carry @type-binding.name and
// @type-binding.type directly — same shape as parameter annotations.
//
// Source determination order:
//   - @type-binding.self → source=self
//   - @type-binding.cls → source=self (cls is a self-like receiver)
//   - @type-binding.constructor → source=constructor-inferred
//   - @type-binding.annotation → source=annotation
//   - @type-binding.alias → source=assignment-inferred
//   - @type-binding.return → source=return-annotation
//   - default → source=parameter-annotation
//
// Mirrors TS interpretPythonTypeBinding(captures).
// TODO: full implementation — requires CaptureMatch processing + stripNullable/stripGeneric/stripForwardRefQuotes.
func InterpretPythonTypeBinding(captures shared.CaptureMatch) *shared.TypeRef {
	// TODO: implement when CaptureMatch processing is ready.
	// Steps (from TS):
	//   1. Read @type-binding.name and @type-binding.type captures
	//   2. Strip forward-ref quotes, nullable unions, single-arg generics
	//   3. Determine source based on presence of marker captures
	//   4. Construct TypeRef with rawTypeName and source
	return nil
}