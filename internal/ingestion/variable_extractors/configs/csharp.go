// csharp.go — C# variable extraction config.
//
// C# does not have true top-level variables (pre-C# 9). In C# 9+ top-level
// statements, local_declaration_statement can appear at program scope.
// Class-scoped fields are handled by the field extractor.
//
// Ported from TS variable-extractors/configs/csharp.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	fieldconfigs "github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
)

// CSharpVariableConfig is the variable extraction configuration for C#.
var CSharpVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangCSharp,
	ConstNodeTypes:    []string{},
	StaticNodeTypes:   []string{},
	VariableNodeTypes: []string{"local_declaration_statement"},

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
		// local_declaration_statement → variable_declaration → variable_declarator → identifier
		var varDecl *gotreesitter.Node
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "variable_declaration" {
				varDecl = child
				break
			}
		}
		if varDecl == nil {
			return ""
		}
		var declarator *gotreesitter.Node
		for i := 0; i < int(varDecl.NamedChildCount()); i++ {
			child := varDecl.NamedChild(i)
			if child != nil && child.Type(lang) == "variable_declarator" {
				declarator = child
				break
			}
		}
		if declarator != nil {
			name := declarator.ChildByFieldName("name", lang)
			if name != nil && name.Type(lang) == "identifier" {
				return name.Text(source)
			}
		}
		return ""
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		var varDecl *gotreesitter.Node
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "variable_declaration" {
				varDecl = child
				break
			}
		}
		if varDecl == nil {
			return nil
		}
		typeNode := varDecl.ChildByFieldName("type", lang)
		if typeNode != nil {
			t := strings.TrimSpace(typeNode.Text(source))
			return &t
		}
		return nil
	},

	ExtractVisibility: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) core.VariableVisibility {
		if fieldconfigs.HasModifier(node, "modifiers", "public", lang) {
			return core.VisibilityPublic
		}
		if fieldconfigs.HasModifier(node, "modifiers", "private", lang) {
			return core.VisibilityPrivate
		}
		if fieldconfigs.HasModifier(node, "modifiers", "protected", lang) && fieldconfigs.HasModifier(node, "modifiers", "internal", lang) {
			return core.VisibilityProtectedInternal
		}
		if fieldconfigs.HasModifier(node, "modifiers", "private", lang) && fieldconfigs.HasModifier(node, "modifiers", "protected", lang) {
			return core.VisibilityPrivateProtected
		}
		if fieldconfigs.HasModifier(node, "modifiers", "protected", lang) {
			return core.VisibilityProtected
		}
		if fieldconfigs.HasModifier(node, "modifiers", "internal", lang) {
			return core.VisibilityInternal
		}
		return core.VisibilityPrivate
	},

	IsConst: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return fieldconfigs.HasModifier(node, "modifiers", "const", lang)
	},
	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return fieldconfigs.HasModifier(node, "modifiers", "static", lang)
	},
	IsMutable: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return !fieldconfigs.HasModifier(node, "modifiers", "const", lang) && !fieldconfigs.HasModifier(node, "modifiers", "readonly", lang)
	},
}