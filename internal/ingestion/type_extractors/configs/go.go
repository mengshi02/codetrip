// Package configs provides per-language type extraction configurations.
// This file implements the Go language type extractor configuration,
// ported from TS type-extractors/go.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// Go function/method node types that carry a parameter list.
var goFunctionNodeTypes = map[string]bool{
	"function_declaration": true,
	"method_declaration":   true,
	"func_literal":         true,
}

// ---------------------------------------------------------------------------
// extractGoVarDeclaration — Go: var x Foo
// ---------------------------------------------------------------------------

func extractGoVarDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	// Go var_declaration contains var_spec children
	if node.Type(lang) == "var_declaration" {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			spec := node.NamedChild(i)
			if spec != nil && spec.Type(lang) == "var_spec" {
				extractGoVarDeclaration(spec, source, lang, env)
			}
		}
		return
	}

	// var_spec: name type [= value]
	nameNode := node.ChildByFieldName("name", lang)
	typeNode := node.ChildByFieldName("type", lang)
	if nameNode == nil || typeNode == nil {
		return
	}
	varName := typeextractors.ExtractVarName(nameNode, source, lang)
	typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
	if varName != nil && typeName != nil {
		env[*varName] = *typeName
	}
}

// ---------------------------------------------------------------------------
// extractGoShortVarDeclaration — Go: x := Foo{...}
// ---------------------------------------------------------------------------

func extractGoShortVarDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	left := node.ChildByFieldName("left", lang)
	right := node.ChildByFieldName("right", lang)
	if left == nil || right == nil {
		return
	}

	// Collect LHS names and RHS values (may be expression_lists for multi-assignment)
	var lhsNodes []*gotreesitter.Node
	var rhsNodes []*gotreesitter.Node

	if left.Type(lang) == "expression_list" {
		for i := 0; i < int(left.NamedChildCount()); i++ {
			c := left.NamedChild(i)
			if c != nil {
				lhsNodes = append(lhsNodes, c)
			}
		}
	} else {
		lhsNodes = append(lhsNodes, left)
	}

	if right.Type(lang) == "expression_list" {
		for i := 0; i < int(right.NamedChildCount()); i++ {
			c := right.NamedChild(i)
			if c != nil {
				rhsNodes = append(rhsNodes, c)
			}
		}
	} else {
		rhsNodes = append(rhsNodes, right)
	}

	// Pair each LHS name with its corresponding RHS value
	count := len(lhsNodes)
	if len(rhsNodes) < count {
		count = len(rhsNodes)
	}
	for i := 0; i < count; i++ {
		valueNode := rhsNodes[i]
		// Unwrap &User{} — unary_expression (address-of) wrapping composite_literal
		if valueNode.Type(lang) == "unary_expression" {
			inner := typeextractors.FirstNamedChild(valueNode, lang)
			if inner != nil && inner.Type(lang) == "composite_literal" {
				valueNode = inner
			}
		}
		// Go built-in new(User)
		if valueNode.Type(lang) == "call_expression" {
			funcNode := valueNode.ChildByFieldName("function", lang)
			funcText := ""
			if funcNode != nil {
				funcText = strings.TrimSpace(string(funcNode.Text(source)))
			}
			if funcText == "new" {
				args := valueNode.ChildByFieldName("arguments", lang)
				if args != nil {
					firstArg := typeextractors.FirstNamedChild(args, lang)
					if firstArg != nil {
						typeName := typeextractors.ExtractSimpleTypeNameFromNode(firstArg, source, lang, 0)
						varName := typeextractors.ExtractVarName(lhsNodes[i], source, lang)
						if varName != nil && typeName != nil {
							env[*varName] = *typeName
						}
					}
				}
				continue
			}
			// Go built-in make([]User, 0) / make(map[string]User)
			if funcText == "make" {
				args := valueNode.ChildByFieldName("arguments", lang)
				firstArg := typeextractors.FirstNamedChild(args, lang)
				if firstArg != nil {
					var innerType *gotreesitter.Node
					if firstArg.Type(lang) == "slice_type" {
						innerType = firstArg.ChildByFieldName("element", lang)
					} else if firstArg.Type(lang) == "map_type" {
						innerType = firstArg.ChildByFieldName("value", lang)
					}
					if innerType != nil {
						typeName := typeextractors.ExtractSimpleTypeNameFromNode(innerType, source, lang, 0)
						varName := typeextractors.ExtractVarName(lhsNodes[i], source, lang)
						if varName != nil && typeName != nil {
							env[*varName] = *typeName
						}
					}
				}
				continue
			}
		}
		// Go type assertion: user := iface.(User)
		if valueNode.Type(lang) == "type_assertion_expression" {
			typeNd := valueNode.ChildByFieldName("type", lang)
			if typeNd != nil {
				typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNd, source, lang, 0)
				varName := typeextractors.ExtractVarName(lhsNodes[i], source, lang)
				if varName != nil && typeName != nil {
					env[*varName] = *typeName
				}
			}
			continue
		}
		if valueNode.Type(lang) != "composite_literal" {
			continue
		}
		typeNd := valueNode.ChildByFieldName("type", lang)
		if typeNd == nil {
			continue
		}
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNd, source, lang, 0)
		if typeName == nil {
			continue
		}
		varName := typeextractors.ExtractVarName(lhsNodes[i], source, lang)
		if varName != nil {
			env[*varName] = *typeName
		}
	}
}

// ---------------------------------------------------------------------------
// extractDeclaration — Go
// ---------------------------------------------------------------------------

func goExtractDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	ct := node.Type(lang)
	if ct == "var_declaration" || ct == "var_spec" {
		extractGoVarDeclaration(node, source, lang, env)
	} else if ct == "short_var_declaration" {
		extractGoShortVarDeclaration(node, source, lang, env)
	}
}

// ---------------------------------------------------------------------------
// extractParameter — Go
// ---------------------------------------------------------------------------

func goExtractParameter(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	var nameNode *gotreesitter.Node
	var typeNode *gotreesitter.Node

	if node.Type(lang) == "parameter" {
		nameNode = node.ChildByFieldName("name", lang)
		typeNode = node.ChildByFieldName("type", lang)
	} else {
		nameNode = node.ChildByFieldName("name", lang)
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern", lang)
		}
		typeNode = node.ChildByFieldName("type", lang)
	}

	if nameNode == nil || typeNode == nil {
		return
	}
	varName := typeextractors.ExtractVarName(nameNode, source, lang)
	typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
	if varName != nil && typeName != nil {
		env[*varName] = *typeName
	}
}

// ---------------------------------------------------------------------------
// scanConstructorBinding — Go: user := NewUser(...)
// ---------------------------------------------------------------------------

func goScanConstructorBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *typeextractors.ConstructorBindingResult {
	if node.Type(lang) != "short_var_declaration" {
		return nil
	}
	left := node.ChildByFieldName("left", lang)
	right := node.ChildByFieldName("right", lang)
	if left == nil || right == nil {
		return nil
	}

	var leftIds []*gotreesitter.Node
	var rightExprs []*gotreesitter.Node

	if left.Type(lang) == "expression_list" {
		for i := 0; i < int(left.NamedChildCount()); i++ {
			c := left.NamedChild(i)
			if c != nil {
				leftIds = append(leftIds, c)
			}
		}
	} else {
		leftIds = append(leftIds, left)
	}

	if right.Type(lang) == "expression_list" {
		for i := 0; i < int(right.NamedChildCount()); i++ {
			c := right.NamedChild(i)
			if c != nil {
				rightExprs = append(rightExprs, c)
			}
		}
	} else {
		rightExprs = append(rightExprs, right)
	}

	// Multi-return: user, err := NewUser() — bind first var when second is err/ok/_/error
	if len(leftIds) == 2 && len(rightExprs) == 1 {
		secondVar := leftIds[1]
		secondText := strings.TrimSpace(string(secondVar.Text(source)))
		isErrorOrDiscard := secondText == "_" || secondText == "err" || secondText == "ok" || secondText == "error"
		if isErrorOrDiscard && leftIds[0].Type(lang) == "identifier" {
			if rightExprs[0].Type(lang) != "call_expression" {
				return nil
			}
			fn := rightExprs[0].ChildByFieldName("function", lang)
			if fn == nil {
				return nil
			}
			fnText := strings.TrimSpace(string(fn.Text(source)))
			if fnText == "new" || fnText == "make" {
				return nil
			}
			calleeName := typeextractors.ExtractSimpleTypeNameFromNode(fn, source, lang, 0)
			if calleeName == nil {
				return nil
			}
			return &typeextractors.ConstructorBindingResult{
				VarName:    strings.TrimSpace(string(leftIds[0].Text(source))),
				CalleeName: *calleeName,
			}
		}
	}

	// Single assignment only
	if len(leftIds) != 1 || leftIds[0].Type(lang) != "identifier" {
		return nil
	}
	if len(rightExprs) != 1 || rightExprs[0].Type(lang) != "call_expression" {
		return nil
	}
	fn := rightExprs[0].ChildByFieldName("function", lang)
	if fn == nil {
		return nil
	}
	fnText := strings.TrimSpace(string(fn.Text(source)))
	if fnText == "new" || fnText == "make" {
		return nil
	}
	calleeName := typeextractors.ExtractSimpleTypeNameFromNode(fn, source, lang, 0)
	if calleeName == nil {
		return nil
	}
	return &typeextractors.ConstructorBindingResult{
		VarName:    strings.TrimSpace(string(leftIds[0].Text(source))),
		CalleeName: *calleeName,
	}
}

// ---------------------------------------------------------------------------
// Go for-loop element type helpers
// ---------------------------------------------------------------------------

// extractGoElementTypeFromTypeNode extracts element type from a Go type annotation AST node.
func extractGoElementTypeFromTypeNode(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition, depth int) *string {
	if depth > 50 || typeNode == nil {
		return nil
	}
	// slice_type: []User / array_type: [10]User
	if typeNode.Type(lang) == "slice_type" || typeNode.Type(lang) == "array_type" {
		elemNode := typeNode.ChildByFieldName("element", lang)
		if elemNode != nil {
			return typeextractors.ExtractSimpleTypeNameFromNode(elemNode, source, lang, 0)
		}
	}
	// map_type: map[string]User
	if typeNode.Type(lang) == "map_type" {
		valueNode := typeNode.ChildByFieldName("value", lang)
		if valueNode != nil {
			return typeextractors.ExtractSimpleTypeNameFromNode(valueNode, source, lang, 0)
		}
	}
	// channel_type: chan User
	if typeNode.Type(lang) == "channel_type" {
		valueNode := typeNode.ChildByFieldName("value", lang)
		if valueNode == nil {
			valueNode = typeextractors.LastNamedChild(typeNode, lang)
		}
		if valueNode != nil {
			return typeextractors.ExtractSimpleTypeNameFromNode(valueNode, source, lang, 0)
		}
	}
	// generic_type: Go 1.18+ generics (e.g., MySlice[User], Cache[string, User])
	if typeNode.Type(lang) == "generic_type" {
		args := typeextractors.ExtractGenericTypeArgs(typeNode, source, lang, 0)
		if len(args) >= 1 {
			if pos == typeextractors.TypeArgFirst {
				return &args[0]
			}
			return &args[len(args)-1]
		}
	}
	// Fallback: text-based extraction
	typeText := strings.TrimSpace(string(typeNode.Text(source)))
	return typeextractors.ExtractElementTypeFromString(typeText, pos)
}

// isGoChannelType is a simplified channel type check without AST node access.
func isGoChannelType(iterableName string, scopeEnv map[string]string) bool {
	t, ok := scopeEnv[iterableName]
	return ok && strings.HasPrefix(t, "chan ")
}

// findGoParamElementType walks up from a for-statement to find the enclosing function's parameter.
func findGoParamElementType(iterableName string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	current := startNode.Parent()
	for current != nil {
		if goFunctionNodeTypes[current.Type(lang)] {
			paramsNode := current.ChildByFieldName("parameters", lang)
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
					paramDecl := paramsNode.NamedChild(i)
					if paramDecl == nil || paramDecl.Type(lang) != "parameter_declaration" {
						continue
					}
					nameNode := paramDecl.ChildByFieldName("name", lang)
					if nameNode == nil || strings.TrimSpace(string(nameNode.Text(source))) != iterableName {
						continue
					}
					typeNode := paramDecl.ChildByFieldName("type", lang)
					if typeNode != nil {
						return extractGoElementTypeFromTypeNode(typeNode, source, lang, pos, 0)
					}
				}
			}
			break
		}
		current = current.Parent()
	}
	return nil
}

// ---------------------------------------------------------------------------
// extractForLoopBinding — Go: for _, user := range users
// ---------------------------------------------------------------------------

func goExtractForLoopBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *typeextractors.ForLoopExtractorContext) {
	if node.Type(lang) != "for_statement" {
		return
	}

	// Find the range_clause child
	var rangeClause *gotreesitter.Node
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "range_clause" {
			rangeClause = child
			break
		}
	}
	if rangeClause == nil {
		return
	}

	// The iterable is the `right` field of the range_clause.
	rightNode := rangeClause.ChildByFieldName("right", lang)
	var iterableName *string
	var callExprElementType *string

	if rightNode != nil {
		if rightNode.Type(lang) == "identifier" {
			t := strings.TrimSpace(string(rightNode.Text(source)))
			iterableName = &t
		} else if rightNode.Type(lang) == "selector_expression" {
			field := rightNode.ChildByFieldName("field", lang)
			if field != nil {
				t := strings.TrimSpace(string(field.Text(source)))
				iterableName = &t
			}
		} else if rightNode.Type(lang) == "call_expression" {
			funcNode := rightNode.ChildByFieldName("function", lang)
			var callee *string
			if funcNode != nil && funcNode.Type(lang) == "identifier" {
				t := strings.TrimSpace(string(funcNode.Text(source)))
				callee = &t
			} else if funcNode != nil && funcNode.Type(lang) == "selector_expression" {
				field := funcNode.ChildByFieldName("field", lang)
				if field != nil {
					t := strings.TrimSpace(string(field.Text(source)))
					callee = &t
				}
			}
			if callee != nil && ctx.ReturnTypeLookup != nil {
				rawReturn := ctx.ReturnTypeLookup.LookupRawReturnType(*callee)
				if rawReturn != nil {
					el := typeextractors.ExtractElementTypeFromString(*rawReturn, typeextractors.TypeArgLast)
					callExprElementType = el
				}
			}
		}
	}
	if iterableName == nil && callExprElementType == nil {
		return
	}

	var elementType *string
	if callExprElementType != nil {
		elementType = callExprElementType
	} else {
		containerTypeName := ""
		if t, ok := ctx.ScopeEnv[*iterableName]; ok {
			containerTypeName = t
		}
		typeArgPos := typeextractors.MethodToTypeArgPosition("", containerTypeName)

		extractFromTypeNode := func(typeNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
			return extractGoElementTypeFromTypeNode(typeNd, src, l, pos, 0)
		}
		findParamElementType := func(name string, startNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
			return findGoParamElementType(name, startNd, src, l, pos)
		}

		elementType = typeextractors.ResolveIterableElementType(
			*iterableName, node, source, lang,
			ctx.ScopeEnv, ctx.DeclarationTypeNodes, ctx.Scope,
			extractFromTypeNode, findParamElementType, typeArgPos,
		)
	}
	if elementType == nil {
		return
	}

	// The loop variable(s) are in the `left` field.
	leftNode := rangeClause.ChildByFieldName("left", lang)
	if leftNode == nil {
		return
	}

	var loopVarNode *gotreesitter.Node
	if leftNode.Type(lang) == "expression_list" {
		if leftNode.NamedChildCount() >= 2 {
			// Two-var form: `_, user` — second variable gets element/value type
			loopVarNode = leftNode.NamedChild(1)
		} else {
			// Single-var in expression_list — yields ELEMENT for channels, INDEX for slices/maps
			iterName := ""
			if iterableName != nil {
				iterName = *iterableName
			}
			if iterName != "" && isGoChannelType(iterName, ctx.ScopeEnv) {
				loopVarNode = leftNode.NamedChild(0)
			} else {
				return // index-only range on slice/map — skip
			}
		}
	} else {
		// Plain identifier (single-var form without expression_list)
		iterName := ""
		if iterableName != nil {
			iterName = *iterableName
		}
		if iterName != "" && isGoChannelType(iterName, ctx.ScopeEnv) {
			loopVarNode = leftNode
		} else {
			return // index-only range on slice/map — skip
		}
	}
	if loopVarNode == nil {
		return
	}

	// Skip the blank identifier `_`
	if strings.TrimSpace(string(loopVarNode.Text(source))) == "_" {
		return
	}

	loopVarName := typeextractors.ExtractVarName(loopVarNode, source, lang)
	if loopVarName != nil {
		ctx.ScopeEnv[*loopVarName] = *elementType
	}
}

// ---------------------------------------------------------------------------
// extractPendingAssignment — Go: alias := u
// ---------------------------------------------------------------------------

func goExtractPendingAssignment(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []typeextractors.PendingAssignment {
	if node.Type(lang) == "short_var_declaration" {
		left := node.ChildByFieldName("left", lang)
		right := node.ChildByFieldName("right", lang)
		if left == nil || right == nil {
			return nil
		}
		lhsNode := left
		if left.Type(lang) == "expression_list" {
			lhsNode = typeextractors.FirstNamedChild(left, lang)
		}
		rhsNode := right
		if right.Type(lang) == "expression_list" {
			rhsNode = typeextractors.FirstNamedChild(right, lang)
		}
		if lhsNode == nil || rhsNode == nil {
			return nil
		}
		if lhsNode.Type(lang) != "identifier" {
			return nil
		}
		lhs := strings.TrimSpace(string(lhsNode.Text(source)))
		if _, ok := scopeEnv[lhs]; ok {
			return nil
		}
		if rhsNode.Type(lang) == "identifier" {
			return []typeextractors.PendingAssignment{{
				Kind: typeextractors.PAKindCopy,
				Lhs:  lhs,
				Rhs:  strings.TrimSpace(string(rhsNode.Text(source))),
			}}
		}
		if rhsNode.Type(lang) == "selector_expression" {
			operand := rhsNode.ChildByFieldName("operand", lang)
			field := rhsNode.ChildByFieldName("field", lang)
			if operand != nil && operand.Type(lang) == "identifier" && field != nil {
				return []typeextractors.PendingAssignment{{
					Kind:     typeextractors.PAKindFieldAccess,
					Lhs:      lhs,
					Receiver: strings.TrimSpace(string(operand.Text(source))),
					Field:    strings.TrimSpace(string(field.Text(source))),
				}}
			}
		}
		if rhsNode.Type(lang) == "call_expression" {
			funcNode := rhsNode.ChildByFieldName("function", lang)
			if funcNode != nil && funcNode.Type(lang) == "identifier" {
				return []typeextractors.PendingAssignment{{
					Kind:   typeextractors.PAKindCallResult,
					Lhs:    lhs,
					Callee: strings.TrimSpace(string(funcNode.Text(source))),
				}}
			}
			if funcNode != nil && funcNode.Type(lang) == "selector_expression" {
				operand := funcNode.ChildByFieldName("operand", lang)
				field := funcNode.ChildByFieldName("field", lang)
				if operand != nil && operand.Type(lang) == "identifier" && field != nil {
					return []typeextractors.PendingAssignment{{
						Kind:     typeextractors.PAKindMethodCallResult,
						Lhs:      lhs,
						Receiver: strings.TrimSpace(string(operand.Text(source))),
						Method:   strings.TrimSpace(string(field.Text(source))),
					}}
				}
			}
		}
		return nil
	}
	if node.Type(lang) == "var_spec" || node.Type(lang) == "var_declaration" {
		var specs []*gotreesitter.Node
		if node.Type(lang) == "var_declaration" {
			for i := 0; i < int(node.NamedChildCount()); i++ {
				c := node.NamedChild(i)
				if c != nil && c.Type(lang) == "var_spec" {
					specs = append(specs, c)
				}
			}
		} else {
			specs = append(specs, node)
		}
		for _, spec := range specs {
			nameNode := spec.ChildByFieldName("name", lang)
			if nameNode == nil || nameNode.Type(lang) != "identifier" {
				continue
			}
			lhs := strings.TrimSpace(string(nameNode.Text(source)))
			if _, ok := scopeEnv[lhs]; ok {
				continue
			}
			// Check if there's an expression_list with a bare identifier
			var exprList *gotreesitter.Node
			for i := 0; i < int(spec.ChildCount()); i++ {
				c := spec.Child(i)
				if c != nil && c.Type(lang) == "expression_list" {
					exprList = c
					break
				}
			}
			rhsNode := typeextractors.FirstNamedChild(exprList, lang)
			if rhsNode == nil {
				continue
			}
			if rhsNode.Type(lang) == "identifier" {
				return []typeextractors.PendingAssignment{{
					Kind: typeextractors.PAKindCopy,
					Lhs:  lhs,
					Rhs:  strings.TrimSpace(string(rhsNode.Text(source))),
				}}
			}
			if rhsNode.Type(lang) == "selector_expression" {
				operand := rhsNode.ChildByFieldName("operand", lang)
				field := rhsNode.ChildByFieldName("field", lang)
				if operand != nil && operand.Type(lang) == "identifier" && field != nil {
					return []typeextractors.PendingAssignment{{
						Kind:     typeextractors.PAKindFieldAccess,
						Lhs:      lhs,
						Receiver: strings.TrimSpace(string(operand.Text(source))),
						Field:    strings.TrimSpace(string(field.Text(source))),
					}}
				}
			}
			if rhsNode.Type(lang) == "call_expression" {
				funcNode := rhsNode.ChildByFieldName("function", lang)
				if funcNode != nil && funcNode.Type(lang) == "identifier" {
					return []typeextractors.PendingAssignment{{
						Kind:   typeextractors.PAKindCallResult,
						Lhs:    lhs,
						Callee: strings.TrimSpace(string(funcNode.Text(source))),
					}}
				}
				if funcNode != nil && funcNode.Type(lang) == "selector_expression" {
					operand := funcNode.ChildByFieldName("operand", lang)
					field := funcNode.ChildByFieldName("field", lang)
					if operand != nil && operand.Type(lang) == "identifier" && field != nil {
						return []typeextractors.PendingAssignment{{
							Kind:     typeextractors.PAKindMethodCallResult,
							Lhs:      lhs,
							Receiver: strings.TrimSpace(string(operand.Text(source))),
							Method:   strings.TrimSpace(string(field.Text(source))),
						}}
					}
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// Go TypeConfig
// ---------------------------------------------------------------------------

// GoTypeConfig is the Go language type extractor configuration.
var GoTypeConfig = typeextractors.LanguageTypeConfig{
	DeclarationNodeTypes: []string{
		"var_declaration",
		"var_spec",
		"short_var_declaration",
	},
	ForLoopNodeTypes: []string{"for_statement"},
	ExtractDeclaration:       goExtractDeclaration,
	ExtractParameter:         goExtractParameter,
	ScanConstructorBinding:   goScanConstructorBinding,
	ExtractForLoopBinding:    goExtractForLoopBinding,
	ExtractPendingAssignment: goExtractPendingAssignment,
}

// Ensure utils import is used.
var _ = utils.FindChild