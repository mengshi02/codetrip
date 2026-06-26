// Package configs — Rust import resolution configuration.
//
// Rust uses module strategy (grouped imports, crate/super/self paths), then standard fallback.
// Mirrors TS import-resolvers/configs/rust.ts.
package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	importresolvers "github.com/mengshi02/codetrip/internal/ingestion/import_resolvers"
)

// RustImportConfig is the import resolution configuration for Rust.
// Uses Rust module strategy, then standard suffix fallback.
var RustImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangRust,
	Strategies: []importresolvers.ImportResolverStrategy{
		importresolvers.RustModuleStrategy,
		importresolvers.CreateStandardStrategy(core.LangRust),
	},
}