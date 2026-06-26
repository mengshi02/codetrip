// Package typescript — TypeScript import decomposer.
// Splits a raw import/re-export/dynamic-import statement node into
// decomposed CaptureMatch records with @import.kind/source/name/alias
// markers so interpretTsImport can recover the ParsedImport shape
// without re-parsing raw text.
//
// Handles three node types:
//   - import_statement → splitImport (static imports, incl. side-effect)
//   - export_statement  → splitReexport (re-exports with source)
//   - call_expression   → splitDynamicImport (import() calls)
//
// Ported from TS languages/typescript/import-decomposer.ts.
package typescript

import "github.com/mengshi02/codetrip/internal/ingestion/shared"

// SplitImportStatement decomposes a raw import/re-export/dynamic-import
// SyntaxNode into CaptureMatch records with @import.kind/source/name/alias
// markers. Returns empty slice for unrecognized node types.
//
// Mirrors TS splitImportStatement(stmtNode: SyntaxNode): CaptureMatch[].
// TODO: full implementation — currently returns empty slice.
func SplitImportStatement(stmtNode interface{}) []shared.CaptureMatch {
	// TODO: dispatch by node type:
	//   - "import_statement" → splitImport
	//   - "export_statement" → splitReexport
	//   - "call_expression" → splitDynamicImport
	return nil
}