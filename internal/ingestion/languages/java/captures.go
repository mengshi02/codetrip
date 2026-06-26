// Package java — Java scope capture emission.
// Runs tree-sitter queries against Java source to extract scope boundaries,
// definitions, imports, and type bindings.
// Ported from TS languages/java/captures.ts.
package java

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// EmitJavaScopeCaptures runs the Java tree-sitter query against source and
// returns capture matches representing scopes, definitions, imports,
// and type bindings found in the file.
//
// Mirrors TS emitJavaScopeCaptures(source, filePath).
// TODO: full implementation — currently returns empty slice.
func EmitJavaScopeCaptures(source []byte, filePath string) []shared.CaptureMatch {
	// TODO: parse source with tree-sitter, run JavaScopeQuery,
	// group matches into CaptureMatch slices.
	return nil
}