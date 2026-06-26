package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// javaVisKeywords contains Java visibility modifier keywords.
var javaVisKeywords = map[core.FieldVisibility]bool{
	core.VisibilityPublic:    true,
	core.VisibilityPrivate:   true,
	core.VisibilityProtected: true,
}

// JavaFieldConfig extracts field declarations from Java class/interface/enum/record bodies.
// Visibility: modifiers container (public/private/protected), default = package (package-private).
// Static: keyword "static" or modifier container.
// Readonly: keyword "final" or modifier container.
// Ported from TS field-extractors/configs/jvm.ts (Java only; Kotlin not in 9 core languages).
var JavaFieldConfig = core.FieldExtractionConfig{
	Language:             core.LangJava,
	TypeDeclarationNodes: []string{"class_declaration", "interface_declaration", "enum_declaration", "record_declaration"},
	FieldNodeTypes:       []string{"field_declaration"},
	BodyNodeTypes:        []string{"class_body", "interface_body", "enum_body"},
	DefaultVisibility:    core.VisibilityPackage,

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// field_declaration > declarator:(variable_declarator name:(identifier))
		declarator := node.ChildByFieldName("declarator", lang)
		if declarator != nil {
			nameNode := declarator.ChildByFieldName("name", lang)
			if nameNode != nil {
				t := nameNode.Text(source)
				return &t
			}
		}
		// fallback: walk children for variable_declarator
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "variable_declarator" {
				nameNode := child.ChildByFieldName("name", lang)
				if nameNode != nil {
					t := nameNode.Text(source)
					return &t
				}
			}
		}
		return nil
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// field_declaration > type:(type_identifier|generic_type|...)
		t := TypeFromField(node, "type", source, lang)
		if t != nil {
			return t
		}
		// fallback: first named child that looks like a type (skip modifiers)
		first := node.NamedChild(0)
		if first != nil && first.Type(lang) != "modifiers" {
			if typeName := typeextractors.ExtractSimpleTypeNameFromNode(first, source, lang, 0); typeName != nil {
				return typeName
			}
			trimmed := strings.TrimSpace(first.Text(source))
			return &trimmed
		}
		return nil
	},

	ExtractVisibility: func(node *gotreesitter.Node, lang *gotreesitter.Language) core.FieldVisibility {
		return FindVisibility(node, javaVisKeywords, core.VisibilityPackage, "modifiers", lang)
	},

	IsStatic: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "static", lang) || HasModifier(node, "modifiers", "static", lang)
	},

	IsReadonly: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "final", lang) || HasModifier(node, "modifiers", "final", lang)
	},
}