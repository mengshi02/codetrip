package configs

import (
	"strings"
	"unicode"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// GoFieldConfig extracts field declarations from Go struct types.
//
// Go struct fields live inside type_declaration > type_spec > struct_type >
// field_declaration_list > field_declaration.
//
// Visibility: uppercase first letter = exported (public), lowercase = unexported (package).
//
// Ported from TS field-extractors/configs/go.ts.
var GoFieldConfig = core.FieldExtractionConfig{
	Language:             core.LangGo,
	TypeDeclarationNodes: []string{"type_declaration", "struct_type"},
	FieldNodeTypes:       []string{"field_declaration"},
	BodyNodeTypes:        []string{"field_declaration_list"},
	DefaultVisibility:    core.VisibilityPackage,

	ExtractOwnerName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		if node.Type(lang) == "struct_type" {
			parent := node.Parent()
			if parent != nil && parent.Type(lang) == "type_spec" {
				nameNode := parent.ChildByFieldName("name", lang)
				if nameNode != nil {
					t := nameNode.Text(source)
					return &t
				}
			}
			return nil
		}
		// type_declaration: find type_spec child
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "type_spec" {
				nameNode := child.ChildByFieldName("name", lang)
				if nameNode != nil {
					t := nameNode.Text(source)
					return &t
				}
			}
		}
		return nil
	},

	FindBodyNodes: func(node *gotreesitter.Node, lang *gotreesitter.Language) []*gotreesitter.Node {
		if node.Type(lang) == "struct_type" {
			for i := 0; i < int(node.NamedChildCount()); i++ {
				child := node.NamedChild(i)
				if child != nil && child.Type(lang) == "field_declaration_list" {
					return []*gotreesitter.Node{child}
				}
			}
			return nil
		}
		// type_declaration > type_spec > struct_type > field_declaration_list
		var typeSpec *gotreesitter.Node
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "type_spec" {
				typeSpec = child
				break
			}
		}
		if typeSpec == nil {
			return nil
		}
		typeNode := typeSpec.ChildByFieldName("type", lang)
		if typeNode == nil {
			return nil
		}
		for i := 0; i < int(typeNode.NamedChildCount()); i++ {
			child := typeNode.NamedChild(i)
			if child != nil && child.Type(lang) == "field_declaration_list" {
				return []*gotreesitter.Node{child}
			}
		}
		return nil
	},

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// field_declaration > name:(field_identifier)
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

	ExtractNames: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
		var names []string
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "field_identifier" {
				names = append(names, child.Text(source))
			}
		}
		return names
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		typeNode := node.ChildByFieldName("type", lang)
		if typeNode != nil {
			if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
				return t
			}
			trimmed := strings.TrimSpace(typeNode.Text(source))
			return &trimmed
		}
		// fallback: second named child is usually the type
		if node.NamedChildCount() >= 2 {
			t := node.NamedChild(1)
			if t != nil {
				if name := typeextractors.ExtractSimpleTypeNameFromNode(t, source, lang, 0); name != nil {
					return name
				}
				trimmed := strings.TrimSpace(t.Text(source))
				return &trimmed
			}
		}
		return nil
	},

	ExtractVisibility: func(_ *gotreesitter.Node, _ *gotreesitter.Language) core.FieldVisibility {
		// Go visibility is name-based (uppercase = public, lowercase = package).
		// The generic extractor calls ExtractVisibilityForName when available,
		// which receives the field name directly. This is a default fallback.
		return core.VisibilityPackage
	},

	ExtractVisibilityForName: func(_ *gotreesitter.Node, name string, _ *gotreesitter.Language) core.FieldVisibility {
		if len(name) == 0 {
			return core.VisibilityPackage
		}
		firstChar := rune(name[0])
		if unicode.IsUpper(firstChar) {
			return core.VisibilityPublic
		}
		return core.VisibilityPackage
	},

	IsStatic: func(_ *gotreesitter.Node, _ *gotreesitter.Language) bool {
		return false // Go has no static fields
	},

	IsReadonly: func(_ *gotreesitter.Node, _ *gotreesitter.Language) bool {
		return false // Go fields are not readonly
	},
}