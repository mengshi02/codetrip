// Package configs provides per-language type extraction configurations.
// This file implements the C# language type extractor configuration,
// ported from TS type-extractors/csharp.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// Known container property accessors that operate on the container itself.
var knownContainerProps = map[string]bool{"Keys": true, "Values": true}

// ---------------------------------------------------------------------------
// extractDeclaration — C#: Type x = ...; var x = new Type();
// ---------------------------------------------------------------------------

func csharpExtractDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	// C# tree-sitter: local_declaration_statement > variable_declaration > ...
	// Recursively descend through wrapper nodes
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "variable_declaration" || ct == "local_declaration_statement" {
			csharpExtractDeclaration(child, source, lang, env)
			return
		}
	}

	// At variable_declaration level: first child is type, rest are variable_declarators
	var typeNode *gotreesitter.Node
	var declarators []*gotreesitter.Node

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if typeNode == nil && ct != "variable_declarator" && ct != "equals_value_clause" {
			typeNode = child
		}
		if ct == "variable_declarator" {
			declarators = append(declarators, child)
		}
	}

	if typeNode == nil || len(declarators) == 0 {
		return
	}

	// Handle 'var x = new Foo()'
	var typeName *string
	if typeNode.Type(lang) == "implicit_type" && strings.TrimSpace(string(typeNode.Text(source))) == "var" {
		if len(declarators) == 1 {
			initializer := utils.FindChild(declarators[0], "object_creation_expression", lang)
			if initializer == nil {
				evc := utils.FindChild(declarators[0], "equals_value_clause", lang)
				if evc != nil {
					initializer = typeextractors.FirstNamedChild(evc, lang)
				}
			}
			if initializer != nil && initializer.Type(lang) == "object_creation_expression" {
				ctorType := initializer.ChildByFieldName("type", lang)
				if ctorType != nil {
					typeName = typeextractors.ExtractSimpleTypeNameFromNode(ctorType, source, lang, 0)
				}
			}
		}
	} else {
		typeName = typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
	}

	if typeName == nil {
		return
	}
	for _, decl := range declarators {
		nameNode := decl.ChildByFieldName("name", lang)
		if nameNode == nil {
			nameNode = typeextractors.FirstNamedChild(decl, lang)
		}
		if nameNode != nil {
			varName := typeextractors.ExtractVarName(nameNode, source, lang)
			if varName != nil {
				env[*varName] = *typeName
			}
		}
	}
}

// ---------------------------------------------------------------------------
// extractParameter — C#: parameter → type name
// ---------------------------------------------------------------------------

func csharpExtractParameter(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	var nameNode *gotreesitter.Node
	var typeNode *gotreesitter.Node

	if node.Type(lang) == "parameter" {
		typeNode = node.ChildByFieldName("type", lang)
		nameNode = node.ChildByFieldName("name", lang)
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
// scanConstructorBinding — C#: var x = SomeFactory(...)
// ---------------------------------------------------------------------------

func csharpScanConstructorBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *typeextractors.ConstructorBindingResult {
	if node.Type(lang) != "variable_declaration" {
		return nil
	}
	var typeNode *gotreesitter.Node
	var declarator *gotreesitter.Node
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "variable_declarator" {
			if declarator == nil {
				declarator = child
			}
		} else if typeNode == nil {
			typeNode = child
		}
	}
	if typeNode == nil || typeNode.Type(lang) != "implicit_type" {
		return nil
	}
	if declarator == nil {
		return nil
	}
	nameNode := declarator.ChildByFieldName("name", lang)
	if nameNode == nil {
		nameNode = typeextractors.FirstNamedChild(declarator, lang)
	}
	if nameNode == nil || nameNode.Type(lang) != "identifier" {
		return nil
	}

	// Find the initializer value
	var value *gotreesitter.Node
	for i := 0; i < int(declarator.NamedChildCount()); i++ {
		child := declarator.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "equals_value_clause" {
			value = typeextractors.FirstNamedChild(child, lang)
			break
		}
		ct := child.Type(lang)
		if ct == "invocation_expression" || ct == "object_creation_expression" || ct == "await_expression" {
			value = child
			break
		}
	}
	if value == nil {
		return nil
	}
	// Unwrap await
	value = typeextractors.UnwrapAwait(value, lang)
	if value == nil {
		return nil
	}
	// Skip object_creation_expression — handled by extractInitializer
	if value.Type(lang) == "object_creation_expression" {
		return nil
	}
	if value.Type(lang) != "invocation_expression" {
		return nil
	}
	fn := typeextractors.FirstNamedChild(value, lang)
	if fn == nil {
		return nil
	}
	calleeName := typeextractors.ExtractSimpleTypeNameFromNode(fn, source, lang, 0)
	if calleeName == nil {
		return nil
	}
	varName := strings.TrimSpace(string(nameNode.Text(source)))
	return &typeextractors.ConstructorBindingResult{
		VarName:    varName,
		CalleeName: *calleeName,
	}
}

// ---------------------------------------------------------------------------
// C# for-loop (foreach) element type helpers
// ---------------------------------------------------------------------------

// extractCSharpElementTypeFromTypeNode extracts element type from a C# type annotation.
func extractCSharpElementTypeFromTypeNode(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition, depth int) *string {
	if depth > 50 || typeNode == nil {
		return nil
	}
	// generic_name: List<User>, Dictionary<string, User>
	if typeNode.Type(lang) == "generic_name" {
		argList := utils.FindChild(typeNode, "type_argument_list", lang)
		if argList != nil && argList.NamedChildCount() >= 1 {
			if pos == typeextractors.TypeArgFirst {
				firstArg := argList.NamedChild(0)
				if firstArg != nil {
					return typeextractors.ExtractSimpleTypeNameFromNode(firstArg, source, lang, 0)
				}
			} else {
				lastArg := argList.NamedChild(argList.NamedChildCount() - 1)
				if lastArg != nil {
					return typeextractors.ExtractSimpleTypeNameFromNode(lastArg, source, lang, 0)
				}
			}
		}
	}
	// array_type: User[]
	if typeNode.Type(lang) == "array_type" {
		elemNode := typeextractors.FirstNamedChild(typeNode, lang)
		if elemNode != nil {
			return typeextractors.ExtractSimpleTypeNameFromNode(elemNode, source, lang, 0)
		}
	}
	// nullable_type: unwrap and recurse
	if typeNode.Type(lang) == "nullable_type" {
		inner := typeextractors.FirstNamedChild(typeNode, lang)
		if inner != nil {
			return extractCSharpElementTypeFromTypeNode(inner, source, lang, pos, depth+1)
		}
	}
	return nil
}

// findCSharpParamElementType walks up from a foreach to the enclosing method.
func findCSharpParamElementType(iterableName string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	current := startNode.Parent()
	for current != nil {
		ct := current.Type(lang)
		if ct == "method_declaration" || ct == "local_function_statement" {
			paramsNode := current.ChildByFieldName("parameters", lang)
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
					param := paramsNode.NamedChild(i)
					if param == nil || param.Type(lang) != "parameter" {
						continue
					}
					nameNode := param.ChildByFieldName("name", lang)
					if nameNode == nil || strings.TrimSpace(string(nameNode.Text(source))) != iterableName {
						continue
					}
					typeNode := param.ChildByFieldName("type", lang)
					if typeNode != nil {
						return extractCSharpElementTypeFromTypeNode(typeNode, source, lang, pos, 0)
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
// extractForLoopBinding — C#: foreach (var user in users)
// ---------------------------------------------------------------------------

func csharpExtractForLoopBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *typeextractors.ForLoopExtractorContext) {
	typeNode := node.ChildByFieldName("type", lang)
	nameNode := node.ChildByFieldName("left", lang)
	if typeNode == nil || nameNode == nil {
		return
	}
	varName := typeextractors.ExtractVarName(nameNode, source, lang)
	if varName == nil {
		return
	}

	// Explicit type: foreach (User user in users)
	typeText := strings.TrimSpace(string(typeNode.Text(source)))
	if !(typeNode.Type(lang) == "implicit_type" && typeText == "var") {
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
		if typeName != nil {
			ctx.ScopeEnv[*varName] = *typeName
		}
		return
	}

	// Tier 1c: implicit type (var) — resolve from iterable's container type
	rightNode := node.ChildByFieldName("right", lang)
	var iterableName *string
	var methodName *string
	var callExprElementType *string

	if rightNode != nil {
		if rightNode.Type(lang) == "identifier" {
			t := strings.TrimSpace(string(rightNode.Text(source)))
			iterableName = &t
		} else if rightNode.Type(lang) == "member_access_expression" {
			obj := rightNode.ChildByFieldName("expression", lang)
			prop := rightNode.ChildByFieldName("name", lang)
			propText := ""
			if prop != nil && prop.Type(lang) == "identifier" {
				propText = strings.TrimSpace(string(prop.Text(source)))
			}
			if propText != "" && knownContainerProps[propText] {
				if obj != nil && obj.Type(lang) == "identifier" {
					t := strings.TrimSpace(string(obj.Text(source)))
					iterableName = &t
				} else if obj != nil && obj.Type(lang) == "member_access_expression" {
					innerProp := obj.ChildByFieldName("name", lang)
					if innerProp != nil {
						t := strings.TrimSpace(string(innerProp.Text(source)))
						iterableName = &t
					}
				}
				methodName = &propText
			} else if propText != "" {
				iterableName = &propText
			}
		} else if rightNode.Type(lang) == "invocation_expression" {
			fn := typeextractors.FirstNamedChild(rightNode, lang)
			if fn != nil && fn.Type(lang) == "member_access_expression" {
				obj := fn.ChildByFieldName("expression", lang)
				prop := fn.ChildByFieldName("name", lang)
				if obj != nil && obj.Type(lang) == "identifier" {
					t := strings.TrimSpace(string(obj.Text(source)))
					iterableName = &t
				}
				if prop != nil && prop.Type(lang) == "identifier" {
					t := strings.TrimSpace(string(prop.Text(source)))
					methodName = &t
				}
			} else if fn != nil && fn.Type(lang) == "identifier" {
				// Direct function call: foreach (var u in GetUsers())
				if ctx.ReturnTypeLookup != nil {
					fnText := strings.TrimSpace(string(fn.Text(source)))
					rawReturn := ctx.ReturnTypeLookup.LookupRawReturnType(fnText)
					if rawReturn != nil {
						el := typeextractors.ExtractElementTypeFromString(*rawReturn, typeextractors.TypeArgLast)
						callExprElementType = el
					}
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
		if methodName != nil {
			typeArgPos = typeextractors.MethodToTypeArgPosition(*methodName, containerTypeName)
		}

		extractFromTypeNode := func(typeNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
			return extractCSharpElementTypeFromTypeNode(typeNd, src, l, pos, 0)
		}
		findParamElementType := func(name string, startNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
			return findCSharpParamElementType(name, startNd, src, l, pos)
		}

		elementType = typeextractors.ResolveIterableElementType(
			*iterableName, node, source, lang,
			ctx.ScopeEnv, ctx.DeclarationTypeNodes, ctx.Scope,
			extractFromTypeNode, findParamElementType, typeArgPos,
		)
	}
	if elementType != nil {
		ctx.ScopeEnv[*varName] = *elementType
	}
}

// ---------------------------------------------------------------------------
// extractPatternBinding — C#: obj is Type variable, x != null
// ---------------------------------------------------------------------------

// findCSharpIfConsequenceBlock finds the if-body block for a C# null-check.
func findCSharpIfConsequenceBlock(expr *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	current := expr.Parent()
	for current != nil {
		if current.Type(lang) == "if_statement" {
			consequence := current.ChildByFieldName("consequence", lang)
			if consequence != nil {
				return consequence
			}
			for i := 0; i < int(current.ChildCount()); i++ {
				child := current.Child(i)
				if child != nil && child.Type(lang) == "block" {
					return child
				}
			}
			return nil
		}
		ct := current.Type(lang)
		if ct == "block" || ct == "method_declaration" || ct == "constructor_declaration" ||
			ct == "local_function_statement" || ct == "lambda_expression" {
			return nil
		}
		current = current.Parent()
	}
	return nil
}

// isCSharpNullableDecl checks if a C# declaration type node represents a nullable type.
func isCSharpNullableDecl(declTypeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
	if declTypeNode.Type(lang) == "nullable_type" {
		return true
	}
	return strings.Contains(string(declTypeNode.Text(source)), "?")
}

func csharpExtractPatternBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string, declarationTypeNodes map[string]*gotreesitter.Node, scope string) *typeextractors.PatternBindingResult {
	// is_pattern_expression: `obj is User user`
	if node.Type(lang) == "is_pattern_expression" {
		pattern := node.ChildByFieldName("pattern", lang)
		if pattern == nil {
			return nil
		}
		// Standard type pattern
		if pattern.Type(lang) == "declaration_pattern" || pattern.Type(lang) == "recursive_pattern" {
			typeNode := pattern.ChildByFieldName("type", lang)
			nameNode := pattern.ChildByFieldName("name", lang)
			if typeNode == nil || nameNode == nil {
				return nil
			}
			typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
			varName := typeextractors.ExtractVarName(nameNode, source, lang)
			if typeName == nil || varName == nil {
				return nil
			}
			return &typeextractors.PatternBindingResult{
				VarName:  *varName,
				TypeName: *typeName,
			}
		}
		// Null-check: `x is not null`
		if pattern.Type(lang) == "negated_pattern" {
			inner := typeextractors.FirstNamedChild(pattern, lang)
			if inner != nil && inner.Type(lang) == "constant_pattern" {
				literal := typeextractors.FirstNamedChild(inner, lang)
				if literal == nil {
					for i := 0; i < int(inner.ChildCount()); i++ {
						c := inner.Child(i)
						if c != nil {
							literal = c
							break
						}
					}
				}
				if literal != nil && (literal.Type(lang) == "null_literal" || strings.TrimSpace(string(literal.Text(source))) == "null") {
					expr := node.ChildByFieldName("expression", lang)
					if expr == nil || expr.Type(lang) != "identifier" {
						return nil
					}
					varName := strings.TrimSpace(string(expr.Text(source)))
					resolvedType, ok := scopeEnv[varName]
					if !ok {
						return nil
					}
					declTypeNode := declarationTypeNodes[scope+"\x00"+varName]
					if declTypeNode == nil || !isCSharpNullableDecl(declTypeNode, source, lang) {
						return nil
					}
					ifBody := findCSharpIfConsequenceBlock(node, lang)
					if ifBody == nil {
						return nil
					}
					return &typeextractors.PatternBindingResult{
						VarName:  varName,
						TypeName: resolvedType,
						NarrowingRange: &typeextractors.NarrowingRange{
							StartIndex: ifBody.StartByte(),
							EndIndex:   ifBody.EndByte(),
						},
					}
				}
			}
		}
		return nil
	}

	// declaration_pattern / recursive_pattern: standalone in switch
	if node.Type(lang) == "declaration_pattern" || node.Type(lang) == "recursive_pattern" {
		typeNode := node.ChildByFieldName("type", lang)
		nameNode := node.ChildByFieldName("name", lang)
		if typeNode == nil || nameNode == nil {
			return nil
		}
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
		varName := typeextractors.ExtractVarName(nameNode, source, lang)
		if typeName == nil || varName == nil {
			return nil
		}
		return &typeextractors.PatternBindingResult{
			VarName:  *varName,
			TypeName: *typeName,
		}
	}

	// Null-check: `x != null`
	if node.Type(lang) == "binary_expression" {
		var opFound bool
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c != nil && !c.IsNamed() && strings.TrimSpace(string(c.Text(source))) == "!=" {
				opFound = true
				break
			}
		}
		if !opFound {
			return nil
		}
		left := node.NamedChild(0)
		right := node.NamedChild(1)
		if left == nil || right == nil {
			return nil
		}
		var varNode *gotreesitter.Node
		leftText := strings.TrimSpace(string(left.Text(source)))
		rightText := strings.TrimSpace(string(right.Text(source)))
		if left.Type(lang) == "identifier" && (right.Type(lang) == "null_literal" || rightText == "null") {
			varNode = left
		} else if right.Type(lang) == "identifier" && (left.Type(lang) == "null_literal" || leftText == "null") {
			varNode = right
		}
		if varNode == nil {
			return nil
		}
		varName := strings.TrimSpace(string(varNode.Text(source)))
		resolvedType, ok := scopeEnv[varName]
		if !ok {
			return nil
		}
		declTypeNode := declarationTypeNodes[scope+"\x00"+varName]
		if declTypeNode == nil || !isCSharpNullableDecl(declTypeNode, source, lang) {
			return nil
		}
		ifBody := findCSharpIfConsequenceBlock(node, lang)
		if ifBody == nil {
			return nil
		}
		return &typeextractors.PatternBindingResult{
			VarName:  varName,
			TypeName: resolvedType,
			NarrowingRange: &typeextractors.NarrowingRange{
				StartIndex: ifBody.StartByte(),
				EndIndex:   ifBody.EndByte(),
			},
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// extractPendingAssignment — C#: var alias = u
// ---------------------------------------------------------------------------

func csharpExtractPendingAssignment(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []typeextractors.PendingAssignment {
	if node.Type(lang) == "is_pattern_expression" || node.Type(lang) == "field_declaration" {
		return nil
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != "variable_declarator" {
			continue
		}
		nameNode := child.ChildByFieldName("name", lang)
		if nameNode == nil {
			continue
		}
		lhs := strings.TrimSpace(string(nameNode.Text(source)))
		if _, ok := scopeEnv[lhs]; ok {
			continue
		}
		// C# wraps value in equals_value_clause
		var evc *gotreesitter.Node
		for j := 0; j < int(child.ChildCount()); j++ {
			c := child.Child(j)
			if c != nil && c.Type(lang) == "equals_value_clause" {
				evc = c
				break
			}
		}
		var valueNode *gotreesitter.Node
		if evc != nil {
			valueNode = typeextractors.FirstNamedChild(evc, lang)
		} else {
			cnt := child.NamedChildCount()
			if cnt > 0 {
				valueNode = child.NamedChild(cnt - 1)
			}
		}
		if valueNode == nil || valueNode == nameNode {
			continue
		}
		vt := valueNode.Type(lang)
		if vt == "identifier" || vt == "simple_identifier" {
			return []typeextractors.PendingAssignment{{
				Kind: typeextractors.PAKindCopy,
				Lhs:  lhs,
				Rhs:  strings.TrimSpace(string(valueNode.Text(source))),
			}}
		}
		// member_access_expression → fieldAccess
		if vt == "member_access_expression" {
			expr := valueNode.ChildByFieldName("expression", lang)
			name := valueNode.ChildByFieldName("name", lang)
			if expr != nil && expr.Type(lang) == "identifier" && name != nil && name.Type(lang) == "identifier" {
				return []typeextractors.PendingAssignment{{
					Kind:     typeextractors.PAKindFieldAccess,
					Lhs:      lhs,
					Receiver: strings.TrimSpace(string(expr.Text(source))),
					Field:    strings.TrimSpace(string(name.Text(source))),
				}}
			}
		}
		// invocation_expression
		if vt == "invocation_expression" {
			funcNode := typeextractors.FirstNamedChild(valueNode, lang)
			if funcNode != nil {
				ft := funcNode.Type(lang)
				if ft == "identifier_name" || ft == "identifier" {
					return []typeextractors.PendingAssignment{{
						Kind:   typeextractors.PAKindCallResult,
						Lhs:    lhs,
						Callee: strings.TrimSpace(string(funcNode.Text(source))),
					}}
				}
				if ft == "member_access_expression" {
					expr := funcNode.ChildByFieldName("expression", lang)
					name := funcNode.ChildByFieldName("name", lang)
					if expr != nil && expr.Type(lang) == "identifier" && name != nil && name.Type(lang) == "identifier" {
						return []typeextractors.PendingAssignment{{
							Kind:     typeextractors.PAKindMethodCallResult,
							Lhs:      lhs,
							Receiver: strings.TrimSpace(string(expr.Text(source))),
							Method:   strings.TrimSpace(string(name.Text(source))),
						}}
					}
				}
			}
		}
		// await_expression → unwrap and check inner
		if vt == "await_expression" {
			inner := typeextractors.FirstNamedChild(valueNode, lang)
			if inner != nil && inner.Type(lang) == "invocation_expression" {
				funcNode := typeextractors.FirstNamedChild(inner, lang)
				if funcNode != nil {
					ft := funcNode.Type(lang)
					if ft == "identifier_name" || ft == "identifier" {
						return []typeextractors.PendingAssignment{{
							Kind:   typeextractors.PAKindCallResult,
							Lhs:    lhs,
							Callee: strings.TrimSpace(string(funcNode.Text(source))),
						}}
					}
					if ft == "member_access_expression" {
						expr := funcNode.ChildByFieldName("expression", lang)
						name := funcNode.ChildByFieldName("name", lang)
						if expr != nil && expr.Type(lang) == "identifier" && name != nil && name.Type(lang) == "identifier" {
							return []typeextractors.PendingAssignment{{
								Kind:     typeextractors.PAKindMethodCallResult,
								Lhs:      lhs,
								Receiver: strings.TrimSpace(string(expr.Text(source))),
								Method:   strings.TrimSpace(string(name.Text(source))),
							}}
						}
					}
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// inferLiteralType — C#
// ---------------------------------------------------------------------------

func csharpInferLiteralType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	t := strings.TrimSpace(string(node.Text(source)))
	switch node.Type(lang) {
	case "integer_literal":
		if strings.HasSuffix(t, "L") || strings.HasSuffix(t, "l") {
			s := "long"
			return &s
		}
		s := "int"
		return &s
	case "real_literal":
		if strings.HasSuffix(t, "f") || strings.HasSuffix(t, "F") {
			s := "float"
			return &s
		}
		if strings.HasSuffix(t, "m") || strings.HasSuffix(t, "M") {
			s := "decimal"
			return &s
		}
		s := "double"
		return &s
	case "string_literal", "verbatim_string_literal", "raw_string_literal", "interpolated_string_expression":
		s := "string"
		return &s
	case "character_literal":
		s := "char"
		return &s
	case "boolean_literal":
		s := "bool"
		return &s
	case "null_literal":
		s := "null"
		return &s
	}
	return nil
}

// ---------------------------------------------------------------------------
// getDeclarationTypeNode — C# wrapper node handling
// ---------------------------------------------------------------------------

func csharpGetDeclarationTypeNode(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	direct := node.ChildByFieldName("type", lang)
	if direct != nil {
		return direct
	}
	// C# field_declaration / local_declaration_statement wrap type inside variable_declaration
	wrapped := node.ChildByFieldName("declaration", lang)
	if wrapped == nil {
		for i := 0; i < int(node.NamedChildCount()); i++ {
			c := node.NamedChild(i)
			if c != nil && c.Type(lang) == "variable_declaration" {
				wrapped = c
				break
			}
		}
	}
	if wrapped != nil {
		return wrapped.ChildByFieldName("type", lang)
	}
	return nil
}

// ---------------------------------------------------------------------------
// C# TypeConfig
// ---------------------------------------------------------------------------

// CSharpTypeConfig is the C# language type extractor configuration.
var CSharpTypeConfig = typeextractors.LanguageTypeConfig{
	DeclarationNodeTypes: []string{
		"local_declaration_statement",
		"variable_declaration",
		"field_declaration",
	},
	GetDeclarationTypeNode: csharpGetDeclarationTypeNode,
	ForLoopNodeTypes:      []string{"foreach_statement"},
	PatternBindingNodeTypes: []string{
		"is_pattern_expression",
		"declaration_pattern",
		"recursive_pattern",
		"binary_expression",
	},
	ExtractDeclaration:       csharpExtractDeclaration,
	ExtractParameter:         csharpExtractParameter,
	ScanConstructorBinding:   csharpScanConstructorBinding,
	ExtractForLoopBinding:    csharpExtractForLoopBinding,
	ExtractPendingAssignment: csharpExtractPendingAssignment,
	ExtractPatternBinding:    csharpExtractPatternBinding,
	InferLiteralType:         csharpInferLiteralType,
}

// Ensure utils import is used.
var _ = utils.FindChild