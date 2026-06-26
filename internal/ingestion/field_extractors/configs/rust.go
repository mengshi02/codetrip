package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// RustFieldConfig extracts field declarations from Rust struct/enum items.
//
// Rust struct fields live inside struct_item/enum_item > field_declaration_list > field_declaration.
// Visibility: pub keyword = public, otherwise private (crate-private).
// All fields are immutable by default in Rust (mutability is on the binding).
//
// Ported from TS field-extractors/configs/rust.ts.
var RustFieldConfig = core.FieldExtractionConfig{
	Language:             core.LangRust,
	TypeDeclarationNodes: []string{"struct_item", "enum_item"},
	FieldNodeTypes:       []string{"field_declaration"},
	BodyNodeTypes:        []string{"field_declaration_list"},
	DefaultVisibility:    core.VisibilityPrivate,

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode != nil {
			t := nameNode.Text(source)
			return &t
		}
		// fallback: first field_identifier child
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "field_identifier" {
				t := child.Text(source)
				return &t
			}
		}
		return nil
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		typeNode := node.ChildByFieldName("type", lang)
		if typeNode != nil {
			if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
				return t
			}
			trimmed := typeNode.Text(source)
			t := strings.TrimSpace(trimmed)
			return &t
		}
		return nil
	},

	ExtractVisibility: func(node *gotreesitter.Node, lang *gotreesitter.Language) core.FieldVisibility {
		// Check for visibility_modifier named child (pub, pub(crate), pub(super))
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "visibility_modifier" {
				return core.VisibilityPublic
			}
		}
		if HasKeyword(node, "pub", lang) {
			return core.VisibilityPublic
		}
		return core.VisibilityPrivate
	},

	IsStatic: func(_ *gotreesitter.Node, _ *gotreesitter.Language) bool {
		return false // Rust struct fields are never static
	},

	IsReadonly: func(_ *gotreesitter.Node, _ *gotreesitter.Language) bool {
		return true // All Rust fields are immutable by default (mutability is per-binding)
	},
}