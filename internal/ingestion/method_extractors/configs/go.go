package configs

import (
	"strconv"
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// extractGoName extracts the method/function name from the "name" field.
func extractGoName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode == nil {
		return nil
	}
	t := nameNode.Text(source)
	return &t
}

// extractGoReturnType extracts the return type from the "result" field.
// Go supports single and multiple returns — for multiple returns, it extracts
// the first parameter's type.
func extractGoReturnType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	result := node.ChildByFieldName("result", lang)
	if result == nil {
		return nil
	}
	if result.Type(lang) != "parameter_list" {
		t := strings.TrimSpace(result.Text(source))
		return &t
	}
	// Multiple returns: (Type, error) — extract first parameter type
	for i := 0; i < int(result.NamedChildCount()); i++ {
		param := result.NamedChild(i)
		if param != nil && param.Type(lang) == "parameter_declaration" {
			typeNode := param.ChildByFieldName("type", lang)
			if typeNode != nil {
				t := strings.TrimSpace(typeNode.Text(source))
				return &t
			}
		}
	}
	return nil
}

// extractGoParameters extracts parameters from the "parameters" field.
// Go allows multiple names sharing a type: func(a, b int).
func extractGoParameters(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []core.ParameterInfo {
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
		case "parameter_declaration":
			typeNode := param.ChildByFieldName("type", lang)
			var typeName *string
			if typeNode != nil {
				if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
					typeName = s
				} else {
					typeName = strPtr(strings.TrimSpace(typeNode.Text(source)))
				}
			}
			var rawType *string
			if typeNode != nil {
				rawType = strPtr(strings.TrimSpace(typeNode.Text(source)))
			}
			// Multiple names sharing type: func(a, b int)
			var names []string
			for j := 0; j < int(param.NamedChildCount()); j++ {
				child := param.NamedChild(j)
				if child != nil && child.Type(lang) == "identifier" {
					names = append(names, child.Text(source))
				}
			}
			if len(names) == 0 {
				// Unnamed parameter: func(int, string)
				params = append(params, core.ParameterInfo{
					Name:    "_" + strconv.Itoa(i),
					Type:    typeName,
					RawType: rawType,
				})
			} else {
				for _, name := range names {
					params = append(params, core.ParameterInfo{
						Name:    name,
						Type:    typeName,
						RawType: rawType,
					})
				}
			}

		case "variadic_parameter_declaration":
			nameNode := param.ChildByFieldName("name", lang)
			typeNode := param.ChildByFieldName("type", lang)
			var typeName *string
			if typeNode != nil {
				if s := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); s != nil {
					typeName = s
				} else {
					typeName = strPtr(strings.TrimSpace(typeNode.Text(source)))
				}
			}
			var rawType *string
			if typeNode != nil {
				rawType = strPtr(strings.TrimSpace(typeNode.Text(source)))
			}
			var name string
			if nameNode != nil {
				name = nameNode.Text(source)
			} else {
				name = "_" + strconv.Itoa(i)
			}
			params = append(params, core.ParameterInfo{
				Name:       name,
				Type:       typeName,
				RawType:    rawType,
				IsVariadic: true,
			})
		}
	}
	return params
}

// extractGoVisibility determines Go visibility: uppercase first letter = public, lowercase = private.
func extractGoVisibility(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.MethodVisibility {
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode == nil || source == nil {
		return "private"
	}
	name := nameNode.Text(source)
	if len(name) == 0 {
		return "private"
	}
	first := name[0]
	if first >= 'A' && first <= 'Z' {
		return "public"
	}
	return "private"
}

// extractGoReceiverType extracts the receiver type from the "receiver" field.
func extractGoReceiverType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	receiver := node.ChildByFieldName("receiver", lang)
	if receiver == nil {
		return nil
	}
	for i := 0; i < int(receiver.NamedChildCount()); i++ {
		param := receiver.NamedChild(i)
		if param != nil && param.Type(lang) == "parameter_declaration" {
			typeNode := param.ChildByFieldName("type", lang)
			if typeNode == nil {
				continue
			}
			// Unwrap pointer_type: *User -> User
			if typeNode.Type(lang) == "pointer_type" {
				inner := firstNamedChild(typeNode, lang)
				if inner != nil {
					t := inner.Text(source)
					return &t
				}
			}
			t := typeNode.Text(source)
			return &t
		}
	}
	return nil
}

// GoMethodConfig is the Go method extraction configuration.
var GoMethodConfig = core.MethodExtractionConfig{
	Language:             core.LangGo,
	TypeDeclarationNodes: []string{"method_declaration", "function_declaration", "method_elem"},
	MethodNodeTypes:      []string{"method_declaration", "function_declaration", "method_elem"},
	BodyNodeTypes:        []string{}, // Go does not use body traversal pattern

	ExtractName:         extractGoName,
	ExtractReturnType:   extractGoReturnType,
	ExtractParameters:   extractGoParameters,
	ExtractVisibility:   extractGoVisibility,
	ExtractReceiverType: extractGoReceiverType,
	ExtractOwnerName:    extractGoReceiverType, // owner name = receiver type

	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		// Go functions (no receiver) are effectively static
		return node.Type(lang) == "function_declaration"
	},
	IsAbstract: func(node *gotreesitter.Node, _ *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		// Go interface method signatures (method_elem) are abstract
		return node.Type(lang) == "method_elem"
	},
	IsFinal: func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool { return false },
}
