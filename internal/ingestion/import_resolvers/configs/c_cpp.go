// Package configs — C/C++ import resolution configurations.
//
// Both C and C++ use the standard strategy only (no language-specific strategies).
// Mirrors TS import-resolvers/configs/c-cpp.ts.
package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
	importresolvers "github.com/mengshi02/codetrip/internal/ingestion/import_resolvers"
)

// CImportConfig is the import resolution configuration for C.
// C uses the standard strategy only — relative paths and suffix matching.
var CImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangC,
	Strategies: []importresolvers.ImportResolverStrategy{
		importresolvers.CreateStandardStrategy(core.LangC),
	},
}

// CppImportConfig is the import resolution configuration for C++.
// C++ uses the standard strategy only — relative paths and suffix matching.
var CppImportConfig = importresolvers.ImportResolutionConfig{
	Language: core.LangCpp,
	Strategies: []importresolvers.ImportResolverStrategy{
		importresolvers.CreateStandardStrategy(core.LangCpp),
	},
}