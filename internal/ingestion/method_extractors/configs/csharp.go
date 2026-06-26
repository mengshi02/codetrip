package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// csharpVisSet is the set of C# visibility modifiers.
var csharpVisSet = map[core.FieldVisibility]bool{
	"public":    true,
	"private":   true,
	"protected": true,
	"internal":  true,
}

// extractCSharpVisibility extracts the visibility of a C# method node.
// Extracted as a standalone function to avoid initialization cycle in CSharpMethodConfig.
func extractCSharpVisibility(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.MethodVisibility {
	mods := configs.CollectModifierTexts(node, "modifier", lang)
	if mods["protected"] && mods["internal"] {
		return core.VisibilityProtectedInternal
	}
	if mods["private"] && mods["protected"] {
		return core.VisibilityPrivateProtected
	}
	return configs.FindVisibility(node, csharpVisSet, core.VisibilityPrivate, "modifier", lang)
}

// extractCSharpParameters extracts parameters from the "parameters" field.
// C# params keyword is a bare unnamed token not wrapped in a parameter node.
func extractCSharpParameters(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []core.ParameterInfo {
	paramList := node.ChildByFieldName("parameters", lang)
	if paramList == nil {
		return nil
	}
	return extractParametersFromList(paramList, source, lang)
}

// extractParametersFromList extracts parameters from a parameter_list node.
// Shared between extractCSharpParameters and ExtractPrimaryConstructor.
func extractParametersFromList(paramList *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []core.ParameterInfo {
	var params []core.ParameterInfo
	i := 0
	for i < int(paramList.ChildCount()) {
		child := paramList.Child(i)
		if child == nil {
			i++
			continue
		}

		// params variadic: bare unnamed token followed by type + identifier siblings
		if !child.IsNamed() && child.Type(lang) == "params" {
			var typeNode *gotreesitter.Node
			var nameText *string
			j := i + 1
			for j < int(paramList.ChildCount()) {
				sibling := paramList.Child(j)
				if sibling == nil {
					j++
					continue
				}
				if sibling.IsNamed() && sibling.Type(lang) != "parameter" {
					if typeNode == nil {
						typeNode = sibling
					} else if sibling.Type(lang) == "identifier" {
						t := sibling.Text(source)
						nameText = &t
						i = j
						break
					}
				}
				j++
			}
			if nameText != nil {
				var typ *string
				if typeNode != nil {
					if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
						typ = s
					} else {
						t := strings.TrimSpace(typeNode.Text(source))
						typ = &t
					}
				}
				var rawType *string
				if typeNode != nil {
					t := strings.TrimSpace(typeNode.Text(source))
					rawType = &t
				}
				params = append(params, core.ParameterInfo{
					Name:       *nameText,
					Type:       typ,
					RawType:    rawType,
					IsVariadic: true,
				})
			}
			i++
			continue
		}

		// Normal named parameter node
		if child.IsNamed() && child.Type(lang) == "parameter" {
			nameNode := child.ChildByFieldName("name", lang)
			if nameNode != nil && strings.TrimSpace(nameNode.Text(source)) != "" {
				typeNode := child.ChildByFieldName("type", lang)
				var typeName *string
				if typeNode != nil {
					if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
						typeName = s
					} else {
						t := strings.TrimSpace(typeNode.Text(source))
						typeName = &t
					}
				}

				// ref/out/in/this modifier prefix
				for j := 0; j < int(child.NamedChildCount()); j++ {
					c := child.NamedChild(j)
					if c == nil || c.Type(lang) != "modifier" {
						continue
					}
					modText := strings.TrimSpace(c.Text(source))
					if modText == "out" || modText == "ref" || modText == "in" || modText == "this" {
						if typeName != nil {
							typeName = strPtr(modText + " " + *typeName)
						} else {
							typeName = strPtr(modText)
						}
						break
					}
				}

				// Default value detection (= token)
				isOptional := false
				for j := 0; j < int(child.ChildCount()); j++ {
					c := child.Child(j)
					if c != nil && strings.TrimSpace(c.Type(lang)) == "=" {
						isOptional = true
						break
					}
				}

				var rawType *string
				if typeNode != nil {
					rawType = strPtr(strings.TrimSpace(typeNode.Text(source)))
				}
				params = append(params, core.ParameterInfo{
					Name:       nameNode.Text(source),
					Type:       typeName,
					RawType:    rawType,
					IsOptional: isOptional,
				})
			}
		}
		i++
	}
	return params
}

// extractCSharpAnnotations extracts C# attributes from attribute_list nodes.
// Skips attribute lists that have a target specifier (e.g. [return: NotNull]).
func extractCSharpAnnotations(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
	var annotations []string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != "attribute_list" {
			continue
		}
		// Skip attribute lists with target specifier
		hasTarget := false
		for j := 0; j < int(child.NamedChildCount()); j++ {
			if child.NamedChild(j) != nil && child.NamedChild(j).Type(lang) == "attribute_target_specifier" {
				hasTarget = true
				break
			}
		}
		if hasTarget {
			continue
		}
		for j := 0; j < int(child.NamedChildCount()); j++ {
			attr := child.NamedChild(j)
			if attr == nil || attr.Type(lang) != "attribute" {
				continue
			}
			nameNode := attr.ChildByFieldName("name", lang)
			if nameNode != nil {
				annotations = append(annotations, "@"+nameNode.Text(source))
			}
		}
	}
	return annotations
}

// CSharpMethodConfig is the method extraction configuration for C#.
var CSharpMethodConfig = core.MethodExtractionConfig{
	Language: core.LangCSharp,
	TypeDeclarationNodes: []string{
		"class_declaration", "struct_declaration",
		"interface_declaration", "record_declaration",
	},
	MethodNodeTypes: []string{
		"method_declaration", "constructor_declaration",
		"destructor_declaration", "operator_declaration",
		"conversion_operator_declaration", "local_function_statement",
	},
	BodyNodeTypes: []string{"declaration_list"},

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// destructor: ~ClassName
		if node.Type(lang) == "destructor_declaration" {
			name := node.ChildByFieldName("name", lang)
			if name != nil {
				return strPtr("~" + name.Text(source))
			}
			return nil
		}
		// operator: operator + / operator ==
		if node.Type(lang) == "operator_declaration" {
			op := node.ChildByFieldName("operator", lang)
			if op != nil {
				return strPtr("operator " + strings.TrimSpace(op.Text(source)))
			}
			return nil
		}
		// conversion_operator: implicit/explicit operator Type
		if node.Type(lang) == "conversion_operator_declaration" {
			typeNode := node.ChildByFieldName("type", lang)
			var typeName *string
			if typeNode != nil {
				if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
					typeName = s
				} else {
					typeName = strPtr(strings.TrimSpace(typeNode.Text(source)))
				}
			}
			for i := 0; i < int(node.ChildCount()); i++ {
				c := node.Child(i)
				if c != nil && !c.IsNamed() {
					text := c.Type(lang)
					if text == "implicit" || text == "explicit" {
						if typeName != nil {
							return strPtr(text + " operator " + *typeName)
						}
						return nil
					}
				}
			}
			if typeName != nil {
				return strPtr("operator " + *typeName)
			}
			return nil
		}
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil {
			return nil
		}
		t := nameNode.Text(source)
		return &t
	},

	ExtractReturnType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// Constructor/destructor have no return type
		returnsNode := node.ChildByFieldName("returns", lang)
		if returnsNode != nil {
			t := strings.TrimSpace(returnsNode.Text(source))
			return &t
		}
		// operator/conversion declarations use "type" field as return type
		if node.Type(lang) == "operator_declaration" || node.Type(lang) == "conversion_operator_declaration" {
			typeNode := node.ChildByFieldName("type", lang)
			if typeNode != nil {
				if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
					return s
				}
				t := strings.TrimSpace(typeNode.Text(source))
				return &t
			}
		}
		return nil
	},

	ExtractParameters: extractCSharpParameters,

	ExtractVisibility: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.MethodVisibility {
		mods := configs.CollectModifierTexts(node, "modifier", lang)
		if mods["protected"] && mods["internal"] {
			return core.VisibilityProtectedInternal
		}
		if mods["private"] && mods["protected"] {
			return core.VisibilityPrivateProtected
		}
		return configs.FindVisibility(node, csharpVisSet, core.VisibilityPrivate, "modifier", lang)
	},

	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "static", lang) || configs.HasModifier(node, "modifier", "static", lang)
	},

	IsAbstract: func(node, ownerNode *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		if configs.HasKeyword(node, "abstract", lang) || configs.HasModifier(node, "modifier", "abstract", lang) {
			return true
		}
		if ownerNode.Type(lang) == "interface_declaration" {
			body := node.ChildByFieldName("body", lang)
			return body == nil
		}
		return false
	},

	IsFinal: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		// C# uses 'sealed' instead of 'final'
		return configs.HasKeyword(node, "sealed", lang) || configs.HasModifier(node, "modifier", "sealed", lang)
	},

	ExtractAnnotations: extractCSharpAnnotations,

	IsVirtual: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "virtual", lang) || configs.HasModifier(node, "modifier", "virtual", lang)
	},

	IsOverride: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "override", lang) || configs.HasModifier(node, "modifier", "override", lang)
	},

	IsAsync: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "async", lang) || configs.HasModifier(node, "modifier", "async", lang)
	},

	IsPartial: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "partial", lang) || configs.HasModifier(node, "modifier", "partial", lang)
	},

	ExtractPrimaryConstructor: func(ownerNode *gotreesitter.Node, ctx *core.MethodExtractorContext, source []byte, lang *gotreesitter.Language) *core.MethodInfo {
		// C# 12 primary constructor: class Point(int x, int y) { }
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
		nameNode := ownerNode.ChildByFieldName("name", lang)
		if nameNode == nil {
			return nil
		}
		name := nameNode.Text(source)
		parameters := extractParametersFromList(paramList, source, lang)
		visibility := extractCSharpVisibility(ownerNode, source, lang)
		return &core.MethodInfo{
			Name:        name,
			Parameters:  parameters,
			Visibility:  visibility,
			Annotations: []string{},
			SourceFile:  ctx.FilePath,
			Line:        int(paramList.StartPoint().Row) + 1,
		}
	},
}
