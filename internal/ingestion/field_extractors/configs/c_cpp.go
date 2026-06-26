package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// CppFieldConfig extracts field declarations from C++ struct/class/union.
//
// C++ fields live inside struct_specifier/class_specifier/union_specifier >
// field_declaration_list > field_declaration.
//
// Visibility is determined by access_specifier (public:/private:/protected:)
// walking backwards from the field node through siblings.
// C++ class default is private, struct default is public.
//
// Ported from TS field-extractors/configs/c-cpp.ts.
var CppFieldConfig = core.FieldExtractionConfig{
	Language:             core.LangCpp,
	TypeDeclarationNodes: []string{"struct_specifier", "class_specifier", "union_specifier"},
	FieldNodeTypes:       []string{"field_declaration"},
	BodyNodeTypes:        []string{"field_declaration_list"},
	DefaultVisibility:    core.VisibilityPrivate,

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		return extractCFieldName(node, source, lang)
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		return extractCFieldType(node, source, lang)
	},

	ExtractVisibility: func(node *gotreesitter.Node, lang *gotreesitter.Language) core.FieldVisibility {
		access := cppAccessSpecifier(node, lang)
		if access != "" {
			return access
		}
		// struct default = public, class default = private
		parent := node.Parent()
		if parent != nil {
			grandparent := parent.Parent()
			if grandparent != nil && grandparent.Type(lang) == "struct_specifier" {
				return core.VisibilityPublic
			}
		}
		return core.VisibilityPrivate
	},

	IsStatic: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "static", lang)
	},

	IsReadonly: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "const", lang)
	},
}

// CFieldConfig extracts field declarations from C struct/union.
// C has no access control — all fields are public.
var CFieldConfig = core.FieldExtractionConfig{
	Language:             core.LangC,
	TypeDeclarationNodes: []string{"struct_specifier", "union_specifier"},
	FieldNodeTypes:       []string{"field_declaration"},
	BodyNodeTypes:        []string{"field_declaration_list"},
	DefaultVisibility:    core.VisibilityPublic,

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		return extractCFieldName(node, source, lang)
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		return extractCFieldType(node, source, lang)
	},

	ExtractVisibility: func(_ *gotreesitter.Node, _ *gotreesitter.Language) core.FieldVisibility {
		return core.VisibilityPublic // C has no access control
	},

	IsStatic: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "static", lang)
	},

	IsReadonly: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "const", lang)
	},
}

// cppAccessSpecifier walks backwards from the field node through named siblings
// to find a C++ access_specifier (public:/private:/protected:).
func cppAccessSpecifier(node *gotreesitter.Node, lang *gotreesitter.Language) core.FieldVisibility {
	sibling := PrevNamedSibling(node, lang)
	for sibling != nil {
		if sibling.Type(lang) == "access_specifier" {
			text := strings.TrimSpace(strings.ReplaceAll(sibling.Text(nil), ":", ""))
			switch core.FieldVisibility(text) {
			case core.VisibilityPublic, core.VisibilityPrivate, core.VisibilityProtected:
				return core.FieldVisibility(text)
			}
		}
		sibling = PrevNamedSibling(sibling, lang)
	}
	return ""
}

// extractCFieldName extracts the name from a C/C++ field_declaration node.
func extractCFieldName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	declarator := node.ChildByFieldName("declarator", lang)
	if declarator != nil {
		if declarator.Type(lang) == "field_identifier" {
			t := declarator.Text(source)
			return &t
		}
		// pointer_declarator: *fieldName
		for i := 0; i < int(declarator.NamedChildCount()); i++ {
			child := declarator.NamedChild(i)
			if child != nil && child.Type(lang) == "field_identifier" {
				t := child.Text(source)
				return &t
			}
		}
		t := declarator.Text(source)
		return &t
	}
	// fallback: find field_identifier child
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "field_identifier" {
			t := child.Text(source)
			return &t
		}
	}
	return nil
}

// extractCFieldType extracts the type from a C/C++ field_declaration node.
func extractCFieldType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode != nil {
		if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
			return t
		}
		trimmed := strings.TrimSpace(typeNode.Text(source))
		return &trimmed
	}
	// fallback: first named child that looks like a type
	first := typeextractors.FirstNamedChild(node, lang)
	if first != nil {
		ft := first.Type(lang)
		if ft == "type_identifier" || ft == "primitive_type" || ft == "sized_type_specifier" || ft == "template_type" {
			if t := typeextractors.ExtractSimpleTypeNameFromNode(first, source, lang, 0); t != nil {
				return t
			}
			trimmed := strings.TrimSpace(first.Text(source))
			return &trimmed
		}
	}
	return nil
}