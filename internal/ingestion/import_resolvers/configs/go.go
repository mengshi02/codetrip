// Package configs — Go import resolution configuration.
//
// Go uses goPackageStrategy (module path → package dir), then standard fallback.
// Mirrors TS import-resolvers/configs/go.ts.
package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	importresolvers "github.com/mengshi02/codetrip/internal/ingestion/import_resolvers"
)

// GoImportConfig is the import resolution configuration for Go.
// Uses Go package strategy, then standard suffix fallback.
var GoImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangGo,
	Strategies: []importresolvers.ImportResolverStrategy{
		importresolvers.GoPackageStrategy,
		importresolvers.CreateStandardStrategy(core.LangGo),
	},
}