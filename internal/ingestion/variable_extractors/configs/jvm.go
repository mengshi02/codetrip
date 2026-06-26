// jvm.go — Java variable extraction config.
//
// Java: local_variable_declaration at file scope (rare; class-scoped fields
// are handled by the field extractor).
//
// Note: Kotlin is not one of the 9 core languages in this project.
// If Kotlin support is added in the future, its property_declaration config
// can be added here following the same pattern.
//
// Ported from TS variable-extractors/configs/jvm.ts.
package configs

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	fieldconfigs "github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// JavaVariableConfig is the variable extraction configuration for Java.
var JavaVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangJava,
	ConstNodeTypes:    []string{},
	StaticNodeTypes:   []string{},
	VariableNodeTypes: []string{"local_variable_declaration"},

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
		// local_variable_declaration → variable_declarator → identifier (name)
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
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		typeNode := node.ChildByFieldName("type", lang)
		if typeNode != nil {
			if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
				return t
			}
			trimmed := typeNode.Text(source)
			return &trimmed
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
		if fieldconfigs.HasModifier(node, "modifiers", "protected", lang) {
			return core.VisibilityProtected
		}
		return core.VisibilityPackage
	},

	IsConst: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return fieldconfigs.HasModifier(node, "modifiers", "final", lang)
	},

	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return fieldconfigs.HasModifier(node, "modifiers", "static", lang)
	},

	IsMutable: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return !fieldconfigs.HasModifier(node, "modifiers", "final", lang)
	},
}