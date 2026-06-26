package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// extractRustMethodName extracts the method name from function_item/function_signature_item.
func extractRustMethodName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode == nil {
		return nil
	}
	t := nameNode.Text(source)
	return &t
}

// extractRustReturnType extracts the return type from the return_type field.
func extractRustReturnType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	typeNode := node.ChildByFieldName("return_type", lang)
	if typeNode == nil {
		return nil
	}
	t := strings.TrimSpace(typeNode.Text(source))
	return &t
}

// extractRustParameters extracts parameters, skipping self_parameter (handled by extractReceiverType).
func extractRustParameters(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []core.ParameterInfo {
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
		// Skip self_parameter — it is the receiver
		if param.Type(lang) == "self_parameter" {
			continue
		}
		if param.Type(lang) == "parameter" {
			patternNode := param.ChildByFieldName("pattern", lang)
			typeNode := param.ChildByFieldName("type", lang)
			var name string
			if patternNode != nil {
				name = patternNode.Text(source)
			} else {
				name = "?"
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
				Name: name, Type: typ, RawType: rawType,
			})
		}
	}
	return params
}

// extractRustVisibility detects visibility from visibility_modifier children.
func extractRustVisibility(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.MethodVisibility {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "visibility_modifier" {
			return "public"
		}
	}
	return "private"
}

// extractRustReceiverType extracts the receiver type from the first self_parameter.
func extractRustReceiverType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	paramList := node.ChildByFieldName("parameters", lang)
	if paramList == nil {
		return nil
	}
	if paramList.NamedChildCount() == 0 {
		return nil
	}
	first := paramList.NamedChild(0)
	if first == nil || first.Type(lang) != "self_parameter" {
		return nil
	}
	t := first.Text(source)
	return &t
}

// isRustAsync checks whether the function has the async keyword.
func isRustAsync(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "function_modifiers" {
			// Check sub-children for async keyword
			for j := 0; j < int(child.ChildCount()); j++ {
				sub := child.Child(j)
				if sub != nil && !sub.IsNamed() && strings.TrimSpace(sub.Type(lang)) == "async" {
					return true
				}
			}
		}
	}
	return false
}

// extractRustAnnotations extracts attributes from preceding attribute_item sibling nodes.
func extractRustAnnotations(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
	var annotations []string
	sibling := prevNamedSibling(node, lang)
	for sibling != nil {
		if sibling.Type(lang) == "attribute_item" {
			annotations = append([]string{sibling.Text(source)}, annotations...)
		} else {
			break // attributes are contiguous; non-attribute sibling stops search
		}
		sibling = prevNamedSibling(sibling, lang)
	}
	return annotations
}

// RustMethodConfig is the Rust method extraction configuration.
var RustMethodConfig = core.MethodExtractionConfig{
	Language:             core.LangRust,
	TypeDeclarationNodes: []string{"impl_item", "trait_item"},
	MethodNodeTypes:      []string{"function_item", "function_signature_item"},
	BodyNodeTypes:        []string{"declaration_list"},

	ExtractOwnerName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		if node.Type(lang) != "impl_item" {
			return nil
		}
		// impl Trait for Struct — get the type after "for"
		forIdx := -1
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c != nil && !c.IsNamed() && c.Type(lang) == "for" {
				forIdx = i
				break
			}
		}
		if forIdx != -1 {
			for i := forIdx + 1; i < int(node.ChildCount()); i++ {
				c := node.Child(i)
				if c != nil && (c.Type(lang) == "type_identifier" || c.Type(lang) == "scoped_type_identifier") {
					t := c.Text(source)
					return &t
				}
			}
		}
		// Plain impl Struct — get the first type_identifier
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c != nil && c.Type(lang) == "type_identifier" {
				t := c.Text(source)
				return &t
			}
		}
		return nil
	},

	ExtractName:         extractRustMethodName,
	ExtractReturnType:   extractRustReturnType,
	ExtractParameters:   extractRustParameters,
	ExtractVisibility:   extractRustVisibility,
	ExtractAnnotations:  extractRustAnnotations,
	ExtractReceiverType: extractRustReceiverType,

	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		// No self_parameter = associated function (static)
		paramList := node.ChildByFieldName("parameters", lang)
		if paramList == nil {
			return true
		}
		if paramList.NamedChildCount() == 0 {
			return true
		}
		first := paramList.NamedChild(0)
		return first == nil || first.Type(lang) != "self_parameter"
	},

	IsAbstract: func(node *gotreesitter.Node, ownerNode *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		// function_signature_item in a trait is abstract
		return ownerNode.Type(lang) == "trait_item" && node.Type(lang) == "function_signature_item"
	},

	IsFinal: func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool { return false },

	IsAsync: isRustAsync,
}