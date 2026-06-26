package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// selfNames contains Python conventional first-parameter names for instance/class methods.
var selfNames = map[string]bool{"self": true, "cls": true}

// unwrapDecorated unwraps a decorated_definition to its inner function_definition.
func unwrapDecorated(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if node.Type(lang) == "decorated_definition" {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "function_definition" {
				return child
			}
		}
	}
	return node
}

// collectDecorators collects decorator nodes from a decorated_definition wrapper.
func collectDecorators(node *gotreesitter.Node, lang *gotreesitter.Language) []*gotreesitter.Node {
	var wrapper *gotreesitter.Node
	if node.Type(lang) == "decorated_definition" {
		wrapper = node
	} else if parent := node.Parent(); parent != nil && parent.Type(lang) == "decorated_definition" {
		wrapper = parent
	}
	if wrapper == nil {
		return nil
	}
	var decorators []*gotreesitter.Node
	for i := 0; i < int(wrapper.NamedChildCount()); i++ {
		child := wrapper.NamedChild(i)
		if child != nil && child.Type(lang) == "decorator" {
			decorators = append(decorators, child)
		}
	}
	return decorators
}

// extractDecoratorName extracts the decorator name (prefixed with @) from a decorator node.
func extractDecoratorName(decorator *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	expr := firstNamedChild(decorator, lang)
	if expr == nil {
		return nil
	}
	switch expr.Type(lang) {
	case "identifier":
		return strPtr("@" + expr.Text(source))
	case "attribute":
		return strPtr("@" + expr.Text(source))
	case "call":
		fn := expr.ChildByFieldName("function", lang)
		if fn != nil {
			return strPtr("@" + fn.Text(source))
		}
	}
	return nil
}

// hasDecorator checks whether a node has a decorator with the given name.
func hasDecorator(node *gotreesitter.Node, source []byte, name string, lang *gotreesitter.Language) bool {
	decorators := collectDecorators(node, lang)
	for _, dec := range decorators {
		decName := extractDecoratorName(dec, source, lang)
		if decName != nil && (*decName == "@"+name || strings.HasSuffix(*decName, "."+name)) {
			return true
		}
	}
	return false
}

// extractPythonParameters extracts parameters from a function parameter list, skipping self/cls.
// Python has 7 parameter types: identifier, default_parameter, typed_parameter,
// typed_default_parameter, list_splat_pattern, dictionary_splat_pattern.
func extractPythonParameters(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []core.ParameterInfo {
	funcNode := unwrapDecorated(node, lang)
	paramList := funcNode.ChildByFieldName("parameters", lang)
	if paramList == nil {
		return nil
	}
	var params []core.ParameterInfo
	isFirst := true

	for i := 0; i < int(paramList.NamedChildCount()); i++ {
		param := paramList.NamedChild(i)
		if param == nil {
			continue
		}

		switch param.Type(lang) {
		case "identifier":
			// Bare parameter: self/cls or untyped x
			text := param.Text(source)
			if isFirst && selfNames[text] {
				isFirst = false
				continue
			}
			isFirst = false
			params = append(params, core.ParameterInfo{Name: text})

		case "default_parameter":
			// x = value — untyped with default
			isFirst = false
			nameNode := param.ChildByFieldName("name", lang)
			if nameNode != nil {
				params = append(params, core.ParameterInfo{
					Name:       nameNode.Text(source),
					IsOptional: true,
				})
			}

		case "typed_parameter":
			// x: int or *args: str or **kwargs: int
			inner := firstNamedChild(param, lang)
			if inner == nil {
				isFirst = false
				break
			}
			if isFirst && inner.Type(lang) == "identifier" && selfNames[inner.Text(source)] {
				isFirst = false
				continue
			}
			isFirst = false
			typeNode := param.ChildByFieldName("type", lang)
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
			switch inner.Type(lang) {
			case "list_splat_pattern":
				nameId := firstNamedChild(inner, lang)
				if nameId != nil {
					params = append(params, core.ParameterInfo{
						Name: nameId.Text(source), Type: typ, RawType: rawType, IsVariadic: true,
					})
				}
			case "dictionary_splat_pattern":
				nameId := firstNamedChild(inner, lang)
				if nameId != nil {
					params = append(params, core.ParameterInfo{
						Name: nameId.Text(source), Type: typ, RawType: rawType, IsVariadic: true,
					})
				}
			default:
				params = append(params, core.ParameterInfo{
					Name: inner.Text(source), Type: typ, RawType: rawType,
				})
			}

		case "typed_default_parameter":
			// x: int = 5
			isFirst = false
			nameNode := param.ChildByFieldName("name", lang)
			typeNode := param.ChildByFieldName("type", lang)
			if nameNode != nil {
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
					Name: nameNode.Text(source), Type: typ, RawType: rawType, IsOptional: true,
				})
			}

		case "list_splat_pattern":
			// *args (untyped)
			isFirst = false
			nameId := firstNamedChild(param, lang)
			if nameId != nil {
				params = append(params, core.ParameterInfo{
					Name: nameId.Text(source), IsVariadic: true,
				})
			}

		case "dictionary_splat_pattern":
			// **kwargs (untyped)
			isFirst = false
			nameId := firstNamedChild(param, lang)
			if nameId != nil {
				params = append(params, core.ParameterInfo{
					Name: nameId.Text(source), IsVariadic: true,
				})
			}

		default:
			isFirst = false
		}
	}
	return params
}

// extractPythonReturnType extracts the return type from the return_type field.
func extractPythonReturnType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	funcNode := unwrapDecorated(node, lang)
	returnType := funcNode.ChildByFieldName("return_type", lang)
	if returnType == nil {
		return nil
	}
	t := strings.TrimSpace(returnType.Text(source))
	return &t
}

// extractPythonVisibility determines visibility from Python naming conventions.
// __name (not dunder) -> private, _name -> protected, otherwise public.
func extractPythonVisibility(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.MethodVisibility {
	funcNode := unwrapDecorated(node, lang)
	nameNode := funcNode.ChildByFieldName("name", lang)
	if nameNode == nil {
		return core.VisibilityPublic
	}
	name := nameNode.Text(source)
	if strings.HasPrefix(name, "__") && !strings.HasSuffix(name, "__") {
		return core.VisibilityPrivate
	}
	if strings.HasPrefix(name, "_") && !(strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__")) {
		return core.VisibilityProtected
	}
	return core.VisibilityPublic
}

// PythonMethodConfig is the method extraction configuration for Python.
var PythonMethodConfig = core.MethodExtractionConfig{
	Language:             core.LangPython,
	TypeDeclarationNodes: []string{"class_definition"},
	MethodNodeTypes:      []string{"function_definition", "decorated_definition"},
	BodyNodeTypes:        []string{"block"},

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		funcNode := unwrapDecorated(node, lang)
		nameNode := funcNode.ChildByFieldName("name", lang)
		if nameNode == nil {
			return nil
		}
		t := nameNode.Text(source)
		return &t
	},

	ExtractReturnType: extractPythonReturnType,
	ExtractParameters: extractPythonParameters,
	ExtractVisibility: extractPythonVisibility,

	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		// Note: Python static/classmethod detection ideally needs source for
		// decorator text inspection, but HasKeyword on the unwrapped node works
		// as a fallback for the common patterns.
		return configs.HasKeyword(node, "staticmethod", lang) || configs.HasKeyword(node, "classmethod", lang)
	},
	IsAbstract: func(node *gotreesitter.Node, _ *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "abstractmethod", lang)
	},
	IsFinal: func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool { return false },

	ExtractAnnotations: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
		decorators := collectDecorators(node, lang)
		var annotations []string
		for _, dec := range decorators {
			name := extractDecoratorName(dec, source, lang)
			if name != nil {
				annotations = append(annotations, *name)
			}
		}
		return annotations
	},

	IsAsync: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		funcNode := unwrapDecorated(node, lang)
		return configs.HasKeyword(funcNode, "async", lang)
	},
}
