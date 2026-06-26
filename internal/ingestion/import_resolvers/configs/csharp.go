// Package configs — C# import resolution configuration.
//
// C# uses namespace-based strategy via .csproj configs, then standard fallback.
// Mirrors TS import-resolvers/configs/csharp.ts.
package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	importresolvers "github.com/mengshi02/codetrip/internal/ingestion/import_resolvers"
)

// CSharpImportConfig is the import resolution configuration for C#.
// Uses namespace-based strategy, then standard suffix fallback.
var CSharpImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangCSharp,
	Strategies: []importresolvers.ImportResolverStrategy{
		importresolvers.CSharpNamespaceStrategy,
		importresolvers.CreateStandardStrategy(core.LangCSharp),
	},
}