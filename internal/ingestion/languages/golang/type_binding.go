// Package golang — Go type binding synthesis and extraction.
// Go type bindings come from several sources: receiver self declarations,
// variable type annotations, return type annotations, short variable
// declarations with inferred types, new/make calls, composite literals,
// and type assertions. These functions synthesize CaptureMatch records
// and extract simple type names from raw Go type syntax.
// Ported from TS languages/go/type-binding.ts.
package golang

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/utils"
	"github.com/odvcencio/gotreesitter"
)

// SynthesizeGoTypeBindingsFromRoot walks the root AST node and synthesizes
// @type-binding captures that the S-expression query cannot express:
//   - new(T) → @type-binding.constructor with @type-binding.type = T
//   - make(T, ...) → @type-binding.constructor with @type-binding.type = T
//   - T{...} composite literal → @type-binding.constructor with @type-binding.type = T
//   - short_var_declaration with multi-return → @type-binding.call-return
//   - index_expression with type assertion → @type-binding.assertion
//
// Mirrors TS synthesizeGoTypeBindings(root, source).
func SynthesizeGoTypeBindingsFromRoot(lang *gotreesitter.Language, root *gotreesitter.Node, source []byte) []CaptureMatch {
	var out []CaptureMatch

	walkNamedTree(lang, root, func(node *gotreesitter.Node) {
		if node == nil {
			return
		}

		switch node.Type(lang) {
		case "call_expression":
			synthesizeCallExpressionTypeBinding(lang, node, &out, source)
		case "composite_literal":
			synthesizeCompositeLiteralTypeBinding(lang, node, &out, source)
		case "short_var_declaration":
			synthesizeShortVarDeclTypeBinding(lang, node, &out, source)
		case "type_assertion":
			synthesizeTypeAssertionBinding(lang, node, &out, source)
		}
	})

	return out
}

// synthesizeCallExpressionTypeBinding handles new(T) and make(T, ...) calls.
func synthesizeCallExpressionTypeBinding(lang *gotreesitter.Language, node *gotreesitter.Node, out *[]CaptureMatch, source []byte) {
	funcNode := node.ChildByFieldName("function", lang)
	if funcNode == nil {
		return
	}

	funcName := funcNode.Text(source)
	if funcName != "new" && funcName != "make" {
		return
	}

	argsNode := node.ChildByFieldName("arguments", lang)
	if argsNode == nil || argsNode.NamedChildCount() == 0 {
		return
	}

	// First argument is the type
	firstArg := argsNode.NamedChild(0)
	if firstArg == nil {
		return
	}

	typeName := ExtractSimpleTypeNameTextFromNode(lang, firstArg, source)
	if typeName == "" {
		return
	}

	*out = append(*out, CaptureMatch{
		"@type-binding.constructor": utils.SyntheticCapture("@type-binding.constructor", node, node.Text(source)),
		"@type-binding.type":        utils.SyntheticCapture("@type-binding.type", firstArg, typeName),
	})
}

// synthesizeCompositeLiteralTypeBinding handles T{...} composite literals.
func synthesizeCompositeLiteralTypeBinding(lang *gotreesitter.Language, node *gotreesitter.Node, out *[]CaptureMatch, source []byte) {
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return
	}

	typeName := ExtractSimpleTypeNameTextFromNode(lang, typeNode, source)
	if typeName == "" {
		return
	}

	*out = append(*out, CaptureMatch{
		"@type-binding.constructor": utils.SyntheticCapture("@type-binding.constructor", node, node.Text(source)),
		"@type-binding.type":        utils.SyntheticCapture("@type-binding.type", typeNode, typeName),
	})
}

// synthesizeShortVarDeclTypeBinding handles short variable declarations where
// the RHS is a call that returns multiple values: x, y := fn()
// These produce @type-binding.call-return captures.
func synthesizeShortVarDeclTypeBinding(lang *gotreesitter.Language, node *gotreesitter.Node, out *[]CaptureMatch, source []byte) {
	lhs := node.ChildByFieldName("left", lang)
	rhs := node.ChildByFieldName("right", lang)
	if lhs == nil || rhs == nil {
		return
	}

	// Only synthesize for multi-value returns
	rhsCount := countNamedChildrenOfSpecificTypes(lang, rhs, "call_expression", "type_assertion", "index_expression")
	if rhsCount < 2 {
		return
	}

	lhsIdents := collectIdentifiers(lang, lhs)
	if len(lhsIdents) == 0 {
		return
	}

	// Create one @type-binding.call-return per LHS identifier
	for _, ident := range lhsIdents {
		*out = append(*out, CaptureMatch{
			"@type-binding.call-return": utils.SyntheticCapture("@type-binding.call-return", ident, ident.Text(source)),
			"@type-binding.name":        utils.SyntheticCapture("@type-binding.name", ident, ident.Text(source)),
		})
	}
}

// synthesizeTypeAssertionBinding handles type assertions: x.(T)
func synthesizeTypeAssertionBinding(lang *gotreesitter.Language, node *gotreesitter.Node, out *[]CaptureMatch, source []byte) {
	// type_assertion has an operand and a type field
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return
	}

	typeName := ExtractSimpleTypeNameTextFromNode(lang, typeNode, source)
	if typeName == "" {
		return
	}

	*out = append(*out, CaptureMatch{
		"@type-binding.assertion": utils.SyntheticCapture("@type-binding.assertion", node, node.Text(source)),
		"@type-binding.type":      utils.SyntheticCapture("@type-binding.type", typeNode, typeName),
	})
}

// ExtractSimpleTypeNameTextFromNode extracts the simple type name from a Go AST node.
// Handles: type_identifier, qualified_type, generic_type, pointer_type, array_type,
// slice_type, map_type, chan_type, interface_type, struct_type, function_type.
//
// Mirrors TS extractSimpleTypeNameText(node).
func ExtractSimpleTypeNameTextFromNode(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) string {
	if node == nil {
		return ""
	}

	switch node.Type(lang) {
	case "type_identifier":
		return node.Text(source)

	case "qualified_type":
		// pkg.Type → extract Type (the name field)
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode != nil {
			return nameNode.Text(source)
		}
		// Fallback: take text after last dot
		text := node.Text(source)
		if idx := strings.LastIndex(text, "."); idx >= 0 {
			return text[idx+1:]
		}
		return text

	case "generic_type":
		// Type[T] → extract Type (the type field)
		inner := node.ChildByFieldName("type", lang)
		if inner != nil {
			return ExtractSimpleTypeNameTextFromNode(lang, inner, source)
		}
		return ""

	case "pointer_type":
		// *Type → extract Type
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil {
				return ExtractSimpleTypeNameTextFromNode(lang, child, source)
			}
		}
		return ""

	case "slice_type", "array_type":
		// []Type or [N]Type → extract the element type
		elemNode := node.ChildByFieldName("element", lang)
		if elemNode != nil {
			return ExtractSimpleTypeNameTextFromNode(lang, elemNode, source)
		}
		return ""

	case "map_type":
		// map[K]V → extract V (the value type)
		valueNode := node.ChildByFieldName("value", lang)
		if valueNode != nil {
			return ExtractSimpleTypeNameTextFromNode(lang, valueNode, source)
		}
		return ""

	case "chan_type":
		// chan Type → extract the element type
		valueNode := node.ChildByFieldName("value", lang)
		if valueNode != nil {
			return ExtractSimpleTypeNameTextFromNode(lang, valueNode, source)
		}
		// Fallback: last named child
		for i := int(node.NamedChildCount()) - 1; i >= 0; i-- {
			child := node.NamedChild(i)
			if child != nil {
				return ExtractSimpleTypeNameTextFromNode(lang, child, source)
			}
		}
		return ""

	case "interface_type", "struct_type", "function_type":
		return ""

	default:
		// For unknown types, try to extract a reasonable name
		text := node.Text(source)
		return NormalizeGoTypeName(text)
	}
}

// collectIdentifiers collects all identifier nodes from a expression_list or
// similar container node.
func collectIdentifiers(lang *gotreesitter.Language, node *gotreesitter.Node) []*gotreesitter.Node {
	var result []*gotreesitter.Node
	if node == nil {
		return result
	}

	if node.Type(lang) == "identifier" {
		return []*gotreesitter.Node{node}
	}

	// For expression_list, walk named children
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "identifier" {
			result = append(result, child)
		} else {
			result = append(result, collectIdentifiers(lang, child)...)
		}
	}
	return result
}

// countNamedChildrenOfSpecificTypes counts named children that match any of the given types.
func countNamedChildrenOfSpecificTypes(lang *gotreesitter.Language, node *gotreesitter.Node, types ...string) int {
	if node == nil {
		return 0
	}
	count := 0
	typeSet := make(map[string]bool, len(types))
	for _, t := range types {
		typeSet[t] = true
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && typeSet[child.Type(lang)] {
			count++
		}
	}
	return count
}