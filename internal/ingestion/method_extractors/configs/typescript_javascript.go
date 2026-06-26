package configs

import (
	"regexp"
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// tsVisibilityKeywords is the set of TypeScript visibility modifiers.
var tsVisibilityKeywords = map[core.MethodVisibility]bool{
	"public":    true,
	"private":   true,
	"protected": true,
}

// jsdocReturnRE extracts @returns {Type} from JSDoc comments.
var jsdocReturnRE = regexp.MustCompile(`@returns?\s*\{([^}]+)\}`)

// sanitizeJsDocReturnType performs minimal JSDoc return type cleanup.
func sanitizeJsDocReturnType(raw string) *string {
	typ := strings.TrimSpace(raw)
	// Strip JSDoc nullable/non-nullable prefix
	if strings.HasPrefix(typ, "?") || strings.HasPrefix(typ, "!") {
		typ = typ[1:]
	}
	// Strip module: prefix
	if strings.HasPrefix(typ, "module:") {
		typ = typ[7:]
	}
	// Reject union types (ambiguous)
	if strings.Contains(typ, "|") {
		return nil
	}
	if typ == "" {
		return nil
	}
	return &typ
}

// extractJsDocReturnType extracts the return type from a preceding JSDoc comment.
// Walks PrevSibling chain looking for comment nodes with @returns annotations.
func extractJsDocReturnType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	sibling := prevSibling(node)
	for sibling != nil {
		if sibling.Type(lang) == "comment" {
			text := sibling.Text(source)
			match := jsdocReturnRE.FindStringSubmatch(text)
			if len(match) >= 2 {
				return sanitizeJsDocReturnType(match[1])
			}
		} else if sibling.IsNamed() && sibling.Type(lang) != "decorator" {
			break
		}
		sibling = prevSibling(sibling)
	}
	return nil
}

// extractTsJsParameters extracts parameters from formal_parameters.
// Handles both TS (required/optional/rest_parameter) and JS
// (identifier/assignment_pattern/rest_pattern) syntax.
func extractTsJsParameters(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []core.ParameterInfo {
	paramList := node.ChildByFieldName("parameters", lang)
	if paramList == nil {
		return nil
	}
	var params []core.ParameterInfo

	for i := 0; i < int(paramList.NamedChildCount()); i++ {
		param := paramList.NamedChild(i)
		if param == nil {
			continue
		}

		switch param.Type(lang) {
		case "required_parameter":
			patternNode := param.ChildByFieldName("pattern", lang)
			if patternNode == nil {
				break
			}
			// Skip TS `this` parameter
			if patternNode.Type(lang) == "this" {
				break
			}
			isRest := patternNode.Type(lang) == "rest_pattern"
			nameNode := patternNode
			if isRest {
				nameNode = firstNamedChild(patternNode, lang)
			}
			if nameNode == nil {
				break
			}
			typeAnnotation := param.ChildByFieldName("type", lang)
			var typeNode *gotreesitter.Node
			if typeAnnotation != nil {
				typeNode = firstNamedChild(typeAnnotation, lang)
			}
			hasDefault := param.ChildByFieldName("value", lang) != nil

			var typ *string
			if typeNode != nil {
				if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
					typ = s
				} else {
					typ = strPtr(strings.TrimSpace(typeNode.Text(source)))
				}
			}
			var rawType *string
			if typeNode != nil {
				rawType = strPtr(strings.TrimSpace(typeNode.Text(source)))
			}
			params = append(params, core.ParameterInfo{
				Name:       nameNode.Text(source),
				Type:       typ,
				RawType:    rawType,
				IsOptional: hasDefault,
				IsVariadic: isRest,
			})

		case "optional_parameter":
			nameNode := param.ChildByFieldName("pattern", lang)
			if nameNode == nil {
				break
			}
			typeAnnotation := param.ChildByFieldName("type", lang)
			var typeNode *gotreesitter.Node
			if typeAnnotation != nil {
				typeNode = firstNamedChild(typeAnnotation, lang)
			}
			var typ *string
			if typeNode != nil {
				if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
					typ = s
				} else {
					typ = strPtr(strings.TrimSpace(typeNode.Text(source)))
				}
			}
			var rawType *string
			if typeNode != nil {
				rawType = strPtr(strings.TrimSpace(typeNode.Text(source)))
			}
			params = append(params, core.ParameterInfo{
				Name:       nameNode.Text(source),
				Type:       typ,
				RawType:    rawType,
				IsOptional: true,
			})

		case "rest_parameter":
			nameNode := param.ChildByFieldName("pattern", lang)
			if nameNode == nil {
				break
			}
			typeAnnotation := param.ChildByFieldName("type", lang)
			var typeNode *gotreesitter.Node
			if typeAnnotation != nil {
				typeNode = firstNamedChild(typeAnnotation, lang)
			}
			var typ *string
			if typeNode != nil {
				if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
					typ = s
				} else {
					typ = strPtr(strings.TrimSpace(typeNode.Text(source)))
				}
			}
			var rawType *string
			if typeNode != nil {
				rawType = strPtr(strings.TrimSpace(typeNode.Text(source)))
			}
			params = append(params, core.ParameterInfo{
				Name:       nameNode.Text(source),
				Type:       typ,
				RawType:    rawType,
				IsVariadic: true,
			})

		case "identifier":
			// JS: bare parameter name, no type
			params = append(params, core.ParameterInfo{
				Name: param.Text(source),
			})

		case "assignment_pattern":
			// JS: param = defaultValue
			left := param.ChildByFieldName("left", lang)
			if left != nil {
				params = append(params, core.ParameterInfo{
					Name:       left.Text(source),
					IsOptional: true,
				})
			}

		case "rest_pattern":
			// JS: ...args
			inner := firstNamedChild(param, lang)
			if inner != nil {
				params = append(params, core.ParameterInfo{
					Name:       inner.Text(source),
					IsVariadic: true,
				})
			}

		case "object_pattern", "array_pattern":
			// Destructured parameter — use full text as name
			params = append(params, core.ParameterInfo{
				Name: param.Text(source),
			})
		}
	}
	return params
}

// extractTsJsReturnType extracts the return type from the return_type field,
// falling back to JSDoc @returns annotation.
func extractTsJsReturnType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	returnType := node.ChildByFieldName("return_type", lang)
	if returnType != nil {
		if returnType.Type(lang) == "type_annotation" {
			inner := firstNamedChild(returnType, lang)
			if inner != nil {
				t := strings.TrimSpace(inner.Text(source))
				return &t
			}
		}
		t := strings.TrimSpace(returnType.Text(source))
		return &t
	}
	// No AST return type annotation — try JSDoc fallback
	return extractJsDocReturnType(node, source, lang)
}

// extractTsJsVisibility extracts visibility from accessibility_modifier or #private name.
func extractTsJsVisibility(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.MethodVisibility {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "accessibility_modifier" {
			t := strings.TrimSpace(child.Text(source))
			if tsVisibilityKeywords[core.MethodVisibility(t)] {
				return core.MethodVisibility(t)
			}
		}
	}
	// ES2022 private method (#name)
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode != nil && nameNode.Type(lang) == "private_property_identifier" {
		return "private"
	}
	return "public"
}

// extractTsJsDecorators extracts decorator names from preceding sibling decorator nodes.
func extractTsJsDecorators(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
	var decorators []string
	sibling := prevNamedSibling(node, lang)
	for sibling != nil && sibling.Type(lang) == "decorator" {
		name := extractTsJsDecoratorNameFromNode(sibling, source, lang)
		if name != nil {
			decorators = append([]string{*name}, decorators...)
		}
		sibling = prevNamedSibling(sibling, lang)
	}
	return decorators
}

// extractTsJsDecoratorNameFromNode extracts the decorator name from a decorator node.
func extractTsJsDecoratorNameFromNode(decorator *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	expr := firstNamedChild(decorator, lang)
	if expr == nil {
		return nil
	}
	switch expr.Type(lang) {
	case "call_expression":
		fn := expr.ChildByFieldName("function", lang)
		if fn != nil {
			return strPtr("@" + fn.Text(source))
		}
		return nil
	case "identifier":
		return strPtr("@" + expr.Text(source))
	case "member_expression":
		return strPtr("@" + expr.Text(source))
	}
	return nil
}

// sharedMethodConfig is the base method config shared by TS and JS (no Language field).
var sharedMethodConfig = core.MethodExtractionConfig{
	TypeDeclarationNodes: []string{
		"class_declaration", "abstract_class_declaration", "interface_declaration",
	},
	MethodNodeTypes: []string{
		"method_definition", "method_signature", "abstract_method_signature",
		"function_declaration", "generator_function_declaration", "function_signature",
	},
	BodyNodeTypes: []string{"class_body", "interface_body"},

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil {
			return nil
		}
		t := nameNode.Text(source)
		return &t
	},

	ExtractReturnType: extractTsJsReturnType,
	ExtractParameters: extractTsJsParameters,
	ExtractVisibility: extractTsJsVisibility,

	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "static", lang)
	},
	IsAbstract: func(node, ownerNode *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		if configs.HasKeyword(node, "abstract", lang) {
			return true
		}
		// Interface methods are implicitly abstract
		return ownerNode.Type(lang) == "interface_declaration"
	},
	IsFinal:            func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool { return false },
	ExtractAnnotations: extractTsJsDecorators,

	IsAsync: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "async", lang)
	},
	IsOverride: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "override", lang)
	},
}

// TypeScriptMethodConfig is the method extraction configuration for TypeScript.
var TypeScriptMethodConfig = core.MethodExtractionConfig{
	Language:             core.LangTypeScript,
	TypeDeclarationNodes: sharedMethodConfig.TypeDeclarationNodes,
	MethodNodeTypes:      sharedMethodConfig.MethodNodeTypes,
	BodyNodeTypes:        sharedMethodConfig.BodyNodeTypes,
	ExtractName:          sharedMethodConfig.ExtractName,
	ExtractReturnType:    sharedMethodConfig.ExtractReturnType,
	ExtractParameters:    sharedMethodConfig.ExtractParameters,
	ExtractVisibility:    sharedMethodConfig.ExtractVisibility,
	IsStatic:             sharedMethodConfig.IsStatic,
	IsAbstract:           sharedMethodConfig.IsAbstract,
	IsFinal:              sharedMethodConfig.IsFinal,
	ExtractAnnotations:   sharedMethodConfig.ExtractAnnotations,
	IsAsync:              sharedMethodConfig.IsAsync,
	IsOverride:           sharedMethodConfig.IsOverride,
}

// JavaScriptMethodConfig is the method extraction configuration for JavaScript.
var JavaScriptMethodConfig = core.MethodExtractionConfig{
	Language:             core.LangJavaScript,
	TypeDeclarationNodes: sharedMethodConfig.TypeDeclarationNodes,
	MethodNodeTypes:      sharedMethodConfig.MethodNodeTypes,
	BodyNodeTypes:        sharedMethodConfig.BodyNodeTypes,
	ExtractName:          sharedMethodConfig.ExtractName,
	ExtractReturnType:    sharedMethodConfig.ExtractReturnType,
	ExtractParameters:    sharedMethodConfig.ExtractParameters,
	ExtractVisibility:    sharedMethodConfig.ExtractVisibility,
	IsStatic:             sharedMethodConfig.IsStatic,
	IsAbstract:           sharedMethodConfig.IsAbstract,
	IsFinal:              sharedMethodConfig.IsFinal,
	ExtractAnnotations:   sharedMethodConfig.ExtractAnnotations,
	IsAsync:              sharedMethodConfig.IsAsync,
	IsOverride:           sharedMethodConfig.IsOverride,
}