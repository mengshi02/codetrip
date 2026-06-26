// Package javascript — emitJsScopeCaptures for JavaScript.
// Adapts the TypeScript capture emitter for the JavaScript grammar:
//  1. JS grammar — uses tree-sitter-javascript instead of tree-sitter-typescript.
//  2. CJS require() decomposition — synthesized as @import.* markers.
//  3. JSDoc type bindings — inferred from leading JSDoc comments.
//  4. Constructor field bindings — this.X = new Y() assignments in constructors.
//  5. Shared synthesis passes — destructuring, for-of, instanceof.
//  6. Inheritance references — EXTENDS edges from class heritage.
//
// Ported from TS languages/javascript/captures.ts.
package javascript

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// EmitJsScopeCaptures runs the JS scope query on source text and returns
// capture matches with synthesized CJS imports, JSDoc bindings, constructor
// field bindings, destructuring bindings, for-of map-tuple bindings,
// instanceof narrowings, and inheritance references.
//
// Parameters:
//   - source: the raw source text (string in TS, []byte in Go skeleton)
//   - filePath: file path used to select JSX vs JS grammar
//   - cachedTree: optional pre-parsed tree-sitter Tree (interface{} placeholder)
//
// Returns a slice of CaptureMatch maps.
// TODO: full implementation — 946 lines of TS source to port.
func EmitJsScopeCaptures(source []byte, filePath string, cachedTree interface{}) []shared.CaptureMatch {
	return nil
}