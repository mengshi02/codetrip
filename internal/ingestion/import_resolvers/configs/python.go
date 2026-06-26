// Package configs — Python import resolution configuration.
//
// Python uses PEP 328 relative + proximity-based strategy, then standard fallback.
// Mirrors TS import-resolvers/configs/python.ts.
package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	importresolvers "github.com/mengshi02/codetrip/internal/ingestion/import_resolvers"
)

// PythonImportConfig is the import resolution configuration for Python.
// Uses PEP 328 relative + proximity strategy, then standard suffix fallback.
var PythonImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangPython,
	Strategies: []importresolvers.ImportResolverStrategy{
		importresolvers.PythonImportStrategy,
		importresolvers.CreateStandardStrategy(core.LangPython),
	},
}