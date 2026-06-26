package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// CSharpFieldConfig extracts field declarations from C# class/struct/interface/record bodies.
//
// Handles field_declaration and property_declaration inside declaration_list.
// Visibility: compound visibilities (protected internal, private protected) are detected.
// Record positional parameters become public init-only properties via ExtractPrimaryFields.
//
// Ported from TS field-extractors/configs/csharp.ts.
var CSharpFieldConfig = core.FieldExtractionConfig{
	Language: core.LangCSharp,
	TypeDeclarationNodes: []string{
		"class_declaration",
		"struct_declaration",
		"interface_declaration",
		"record_declaration",
	},
	FieldNodeTypes:    []string{"field_declaration", "property_declaration"},
	BodyNodeTypes:     []string{"declaration_list"},
	DefaultVisibility: core.VisibilityPrivate,

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// field_declaration > variable_declaration > variable_declarator > identifier
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "variable_declaration" {
				for j := 0; j < int(child.NamedChildCount()); j++ {
					declarator := child.NamedChild(j)
					if declarator != nil && declarator.Type(lang) == "variable_declarator" {
						nameNode := declarator.ChildByFieldName("name", lang)
						if nameNode != nil {
							t := nameNode.Text(source)
							return &t
						}
						first := typeextractors.FirstNamedChild(declarator, lang)
						if first != nil {
							t := first.Text(source)
							return &t
						}
					}
				}
			}
		}
		// property_declaration: name field
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode != nil {
			t := nameNode.Text(source)
			return &t
		}
		return nil
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// field_declaration > variable_declaration > type
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "variable_declaration" {
				typeNode := child.ChildByFieldName("type", lang)
				if typeNode != nil {
					return extractCSharpDeclaredType(typeNode, source, lang)
				}
				first := typeextractors.FirstNamedChild(child, lang)
				if first != nil && first.Type(lang) != "variable_declarator" {
					return extractCSharpDeclaredType(first, source, lang)
				}
			}
		}
		// property_declaration: type is first named child or named 'type' field
		typeNode := node.ChildByFieldName("type", lang)
		if typeNode != nil {
			return extractCSharpDeclaredType(typeNode, source, lang)
		}
		return nil
	},

	ExtractVisibility: func(node *gotreesitter.Node, lang *gotreesitter.Language) core.FieldVisibility {
		// Detect compound C# visibilities: protected internal, private protected
		mods := CollectModifierTexts(node, "modifier", lang)
		if mods["protected"] && mods["internal"] {
			return core.VisibilityProtectedInternal
		}
		if mods["private"] && mods["protected"] {
			return core.VisibilityPrivateProtected
		}
		return FindVisibility(node, cSharpVisSet, core.VisibilityPrivate, "modifier", lang)
	},

	IsStatic: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "static", lang) || HasModifier(node, "modifier", "static", lang)
	},

	IsReadonly: func(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
		return HasKeyword(node, "readonly", lang) || HasModifier(node, "modifier", "readonly", lang)
	},

	ExtractPrimaryFields: func(ownerNode *gotreesitter.Node, ctx *core.FieldExtractorContext, source []byte, lang *gotreesitter.Language) []core.FieldInfo {
		// C# record positional parameters become public init-only properties.
		// C# 12 class primary constructor parameters are captured as private fields.
		var paramList *gotreesitter.Node
		for i := 0; i < int(ownerNode.NamedChildCount()); i++ {
			child := ownerNode.NamedChild(i)
			if child != nil && child.Type(lang) == "parameter_list" {
				paramList = child
				break
			}
		}
		if paramList == nil {
			return nil
		}

		isRecord := ownerNode.Type(lang) == "record_declaration"
		var fields []core.FieldInfo

		for i := 0; i < int(paramList.NamedChildCount()); i++ {
			param := paramList.NamedChild(i)
			if param == nil || param.Type(lang) != "parameter" {
				continue
			}
			nameNode := param.ChildByFieldName("name", lang)
			typeNode := param.ChildByFieldName("type", lang)
			if nameNode == nil {
				continue
			}

			var typ *string
			if typeNode != nil {
				if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
					typ = t
				} else {
					trimmed := strings.TrimSpace(typeNode.Text(source))
					typ = &trimmed
				}
			}

			fields = append(fields, core.FieldInfo{
				Name:       nameNode.Text(source),
				Type:       typ,
				Visibility: func() core.FieldVisibility { if isRecord { return core.VisibilityPublic } else { return core.VisibilityPrivate } }(),
				IsStatic:   false,
				IsReadonly: isRecord,
				SourceFile: ctx.FilePath,
				Line:       int(param.StartPoint().Row) + 1,
			})
		}

		return fields
	},
}

// cSharpVisSet maps C# visibility keywords.
var cSharpVisSet = map[core.FieldVisibility]bool{
	core.VisibilityPublic:    true,
	core.VisibilityPrivate:   true,
	core.VisibilityProtected: true,
	core.VisibilityInternal:  true,
}

// extractCSharpDeclaredType extracts a type name from a C# type AST node.
func extractCSharpDeclaredType(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	if typeNode.Type(lang) == "generic_name" {
		t := strings.TrimSpace(typeNode.Text(source))
		return &t
	}
	if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
		return t
	}
	trimmed := strings.TrimSpace(typeNode.Text(source))
	return &trimmed
}