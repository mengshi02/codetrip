// Package golang — Go arity metadata: declaration and call arity computation.
// Ported from TS languages/go/arity-metadata.ts.
package golang

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// GoDeclarationArity holds arity metadata extracted from a Go function/method declaration.
type GoDeclarationArity struct {
	ParameterCount        int
	RequiredParameterCount int
	ParameterTypes        []string
	ReturnType            string
}

// ComputeGoDeclarationArityFromNode inspects a function_declaration or
// method_declaration AST node and computes parameter counts, types,
// and the return type.
//
// Mirrors TS computeGoDeclarationArity(node).
func ComputeGoDeclarationArityFromNode(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) GoDeclarationArity {
	result := GoDeclarationArity{}

	if node == nil {
		return result
	}

	// Find the parameter list
	paramsNode := node.ChildByFieldName("parameters", lang)
	if paramsNode == nil {
		return result
	}

	total, required := 0, 0
	var paramTypes []string

	for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
		param := paramsNode.NamedChild(i)
		if param == nil {
			continue
		}
		total++

		// Check if variadic: the type node will be a variadic_type or
		// the param text will contain "..."
		isVariadic := isVariadicParam(lang, param, source)
		if !isVariadic {
			required++
		}

		// Extract parameter type text
		typeNode := param.ChildByFieldName("type", lang)
		if typeNode != nil {
			paramTypes = append(paramTypes, typeNode.Text(source))
		}
	}

	result.ParameterCount = total
	result.RequiredParameterCount = required
	result.ParameterTypes = paramTypes

	// Extract return type
	resultNode := node.ChildByFieldName("result", lang)
	if resultNode != nil {
		result.ReturnType = normalizeGoReturnType(resultNode.Text(source))
	}

	return result
}

// ComputeGoCallArityFromNode counts the number of arguments in a
// call_expression or composite_literal node.
//
// Mirrors TS computeGoCallArity(node).
func ComputeGoCallArityFromNode(lang *gotreesitter.Language, node *gotreesitter.Node) int {
	if node == nil {
		return 0
	}

	// call_expression has an "arguments" field
	argsNode := node.ChildByFieldName("arguments", lang)
	if argsNode == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		child := argsNode.NamedChild(i)
		if child != nil {
			count++
		}
	}
	return count
}

// isVariadicParam checks if a parameter declaration is variadic (...T).
func isVariadicParam(lang *gotreesitter.Language, param *gotreesitter.Node, source []byte) bool {
	// Method 1: check if the type child is a variadic_type node
	typeNode := param.ChildByFieldName("type", lang)
	if typeNode != nil && typeNode.Type(lang) == "variadic_type" {
		return true
	}

	// Method 2: check the text for "..." prefix
	text := param.Text(source)
	return strings.Contains(text, "...")
}

// normalizeGoReturnType normalizes the return type text for Go declarations.
// Removes braces from multi-return signatures and trims whitespace.
func normalizeGoReturnType(text string) string {
	text = strings.TrimSpace(text)
	// Remove outer parentheses for single returns like "(error)" → "error"
	if strings.HasPrefix(text, "(") && strings.HasSuffix(text, ")") {
		inner := text[1 : len(text)-1]
		// Only strip if there's no comma (multi-return stays as-is)
		if !strings.Contains(inner, ",") {
			text = inner
		}
	}
	return text
}

