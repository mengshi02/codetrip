package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// PythonFieldConfig extracts field declarations from Python class bodies.
//
// Python class fields appear as:
//   - Annotated assignments: name: str = "default"
//   - Plain assignments in __init__: self.name = value
//
// For AST-level extraction we handle expression_statement containing
// assignment or type nodes inside a class body block.
// Visibility: __prefix = private, _prefix = protected, else = public.
//
// Ported from TS field-extractors/configs/python.ts.
var PythonFieldConfig = core.FieldExtractionConfig{
	Language:             core.LangPython,
	TypeDeclarationNodes: []string{"class_definition"},
	FieldNodeTypes:       []string{"expression_statement"},
	BodyNodeTypes:        []string{"block"},
	DefaultVisibility:    core.VisibilityPublic,

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// expression_statement wrapping an assignment or type
		inner := typeextractors.FirstNamedChild(node, lang)
		if inner == nil {
			return nil
		}

		// Annotated assignment: name: str = "default"
		// tree-sitter node: type (expression_statement > type > identifier)
		if inner.Type(lang) == "type" {
			ident := inner.ChildByFieldName("name", lang)
			if ident == nil {
				ident = typeextractors.FirstNamedChild(inner, lang)
			}
			if ident != nil && ident.Type(lang) == "identifier" {
				t := ident.Text(source)
				return &t
			}
		}

		// assignment: x = 5 (class variable)
		if inner.Type(lang) == "assignment" {
			left := inner.ChildByFieldName("left", lang)
			if left != nil && left.Type(lang) == "identifier" {
				t := left.Text(source)
				return &t
			}
		}

		return nil
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		inner := typeextractors.FirstNamedChild(node, lang)
		if inner == nil {
			return nil
		}

		// Annotated assignment: name: str = "default"
		if inner.Type(lang) == "type" {
			typeNode := inner.ChildByFieldName("type", lang)
			if typeNode == nil {
				typeNode = inner.NamedChild(1)
			}
			if typeNode != nil {
				if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
					return t
				}
				trimmed := strings.TrimSpace(typeNode.Text(source))
				return &trimmed
			}
		}

		// Assignment with annotation: address: Address
		if inner.Type(lang) == "assignment" {
			for i := 0; i < int(inner.ChildCount()); i++ {
				child := inner.Child(i)
				if child != nil && child.Type(lang) == "type" {
					typeId := typeextractors.FirstNamedChild(child, lang)
					if typeId != nil {
						if t := typeextractors.ExtractSimpleTypeNameFromNode(typeId, source, lang, 0); t != nil {
							return t
						}
						trimmed := strings.TrimSpace(typeId.Text(source))
						return &trimmed
					}
				}
			}
		}

		return nil
	},

	ExtractVisibilityForName: func(_ *gotreesitter.Node, name string, _ *gotreesitter.Language) core.FieldVisibility {
		if name == "" {
			return core.VisibilityPublic
		}
		// __prefix (not __dunder__) = private, _prefix (not __) = protected
		if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
			return core.VisibilityPrivate
		}
		if strings.HasPrefix(name, "_") && !strings.HasPrefix(name, "__") {
			return core.VisibilityProtected
		}
		return core.VisibilityPublic
	},

	IsStatic: func(_ *gotreesitter.Node, _ *gotreesitter.Language) bool {
		return false // Python class variables don't use explicit static keyword
	},

	IsReadonly: func(_ *gotreesitter.Node, _ *gotreesitter.Language) bool {
		return false
	},
}