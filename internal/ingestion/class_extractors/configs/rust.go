package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// RustClassConfig extracts struct and enum items from Rust source code.
// Modules and nested types form the ancestor scope chain.
var RustClassConfig = core.ClassExtractionConfig{
	Language:               core.LangRust,
	TypeDeclarationNodes:   []string{"struct_item", "enum_item"},
	AncestorScopeNodeTypes: []string{"mod_item", "struct_item", "enum_item"},
}