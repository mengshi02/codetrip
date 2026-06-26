// Package csharp — C# scope capture emission.
// Runs tree-sitter queries against C# source to extract scope boundaries,
// definitions, imports, and type bindings.
// Ported from TS languages/csharp/captures.ts.
package csharp

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// EmitCsharpScopeCaptures runs the C# tree-sitter query against source and
// returns capture matches representing scopes, definitions, imports,
// and type bindings found in the file.
//
// Mirrors TS emitCsharpScopeCaptures(source, filePath).
// TODO: full implementation — currently returns empty slice.
func EmitCsharpScopeCaptures(source []byte, filePath string) []shared.CaptureMatch {
	// TODO: parse source with tree-sitter, run CsharpScopeQuery,
	// group matches into CaptureMatch slices.
	return nil
}