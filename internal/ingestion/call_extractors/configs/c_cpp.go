package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// CCallConfig is the call extraction configuration for C.
var CCallConfig = core.CallExtractionConfig{
	Language: core.LangC,
}

// CppCallConfig is the call extraction configuration for C++.
// Handles operator overload calls.
var CppCallConfig = core.CallExtractionConfig{
	Language:                core.LangCpp,
	ExtractLanguageCallSite: extractCppOperatorCallSite,
}

// extractCppOperatorCallSite extracts C++ operator overload call sites.
func extractCppOperatorCallSite(callNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *core.ExtractedCallSite {
	if callNode.Type(lang) != "binary_expression" {
		return nil
	}
	if isPrimitiveOnlyBinaryOperatorCall(callNode, source, lang) {
		return nil
	}

	operator := ""
	if op := callNode.ChildByFieldName("operator", lang); op != nil {
		operator = strings.TrimSpace(op.Text(source))
	}

	// Keep DAG conservative: only model simple identifier operands.
	// Complex expressions remain unresolved rather than guessed.
	if operator == "+" {
		left := callNode.ChildByFieldName("left", lang)
		right := callNode.ChildByFieldName("right", lang)
		if left == nil || right == nil || left.Type(lang) != "identifier" || right.Type(lang) != "identifier" {
			return nil
		}
		leftText := left.Text(source)
		member := core.CallFormMember
		one := 1
		return &core.ExtractedCallSite{
			CalledName:   "operator+",
			CallForm:     &member,
			ReceiverName: &leftText,
			ArgCount:     &one,
		}
	}

	if operator == "<<" {
		right := callNode.ChildByFieldName("right", lang)
		if right == nil || right.Type(lang) != "identifier" {
			return nil
		}
		free := core.CallFormFree
		two := 2
		return &core.ExtractedCallSite{
			CalledName: "operator<<",
			CallForm:   &free,
			ArgCount:   &two,
		}
	}

	return nil
}

// isPrimitiveOnlyBinaryOperatorCall checks whether both operands of a binary
// operator call are built-in types.
func isPrimitiveOnlyBinaryOperatorCall(callNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
	left := callNode.ChildByFieldName("left", lang)
	right := callNode.ChildByFieldName("right", lang)
	if left == nil || right == nil {
		return false
	}
	return isBuiltinOperatorOperand(left, source, lang) && isBuiltinOperatorOperand(right, source, lang)
}

func isBuiltinOperatorOperand(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
	return isBuiltinOperatorType(inferCppOperatorOperandType(node, source, lang))
}

func inferCppOperatorOperandType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	if lt := inferCppLiteralType(node, source, lang); lt != "" {
		return lt
	}
	if node.Type(lang) == "identifier" {
		return lookupCppIdentifierType(node, source, lang)
	}
	return ""
}

func inferCppLiteralType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	switch node.Type(lang) {
	case "number_literal":
		if strings.Contains(node.Text(source), ".") {
			return "double"
		}
		return "int"
	case "char_literal":
		return "char"
	case "true", "false":
		return "bool"
	default:
		return ""
	}
}

// lookupCppIdentifierType walks up the scope to find the type declaration for an identifier.
func lookupCppIdentifierType(identNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	varName := identNode.Text(source)
	scope := identNode.Parent()
	for scope != nil && scope.Type(lang) != "compound_statement" && scope.Type(lang) != "translation_unit" {
		scope = scope.Parent()
	}
	if scope == nil {
		return ""
	}

	// Check function parameters first
	if pt := lookupCppFunctionParameterType(scope, varName, source, lang); pt != "" {
		return pt
	}

	// Check local declarations
	for i := 0; i < scope.ChildCount(); i++ {
		stmt := scope.Child(i)
		if stmt == nil || stmt.Type(lang) != "declaration" {
			continue
		}
		typeNode := stmt.ChildByFieldName("type", lang)
		declarator := stmt.ChildByFieldName("declarator", lang)
		if typeNode == nil || declarator == nil {
			continue
		}
		if extractDeclaratorLeafName(declarator, source, lang) == varName {
			return normalizeCppTypeText(typeNode.Text(source))
		}
	}
	return ""
}

// lookupCppFunctionParameterType searches upward for function definition parameter types.
func lookupCppFunctionParameterType(scope *gotreesitter.Node, varName string, source []byte, lang *gotreesitter.Language) string {
	node := scope.Parent()
	for node != nil {
		if node.Type(lang) == "function_definition" || node.Type(lang) == "function_declarator" {
			var fnDecl *gotreesitter.Node
			if node.Type(lang) == "function_declarator" {
				fnDecl = node
			} else {
				fnDecl = findFirstDescendantOfType(node, lang, "function_declarator")
			}
			if fnDecl == nil {
				return ""
			}
			params := fnDecl.ChildByFieldName("parameters", lang)
			if params == nil {
				return ""
			}
			for i := 0; i < params.NamedChildCount(); i++ {
				param := params.NamedChild(i)
				if param == nil || param.Type(lang) != "parameter_declaration" {
					continue
				}
				declarator := param.ChildByFieldName("declarator", lang)
				typeNode := param.ChildByFieldName("type", lang)
				if declarator != nil && typeNode != nil && extractDeclaratorLeafName(declarator, source, lang) == varName {
					return normalizeCppTypeText(typeNode.Text(source))
				}
			}
			return ""
		}
		node = node.Parent()
	}
	return ""
}

// findFirstDescendantOfType DFS-searches for a descendant node of the given type.
func findFirstDescendantOfType(node *gotreesitter.Node, lang *gotreesitter.Language, typ string) *gotreesitter.Node {
	if node.Type(lang) == typ {
		return node
	}
	for i := 0; i < node.NamedChildCount(); i++ {
		if found := findFirstDescendantOfType(node.NamedChild(i), lang, typ); found != nil {
			return found
		}
	}
	return nil
}

// extractDeclaratorLeafName extracts the leaf-level name from a declarator node.
func extractDeclaratorLeafName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	switch node.Type(lang) {
	case "identifier", "field_identifier", "operator_name":
		return node.Text(source)
	}
	// Search named children from back to front
	n := node.NamedChildCount()
	for i := int(n) - 1; i >= 0; i-- {
		if name := extractDeclaratorLeafName(node.NamedChild(i), source, lang); name != "" {
			return name
		}
	}
	return ""
}

// normalizeCppTypeText strips CV qualifiers from C++ type text.
func normalizeCppTypeText(text string) string {
	s := text
	for _, kw := range []string{"const", "volatile", "static", "extern", "register", "mutable", "inline", "constexpr"} {
		s = strings.ReplaceAll(s, kw, " ")
	}
	return strings.Join(strings.Fields(s), " ")
}

func isBuiltinOperatorType(typ string) bool {
	switch typ {
	case "bool", "char", "double", "float", "int", "long", "short", "signed", "unsigned":
		return true
	default:
		return false
	}
}
