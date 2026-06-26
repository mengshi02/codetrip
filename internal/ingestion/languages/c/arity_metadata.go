package c

import (
	"github.com/odvcencio/gotreesitter"
)

// CArityInfo holds computed arity metadata for a C function declaration.
type CArityInfo struct {
	ParameterCount         int      // Total parameter count (0 if variadic)
	RequiredParameterCount int      // Required (non-variadic) parameter count
	ParameterTypes         []string // Parameter type names
	HasParameterCount      bool     // Whether ParameterCount is valid (false for K&R style or variadic)
}

// ComputeCDeclarationArity computes arity metadata from a C function definition or declaration node.
// Ported from GitNexus c/arity-metadata.ts.
func ComputeCDeclarationArity(tsLang *gotreesitter.Language, node *gotreesitter.Node, source []byte) CArityInfo {
	funcDecl := findFuncDeclarator(tsLang, node)
	if funcDecl == nil {
		return CArityInfo{}
	}

	paramList := funcDecl.ChildByFieldName("parameters", tsLang)
	if paramList == nil {
		return CArityInfo{}
	}

	// Collect parameter_declaration and variadic_parameter children
	var params []*gotreesitter.Node
	for i := 0; i < int(paramList.ChildCount()); i++ {
		child := paramList.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(tsLang)
		if ct == "parameter_declaration" || ct == "variadic_parameter" {
			params = append(params, child)
		}
	}

	// K&R old-style declaration: `int foo()` has no parameter children.
	// This means unspecified number/types of arguments — return unknown arity.
	if len(params) == 0 {
		return CArityInfo{}
	}

	// (void) means zero parameters
	if len(params) == 1 && params[0].Type(tsLang) == "parameter_declaration" {
		typeNode := params[0].ChildByFieldName("type", tsLang)
		declaratorNode := params[0].ChildByFieldName("declarator", tsLang)
		if typeNode != nil && typeNode.Text(source) == "void" && declaratorNode == nil {
			return CArityInfo{
				ParameterCount:         0,
				RequiredParameterCount: 0,
				ParameterTypes:         []string{},
				HasParameterCount:      true,
			}
		}
	}

	isVariadic := false
	nonVariadicCount := 0
	var types []string

	for _, p := range params {
		if p.Type(tsLang) == "variadic_parameter" {
			isVariadic = true
			types = append(types, "...")
		} else {
			typeNode := p.ChildByFieldName("type", tsLang)
			typeName := "unknown"
			if typeNode != nil {
				typeName = typeNode.Text(source)
			}
			types = append(types, typeName)
			nonVariadicCount++
		}
	}

	result := CArityInfo{
		RequiredParameterCount: nonVariadicCount,
		ParameterTypes:         types,
		HasParameterCount:      true,
	}
	if !isVariadic {
		result.ParameterCount = nonVariadicCount
	}
	// If variadic, ParameterCount is 0 (default) and HasParameterCount is false
	// to indicate we have valid info but the count is open-ended
	if isVariadic {
		result.HasParameterCount = false
	}
	return result
}

// ComputeCCallArity computes the number of arguments in a call_expression node.
// Ported from GitNexus c/arity-metadata.ts.
func ComputeCCallArity(tsLang *gotreesitter.Language, node *gotreesitter.Node, source []byte) int {
	argList := node.ChildByFieldName("arguments", tsLang)
	if argList == nil {
		return 0
	}

	count := 0
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(tsLang)
		// Skip punctuation (commas, parens)
		if ct != "," && ct != "(" && ct != ")" {
			count++
		}
	}
	return count
}

// findFuncDeclarator finds the function_declarator node, unwrapping pointer_declarator wrappers.
func findFuncDeclarator(tsLang *gotreesitter.Language, node *gotreesitter.Node) *gotreesitter.Node {
	decl := node.ChildByFieldName("declarator", tsLang)
	if decl == nil {
		// Try direct child search
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c != nil && c.Type(tsLang) == "function_declarator" {
				return c
			}
		}
		return nil
	}
	// Unwrap pointer_declarator
	for decl.Type(tsLang) == "pointer_declarator" {
		next := decl.ChildByFieldName("declarator", tsLang)
		if next == nil {
			break
		}
		decl = next
	}
	if decl.Type(tsLang) == "function_declarator" {
		return decl
	}
	return nil
}
