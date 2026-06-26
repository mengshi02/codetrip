// typescript_javascript.go — TypeScript and JavaScript variable extraction configs.
//
// Handles module-scoped const/let/var declarations:
//   - `export const X = ...` → public, const
//   - `const X = ...` → private, const
//   - `let x = ...` → private, mutable
//   - `var x = ...` → private, mutable
//
// tree-sitter node structure:
//   - lexical_declaration (const/let) → variable_declarator → identifier (name)
//   - variable_declaration (var) → variable_declarator → identifier (name)
//
// Ported from TS variable-extractors/configs/typescript-javascript.ts.
package configs

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	fieldconfigs "github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
)

// extractTSJSName extracts the variable name from a lexical_declaration or variable_declaration.
func extractTSJSName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "variable_declarator" {
			name := child.ChildByFieldName("name", lang)
			if name != nil && name.Type(lang) == "identifier" {
				return name.Text(source)
			}
		}
	}
	return ""
}

// extractTSJSType extracts the type from a type_annotation in a variable_declarator.
func extractTSJSType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "variable_declarator" {
			return fieldconfigs.TypeFromAnnotation(child, source, lang)
		}
	}
	return nil
}

// extractTSJSVisibility determines visibility from export keywords.
func extractTSJSVisibility(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) core.VariableVisibility {
	// Check parent for export_statement wrapper
	parent := node.Parent()
	if parent != nil && parent.Type(lang) == "export_statement" {
		return core.VisibilityPublic
	}
	// Check for 'export' keyword as direct child
	if fieldconfigs.HasKeyword(node, "export", lang) {
		return core.VisibilityPublic
	}
	return core.VisibilityPrivate
}

// TypeScriptVariableConfig is the variable extraction configuration for TypeScript.
var TypeScriptVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangTypeScript,
	ConstNodeTypes:    []string{"lexical_declaration"},
	StaticNodeTypes:   []string{},
	VariableNodeTypes: []string{"variable_declaration"},

	ExtractName:       extractTSJSName,
	ExtractType:       extractTSJSType,
	ExtractVisibility: extractTSJSVisibility,

	IsConst: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		// lexical_declaration with 'const' keyword
		if node.Type(lang) == "lexical_declaration" {
			return fieldconfigs.HasKeyword(node, "const", lang)
		}
		return false
	},

	IsStatic: func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool {
		// JS/TS module-level variables are not static in the class sense
		return false
	},

	IsMutable: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		// var or let declarations are mutable; const is not
		if node.Type(lang) == "variable_declaration" {
			return true
		}
		if node.Type(lang) == "lexical_declaration" {
			return fieldconfigs.HasKeyword(node, "let", lang)
		}
		return false
	},
}

// JavaScriptVariableConfig is the variable extraction configuration for JavaScript.
// Shares the same extraction logic as TypeScript.
var JavaScriptVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangJavaScript,
	ConstNodeTypes:    []string{"lexical_declaration"},
	StaticNodeTypes:   []string{},
	VariableNodeTypes: []string{"variable_declaration"},

	ExtractName:       extractTSJSName,
	ExtractType:       extractTSJSType,
	ExtractVisibility: extractTSJSVisibility,

	IsConst:           TypeScriptVariableConfig.IsConst,
	IsStatic:          TypeScriptVariableConfig.IsStatic,
	IsMutable:         TypeScriptVariableConfig.IsMutable,
}