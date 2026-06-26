package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// CSharpClassConfig extracts class-like declarations from C# source code.
// Supports class, interface, struct, enum, and record declarations.
// File-scoped namespace provides the top-level package qualifier.
var CSharpClassConfig = core.ClassExtractionConfig{
	Language: core.LangCSharp,
	TypeDeclarationNodes: []string{
		"class_declaration",
		"interface_declaration",
		"struct_declaration",
		"enum_declaration",
		"record_declaration",
	},
	FileScopeNodeTypes: []string{"file_scoped_namespace_declaration"},
	AncestorScopeNodeTypes: []string{
		"namespace_declaration",
		"class_declaration",
		"interface_declaration",
		"struct_declaration",
		"enum_declaration",
		"record_declaration",
	},
}