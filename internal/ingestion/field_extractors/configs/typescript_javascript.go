package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// tsVisibilityKeywords contains TypeScript visibility modifier keywords.
var tsVisibilityKeywords = map[core.FieldVisibility]bool{
	core.VisibilityPublic:    true,
	core.VisibilityPrivate:   true,
	core.VisibilityProtected: true,
}

// jsVisibilityKeywords contains JavaScript visibility modifier keywords (same set as TS).
var jsVisibilityKeywords = map[core.FieldVisibility]bool{
	core.VisibilityPublic:    true,
	core.VisibilityPrivate:   true,
	core.VisibilityProtected: true,
}

// TypeScriptFieldConfig extracts field declarations from TypeScript class/interface/abstract class bodies.
// Shared between TypeScript and JavaScript (JS has no type annotations).
// Visibility: accessibility_modifier (public/private/protected), default = public.
// Ported from TS field-extractors/configs/typescript-javascript.ts.
var TypeScriptFieldConfig = core.FieldExtractionConfig{
	Language:             core.LangTypeScript,
	TypeDeclarationNodes: []string{"class_declaration", "abstract_class_declaration", "interface_declaration"},
	FieldNodeTypes:       []string{"public_field_definition", "property_signature", "field_definition"},
	BodyNodeTypes:        []string{"class_body", "interface_body", "object_type"},
	DefaultVisibility:    core.VisibilityPublic,

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil {
			nameNode = node.ChildByFieldName("property", lang)
		}
		if nameNode != nil {
			t := nameNode.Text(source)
			return &t
		}
		return nil
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// tree-sitter TS uses a named 'type' field for type_annotation
		typeField := node.ChildByFieldName("type", lang)
		if typeField != nil {
			if typeField.Type(lang) == "type_annotation" {
				inner := typeField.NamedChild(0)
				if inner != nil {
					trimmed := strings.TrimSpace(inner.Text(source))
					return &trimmed
				}
			}
			trimmed := strings.TrimSpace(typeField.Text(source))
			return &trimmed
		}
		return TypeFromAnnotation(node, source, lang)
	},

	ExtractVisibility: func(node *gotreesitter.Node, lang *gotreesitter.Language) core.FieldVisibility {
		// TypeScript accessibility_modifier
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "accessibility_modifier" {
				text := strings.TrimSpace(child.Type(lang))
				v := core.FieldVisibility(text)
				if tsVisibilityKeywords[v] {
					return v
				}
			}
		}
		return FindVisibility(node, tsVisibilityKeywords, core.VisibilityPublic, "modifiers", lang)
	},

	IsStatic: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "static", lang)
	},

	IsReadonly: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "readonly", lang)
	},
}

// JavaScriptFieldConfig extracts field declarations from JavaScript class bodies.
// JavaScript has no type annotations or accessibility modifiers.
// Visibility: always public (no access control in JS).
var JavaScriptFieldConfig = core.FieldExtractionConfig{
	Language:             core.LangJavaScript,
	TypeDeclarationNodes: []string{"class_declaration", "abstract_class_declaration", "interface_declaration"},
	FieldNodeTypes:       []string{"public_field_definition", "property_signature", "field_definition"},
	BodyNodeTypes:        []string{"class_body", "interface_body", "object_type"},
	DefaultVisibility:    core.VisibilityPublic,

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil {
			nameNode = node.ChildByFieldName("property", lang)
		}
		if nameNode != nil {
			t := nameNode.Text(source)
			return &t
		}
		return nil
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		typeField := node.ChildByFieldName("type", lang)
		if typeField != nil {
			if typeField.Type(lang) == "type_annotation" {
				inner := typeField.NamedChild(0)
				if inner != nil {
					trimmed := strings.TrimSpace(inner.Text(source))
					return &trimmed
				}
			}
			trimmed := strings.TrimSpace(typeField.Text(source))
			return &trimmed
		}
		return TypeFromAnnotation(node, source, lang)
	},

	ExtractVisibility: func(node *gotreesitter.Node, lang *gotreesitter.Language) core.FieldVisibility {
		// JavaScript has no accessibility modifiers; always public
		return FindVisibility(node, jsVisibilityKeywords, core.VisibilityPublic, "modifiers", lang)
	},

	IsStatic: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "static", lang)
	},

	IsReadonly: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "readonly", lang)
	},
}