// Package python — Python import statement decomposition.
// Decompose a Python import_statement / import_from_statement into
// one CaptureMatch per imported name.
//
// Why split here? The LanguageProvider.interpretImport contract is one
// ParsedImport per call. Tree-sitter delivers "import a, b as c" and
// "from m import x, y, z" as a single match each, so without decomposition
// we'd lose names.
//
// Ported from TS languages/python/import-decomposer.ts (splitImportStatement).
package python

import (
	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// SplitImportStatement decomposes a Python import statement tree-sitter node
// into one CaptureMatch per imported name, with synthesized markers:
//   - @import.kind: 'plain' | 'aliased' | 'from' | 'from-alias' | 'wildcard' | 'dynamic'
//   - @import.name: the imported symbol name
//   - @import.alias: the local alias name (for 'as' forms)
//   - @import.source: the module path
//
// Mirrors TS splitImportStatement(stmtNode).
// TODO: full implementation — requires tree-sitter node traversal.
func SplitImportStatement(stmtNode interface{}) []shared.CaptureMatch {
	// TODO: implement when tree-sitter Python integration is ready.
	// Steps:
	//   1. Determine import kind from stmtNode type (import_statement vs import_from_statement)
	//   2. For plain/aliased: extract dotted_name pairs for each imported name
	//   3. For from/from-alias: extract module source + each imported name
	//   4. For wildcard: emit single match with @import.kind='wildcard'
	//   5. For dynamic: emit match with @import.kind='dynamic'
	return nil
}