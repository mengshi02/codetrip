package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	"github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
	"github.com/mengshi02/codetrip/internal/ingestion/languages/cpp"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// ---------------------------------------------------------------------------
// C/C++ helpers
// ---------------------------------------------------------------------------

// findFunctionDeclarator finds the function_declarator inside a method node,
// handling pointer/reference return type wrapping.
func findFunctionDeclarator(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	declarator := node.ChildByFieldName("declarator", lang)
	if declarator == nil {
		return nil
	}
	if declarator.Type(lang) == "function_declarator" {
		return declarator
	}
	// Recursively unwrap pointer_declarator / reference_declarator chains
	current := declarator
	for current != nil {
		for i := 0; i < int(current.NamedChildCount()); i++ {
			child := current.NamedChild(i)
			if child != nil && child.Type(lang) == "function_declarator" {
				return child
			}
		}
		var next *gotreesitter.Node
		for i := 0; i < int(current.NamedChildCount()); i++ {
			child := current.NamedChild(i)
			if child != nil && (child.Type(lang) == "pointer_declarator" || child.Type(lang) == "reference_declarator") {
				next = child
				break
			}
		}
		current = next
	}
	return nil
}

// hasSpecialMethodClause checks for C++ special member clauses (e.g. delete_method_clause).
func hasSpecialMethodClause(node *gotreesitter.Node, clauseType string, lang *gotreesitter.Language) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == clauseType {
			return true
		}
	}
	return false
}

// extractCppMethodName extracts the method name from a function_declarator.
// The name is the declarator field of the function_declarator — typically a
// field_identifier, but can also be destructor_name or operator_name.
func extractCppMethodName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	funcDecl := findFunctionDeclarator(node, lang)
	if funcDecl == nil {
		return nil
	}
	nameNode := funcDecl.ChildByFieldName("declarator", lang)
	if nameNode == nil {
		return nil
	}
	t := nameNode.Text(source)
	return &t
}

// extractCppReturnType extracts the return type from the method node's type field.
// Handles C++11 trailing return type: auto foo() -> ReturnType.
func extractCppReturnType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode != nil {
		typeText := strings.TrimSpace(typeNode.Text(source))
		// C++11 trailing return type
		if typeText == "auto" {
			funcDecl := findFunctionDeclarator(node, lang)
			if funcDecl != nil {
				for i := 0; i < int(funcDecl.NamedChildCount()); i++ {
					child := funcDecl.NamedChild(i)
					if child != nil && child.Type(lang) == "trailing_return_type" {
						typeDesc := firstNamedChild(child, lang)
						if typeDesc != nil {
							t := strings.TrimSpace(typeDesc.Text(source))
							return &t
						}
					}
				}
			}
		}
		return &typeText
	}
	// Fallback: first type-like named child
	first := firstNamedChild(node, lang)
	if first != nil {
		switch first.Type(lang) {
		case "primitive_type", "type_identifier", "sized_type_specifier", "template_type":
			t := strings.TrimSpace(first.Text(source))
			return &t
		}
	}
	return nil
}

// extractCppParameters extracts parameters from the function_declarator's parameter_list.
// C/C++ uses parameter_declaration / optional_parameter_declaration /
// variadic_parameter_declaration / variadic_parameter (bare ...).
func extractCppParameters(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []core.ParameterInfo {
	funcDecl := findFunctionDeclarator(node, lang)
	if funcDecl == nil {
		return nil
	}
	paramList := funcDecl.ChildByFieldName("parameters", lang)
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
			declNode := param.ChildByFieldName("declarator", lang)
			name := extractParamName(declNode, source, lang)
			var nameStr string
			if name != nil {
				nameStr = *name
			} else if typeNode != nil {
				nameStr = strings.TrimSpace(typeNode.Text(source))
			} else {
				nameStr = "?"
			}
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
				Name:      nameStr,
				Type:      typ,
				RawType:   rawType,
				TypeClass: classifyCppParam(typeNode, declNode, param, source, lang),
			})

		case "optional_parameter_declaration":
			typeNode := param.ChildByFieldName("type", lang)
			declNode := param.ChildByFieldName("declarator", lang)
			name := extractParamName(declNode, source, lang)
			var nameStr string
			if name != nil {
				nameStr = *name
			} else if typeNode != nil {
				nameStr = strings.TrimSpace(typeNode.Text(source))
			} else {
				nameStr = "?"
			}
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
				Name:       nameStr,
				Type:       typ,
				RawType:    rawType,
				TypeClass:  classifyCppParam(typeNode, declNode, param, source, lang),
				IsOptional: true,
			})

		case "variadic_parameter_declaration":
			typeNode := param.ChildByFieldName("type", lang)
			declNode := param.ChildByFieldName("declarator", lang)
			name := extractParamName(declNode, source, lang)
			var nameStr string
			if name != nil {
				nameStr = *name
			} else {
				nameStr = "..."
			}
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
				Name:       nameStr,
				Type:       typ,
				RawType:    rawType,
				TypeClass:  classifyCppParam(typeNode, declNode, param, source, lang),
				IsVariadic: true,
			})

		case "variadic_parameter":
			// Bare `...` (C-style)
			params = append(params, core.ParameterInfo{
				Name:       "...",
				IsVariadic: true,
			})
		}
	}

	// Bare `...` token in parameter_list is an unnamed child
	hasVariadic := false
	for _, p := range params {
		if p.IsVariadic {
			hasVariadic = true
			break
		}
	}
	if !hasVariadic {
		for i := 0; i < int(paramList.ChildCount()); i++ {
			child := paramList.Child(i)
			if child != nil && !child.IsNamed() && child.Text(source) == "..." {
				params = append(params, core.ParameterInfo{
					Name:       "...",
					IsVariadic: true,
				})
				break
			}
		}
	}

	return params
}

// extractParamName recursively unwraps pointer/reference declarators to extract the parameter name.
func extractParamName(declNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	if declNode == nil {
		return nil
	}
	if declNode.Type(lang) == "identifier" {
		t := declNode.Text(source)
		return &t
	}
	for i := 0; i < int(declNode.NamedChildCount()); i++ {
		child := declNode.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "identifier" {
			t := child.Text(source)
			return &t
		}
		if child.Type(lang) == "pointer_declarator" || child.Type(lang) == "reference_declarator" {
			return extractParamName(child, source, lang)
		}
	}
	return nil
}

// classifyCppParam wraps cpp.ClassifyCppParameterType for Go method extractors.
func classifyCppParam(typeNode, declNode, paramNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *core.ParameterTypeClass {
	var typeText, declText, paramText string
	if typeNode != nil {
		typeText = strings.TrimSpace(typeNode.Text(source))
	} else {
		typeText = "unknown"
	}
	if declNode != nil {
		declText = declNode.Text(source)
	}
	if paramNode != nil {
		paramText = paramNode.Text(source)
	}
	tc := cpp.ClassifyCppParameterType(typeText, declText, paramText)
	return &tc
}

// extractCppVisibility detects C++ access specifiers by walking backward through siblings.
func extractCppVisibility(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.MethodVisibility {
	// If node is unwrapped from template_declaration, access specifier is template_declaration's sibling
	startNode := node
	if parent := node.Parent(); parent != nil && parent.Type(lang) == "template_declaration" {
		startNode = parent
	}

	sibling := prevNamedSibling(startNode, lang)
	for sibling != nil {
		if sibling.Type(lang) == "access_specifier" {
			text := strings.ReplaceAll(sibling.Text(source), ":", "")
			text = strings.TrimSpace(text)
			if text == "public" || text == "private" || text == "protected" {
				return core.MethodVisibility(text)
			}
		}
		sibling = prevNamedSibling(sibling, lang)
	}
	// Default: struct/union = public, class = private
	parent := startNode.Parent()
	if parent != nil {
		grandParent := parent.Parent()
		if grandParent != nil && (grandParent.Type(lang) == "struct_specifier" || grandParent.Type(lang) == "union_specifier") {
			return "public"
		}
	}
	return "private"
}

// isPureVirtual detects pure virtual methods (= 0).
func isPureVirtual(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
	foundEquals := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Text(source) == "=" {
			foundEquals = true
		} else if foundEquals && child.Type(lang) == "number_literal" && child.Text(source) == "0" {
			return true
		} else if foundEquals {
			foundEquals = false
		}
	}
	return false
}

// hasVirtualSpecifier detects virtual_specifier (final/override) inside function_declarator.
func hasVirtualSpecifier(node *gotreesitter.Node, keyword string, source []byte, lang *gotreesitter.Language) bool {
	funcDecl := findFunctionDeclarator(node, lang)
	if funcDecl == nil {
		return false
	}
	for i := 0; i < int(funcDecl.NamedChildCount()); i++ {
		child := funcDecl.NamedChild(i)
		if child != nil && child.Type(lang) == "virtual_specifier" && child.Text(source) == keyword {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// C++ config
// ---------------------------------------------------------------------------

// CppMethodConfig is the C++ method extraction configuration.
// C++ methods appear as field_declaration (declaration) or function_definition
// (inline definition) inside field_declaration_list. There is no dedicated
// method_declaration node — a field_declaration containing a function_declarator is a method.
var CppMethodConfig = core.MethodExtractionConfig{
	Language:             core.LangCpp,
	TypeDeclarationNodes: []string{"class_specifier", "struct_specifier", "union_specifier"},
	MethodNodeTypes:      []string{"field_declaration", "function_definition", "declaration"},
	BodyNodeTypes:        []string{"field_declaration_list"},

	ExtractName:       extractCppMethodName,
	ExtractReturnType: extractCppReturnType,
	ExtractParameters: extractCppParameters,
	ExtractVisibility: extractCppVisibility,

	IsStatic: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "static", lang)
	},
	IsAbstract: func(node, _ *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		return isPureVirtual(node, source, lang)
	},
	IsFinal: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		return hasVirtualSpecifier(node, "final", source, lang)
	},
	IsVirtual: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "virtual", lang) ||
			hasVirtualSpecifier(node, "override", source, lang) ||
			hasVirtualSpecifier(node, "final", source, lang)
	},
	IsOverride: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		return hasVirtualSpecifier(node, "override", source, lang)
	},
	IsConst: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		funcDecl := findFunctionDeclarator(node, lang)
		if funcDecl == nil {
			return false
		}
		for i := 0; i < int(funcDecl.NamedChildCount()); i++ {
			child := funcDecl.NamedChild(i)
			if child != nil && child.Type(lang) == "type_qualifier" && child.Text(source) == "const" {
				return true
			}
		}
		return false
	},
	IsDeleted: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		return hasSpecialMethodClause(node, "delete_method_clause", lang)
	},
}

// ---------------------------------------------------------------------------
// C config (minimal — C has no OOP methods)
// ---------------------------------------------------------------------------

// CMethodConfig is the C method extraction configuration.
// C has no OOP methods; function pointers in structs are handled by field extractors.
var CMethodConfig = core.MethodExtractionConfig{
	Language:             core.LangC,
	TypeDeclarationNodes: []string{"struct_specifier"},
	MethodNodeTypes:      []string{"function_definition"},
	BodyNodeTypes:        []string{"field_declaration_list"},

	ExtractName:       extractCppMethodName,
	ExtractReturnType: extractCppReturnType,
	ExtractParameters: extractCppParameters,
	ExtractVisibility: func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) core.MethodVisibility {
		return "public"
	},
	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return configs.HasKeyword(node, "static", lang)
	},
	IsAbstract: func(_, _ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool { return false },
	IsFinal:    func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool { return false },
}
