package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// ---------------------------------------------------------------------------
// Java
// ---------------------------------------------------------------------------

// JavaClassConfig extracts class-like declarations from Java source code.
// Supports class, interface, enum, and record declarations.
// Package declaration provides the top-level scope qualifier.
var JavaClassConfig = core.ClassExtractionConfig{
	Language: core.LangJava,
	TypeDeclarationNodes: []string{
		"class_declaration",
		"interface_declaration",
		"enum_declaration",
		"record_declaration",
	},
	FileScopeNodeTypes: []string{"package_declaration"},
	AncestorScopeNodeTypes: []string{
		"class_declaration",
		"interface_declaration",
		"enum_declaration",
		"record_declaration",
	},
}

// Note: Kotlin is excluded — not one of the 9 core supported languages.