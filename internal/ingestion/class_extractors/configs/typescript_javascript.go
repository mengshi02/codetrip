package configs

import (
	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// sharedClassDecls contains the common type declaration and ancestor scope
// node types for TypeScript and JavaScript.
var sharedClassDecls = struct {
	TypeDeclarationNodes   []string
	AncestorScopeNodeTypes []string
}{
	TypeDeclarationNodes: []string{
		"class_declaration",
		"abstract_class_declaration",
		"interface_declaration",
		"enum_declaration",
	},
	AncestorScopeNodeTypes: []string{
		"class_declaration",
		"abstract_class_declaration",
		"interface_declaration",
		"enum_declaration",
	},
}

// TypeScriptClassConfig extracts class-like declarations from TypeScript source code.
var TypeScriptClassConfig = core.ClassExtractionConfig{
	Language:               core.LangTypeScript,
	TypeDeclarationNodes:   sharedClassDecls.TypeDeclarationNodes,
	AncestorScopeNodeTypes: sharedClassDecls.AncestorScopeNodeTypes,
}

// JavaScriptClassConfig extracts class-like declarations from JavaScript source code.
var JavaScriptClassConfig = core.ClassExtractionConfig{
	Language:               core.LangJavaScript,
	TypeDeclarationNodes:   sharedClassDecls.TypeDeclarationNodes,
	AncestorScopeNodeTypes: sharedClassDecls.AncestorScopeNodeTypes,
}

// Note: Vue is excluded — not one of the 9 core supported languages.