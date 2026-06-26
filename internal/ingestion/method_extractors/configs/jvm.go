package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// ---------------------------------------------------------------------------
// Shared JVM helpers
// ---------------------------------------------------------------------------

// interfaceOwnerTypes lists AST node types that represent interface-like owners.
// Methods inside these are implicitly abstract unless they have a body.
var interfaceOwnerTypes = map[string]bool{
	"interface_declaration":       true,
	"annotation_type_declaration": true,
}

// extractReturnTypeFromField extracts the return type from the "type" field child.
func extractReturnTypeFromField(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return nil
	}
	t := strings.TrimSpace(typeNode.Text(source))
	return &t
}

// extractJvmAnnotations extracts annotations from modifier wrapper nodes.
// In Java, annotations appear as marker_annotation or annotation inside modifiers nodes.
func extractJvmAnnotations(node *gotreesitter.Node, source []byte, modifierType string, lang *gotreesitter.Language) []string {
	var annotations []string
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != modifierType {
			continue
		}
		for j := 0; j < int(child.NamedChildCount()); j++ {
			mod := child.NamedChild(j)
			if mod == nil {
				continue
			}
			if mod.Type(lang) == "marker_annotation" || mod.Type(lang) == "annotation" {
				nameNode := mod.ChildByFieldName("name", lang)
				if nameNode == nil {
					nameNode = firstNamedChild(mod, lang)
				}
				if nameNode != nil {
					annotations = append(annotations, "@"+nameNode.Text(source))
				}
			}
		}
	}
	return annotations
}

// ---------------------------------------------------------------------------
// Java
// ---------------------------------------------------------------------------

// javaVisSet is the set of Java visibility modifiers.
var javaVisSet = map[core.FieldVisibility]bool{
	"public":    true,
	"private":   true,
	"protected": true,
}

// extractJavaParameters extracts method parameters from the "parameters" field.
// Handles formal_parameter, spread_parameter (varargs), and compact constructor
// parameter inheritance from record_declaration.
func extractJavaParameters(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []core.ParameterInfo {
	var params []core.ParameterInfo
	paramList := node.ChildByFieldName("parameters", lang)
	// Compact constructor has no parameter list — inherit from parent record_declaration
	if paramList == nil && node.Type(lang) == "compact_constructor_declaration" {
		if parent := node.Parent(); parent != nil {
			if grandParent := parent.Parent(); grandParent != nil && grandParent.Type(lang) == "record_declaration" {
				paramList = grandParent.ChildByFieldName("parameters", lang)
			}
		}
	}
	if paramList == nil {
		return nil
	}

	for i := 0; i < int(paramList.NamedChildCount()); i++ {
		param := paramList.NamedChild(i)
		if param == nil {
			continue
		}
		switch param.Type(lang) {
		case "formal_parameter":
			nameNode := param.ChildByFieldName("name", lang)
			typeNode := param.ChildByFieldName("type", lang)
			if nameNode != nil {
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
					Name:    nameNode.Text(source),
					Type:    typ,
					RawType: rawType,
				})
			}
		case "spread_parameter":
			var paramName *string
			var paramType *string
			var paramRawType *string
			for j := 0; j < int(param.NamedChildCount()); j++ {
				c := param.NamedChild(j)
				if c == nil {
					continue
				}
				if c.Type(lang) == "variable_declarator" {
					nameChild := c.ChildByFieldName("name", lang)
					if nameChild != nil {
						t := nameChild.Text(source)
						paramName = &t
					} else {
						t := c.Text(source)
						paramName = &t
					}
				} else if c.Type(lang) == "type_identifier" || c.Type(lang) == "generic_type" ||
					c.Type(lang) == "scoped_type_identifier" || c.Type(lang) == "integral_type" ||
					c.Type(lang) == "floating_point_type" || c.Type(lang) == "boolean_type" {
					t := strings.TrimSpace(c.Text(source))
					paramRawType = &t
					if s := typeextractors.ExtractSimpleTypeNameFromNode(c, source, lang, 0); s != nil {
						paramType = s
					} else {
						paramType = paramRawType
					}
				}
			}
			if paramName != nil {
				params = append(params, core.ParameterInfo{
					Name:       *paramName,
					Type:       paramType,
					RawType:    paramRawType,
					IsVariadic: true,
				})
			}
		}
	}
	return params
}

// JavaMethodConfig is the method extraction configuration for Java.
var JavaMethodConfig = core.MethodExtractionConfig{
	Language: core.LangJava,
	TypeDeclarationNodes: []string{
		"class_declaration", "interface_declaration",
		"enum_declaration", "record_declaration", "annotation_type_declaration",
	},
	MethodNodeTypes: []string{
		"method_declaration", "constructor_declaration",
		"compact_constructor_declaration", "annotation_type_element_declaration",
	},
	BodyNodeTypes: []string{
		"class_body", "interface_body", "enum_body",
		"enum_body_declarations", "annotation_type_body",
	},

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil {
			return nil
		}
		t := nameNode.Text(source)
		return &t
	},
	ExtractReturnType: extractReturnTypeFromField,
	ExtractParameters: extractJavaParameters,
	ExtractVisibility: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.MethodVisibility {
		return configs.FindVisibility(node, javaVisSet, core.VisibilityPackage, "modifiers", lang)
	},
	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasModifier(node, "modifiers", "static", lang)
	},
	IsAbstract: func(node, ownerNode *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		if configs.HasModifier(node, "modifiers", "abstract", lang) {
			return true
		}
		// Interface methods are implicitly abstract unless they have a body (default method)
		if interfaceOwnerTypes[ownerNode.Type(lang)] {
			body := node.ChildByFieldName("body", lang)
			return body == nil
		}
		return false
	},
	IsFinal: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasModifier(node, "modifiers", "final", lang)
	},
	ExtractAnnotations: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
		return extractJvmAnnotations(node, source, "modifiers", lang)
	},
}
