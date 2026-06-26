// Package configs — TypeScript/JavaScript import resolution configurations.
//
// TypeScript/JavaScript use the standard strategy only (with TS path alias + ESM support).
// Mirrors TS import-resolvers/configs/typescript-javascript.ts.
package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	importresolvers "github.com/mengshi02/codetrip/internal/ingestion/import_resolvers"
)

// TypeScriptImportConfig is the import resolution configuration for TypeScript.
// Uses standard strategy with TS path alias and ESM extension stripping.
var TypeScriptImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangTypeScript,
	Strategies: []importresolvers.ImportResolverStrategy{
		importresolvers.CreateStandardStrategy(core.LangTypeScript),
	},
}

// JavaScriptImportConfig is the import resolution configuration for JavaScript.
// Uses standard strategy with ESM extension stripping.
var JavaScriptImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangJavaScript,
	Strategies: []importresolvers.ImportResolverStrategy{
		importresolvers.CreateStandardStrategy(core.LangJavaScript),
	},
}