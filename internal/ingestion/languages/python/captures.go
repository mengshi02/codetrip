// Package python — Python scope capture emission.
// Drives the scope query against tree-sitter-python and groups raw matches
// into CaptureMatch[] for the central extractor, then layers two synthesized
// streams on top:
//  1. Per-name import statements (see import-decomposer.ts)
//  2. Receiver type bindings (see receiver-binding.ts)
//
// Ported from TS languages/python/captures.ts (emitPythonScopeCaptures).
package python

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// EmitPythonScopeCaptures runs the Python tree-sitter scope query on sourceText
// and returns grouped CaptureMatch[] for the central scope extractor.
//
// The captures feed into:
//   - interpretPythonImport for import interpretation
//   - interpretPythonTypeBinding for type binding interpretation
//   - synthesizeReceiverTypeBinding for self/cls markers
//   - synthesizeDependsReferences for FastAPI Depends() references
//
// Mirrors TS emitPythonScopeCaptures(sourceText, filePath, cachedTree).
// TODO: full implementation — requires tree-sitter Python parser integration.
func EmitPythonScopeCaptures(sourceText string, filePath string, cachedTree interface{}) []shared.CaptureMatch {
	// TODO: implement when tree-sitter Python integration is ready.
	// Steps:
	//   1. Parse sourceText using getPythonParser()
	//   2. Run getPythonScopeQuery() against the tree
	//   3. Group matches into CaptureMatch[]
	//   4. Layer splitImportStatement() for per-name imports
	//   5. Layer synthesizeReceiverTypeBinding() for self/cls markers
	//   6. Layer synthesizeDependsReferences() for Depends() references
	return nil
}