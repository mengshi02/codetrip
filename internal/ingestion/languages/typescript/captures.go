// Package typescript — TypeScript scope capture emission.
// Runs tree-sitter queries against TypeScript/TSX source to extract scope
// boundaries, definitions, imports, type bindings, and references.
// Layers synthesized streams on top of raw captures:
//
//  1. Import decomposition — import_statement/re-export decomposed
//     with @import.kind/source/name/alias/typeOnly markers.
//  2. Dynamic imports — import('./m') re-emitted as decomposed
//     @import.statement with @import.kind=dynamic.
//  3. Function-decl arity metadata — @declaration.parameter-count/
//     required-parameter-count/parameter-types synthesized.
//  4. Callsite arity metadata — @reference.arity/parameter-types.
//  5. Receiver-binding synthesis — this type anchors on instance
//     methods, with arrow-function lexical-this walk-up.
//  6. Array callback filtering — HOC-wrapped arrow functions
//     named by const, but array-method callbacks suppressed.
//  7. Inheritance reference synthesis — extends/implements clauses.
//  8. Destructuring binding synthesis — object/array patterns.
//
// Ported from TS languages/typescript/captures.ts.
package typescript

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// EmitTsScopeCaptures runs the TypeScript tree-sitter query against source
// and returns capture matches representing scopes, definitions, imports,
// type bindings, and references found in the file.
//
// The function layers synthesized streams on top of raw tree-sitter matches:
// import decomposition, dynamic import emission, arity metadata synthesis,
// receiver binding synthesis, array callback filtering, inheritance
// references, and destructuring bindings.
//
// Mirrors TS emitTsScopeCaptures(sourceText, filePath, cachedTree).
// TODO: full implementation — currently returns empty slice.
func EmitTsScopeCaptures(source []byte, filePath string, cachedTree interface{}) []shared.CaptureMatch {
	// TODO: parse source with tree-sitter (grammar selected by .tsx/.ts extension),
	// run TypeScriptScopeQuery, iterate raw matches,
	// layer synthesized streams (imports, arity, receiver, array-callback, etc.)
	return nil
}