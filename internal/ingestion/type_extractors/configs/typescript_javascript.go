// Package configs provides per-language type extraction configurations.
// This file implements the TypeScript/JavaScript language type extractor configuration,
// ported from TS type-extractors/typescript.ts.
package configs

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/odvcencio/gotreesitter"

	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// ---------------------------------------------------------------------------
// JSDoc helpers
// ---------------------------------------------------------------------------

// normalizeJsDocType normalizes a raw JSDoc type string.
// Strips nullable/non-nullable prefixes, union with null/undefined, module: prefix,
// dotted paths, and generic wrappers.
func normalizeJsDocType(raw string) *string {
	t := strings.TrimSpace(raw)
	// Strip JSDoc nullable/non-nullable prefixes: ?User → User, !User → User
	if strings.HasPrefix(t, "?") || strings.HasPrefix(t, "!") {
		t = t[1:]
	}
	// Strip union with null/undefined/void: User|null → User
	parts := strings.Split(t, "|")
	var filtered []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "null" && p != "undefined" && p != "void" {
			filtered = append(filtered, p)
		}
	}
	if len(filtered) != 1 {
		return nil // ambiguous union
	}
	t = filtered[0]
	// Strip module: prefix — module:models.User → models.User
	if strings.HasPrefix(t, "module:") {
		t = t[7:]
	}
	// Take last segment of dotted path: models.User → User
	segments := strings.Split(t, ".")
	t = segments[len(segments)-1]
	// Strip generic wrapper: Promise<User> → Promise (base type, not inner)
	re := regexp.MustCompile(`^(\w+)\s*<`)
	if m := re.FindStringSubmatch(t); m != nil {
		t = m[1]
	}
	// Simple identifier check
	if m, _ := regexp.MatchString(`^\w+$`, t); m {
		return &t
	}
	return nil
}

// jsDocParamRe extracts JSDoc @param annotations: @param {Type} name
var jsDocParamRe = regexp.MustCompile(`@param\s*\{([^}]+)\}\s+\[?(\w+)[\]=]?[^\s]*`)

// collectJsDocParams collects JSDoc @param type bindings from comment nodes preceding a function/method.
func collectJsDocParams(funcNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) map[string]string {
	var commentTexts []string
	sibling := funcNode.PrevSibling()
	for sibling != nil {
		if sibling.Type(lang) == "comment" {
			commentTexts = append([]string{string(sibling.Text(source))}, commentTexts...)
		} else if sibling.IsNamed() && sibling.Type(lang) != "decorator" {
			break
		}
		sibling = sibling.PrevSibling()
	}
	if len(commentTexts) == 0 {
		return nil
	}

	params := make(map[string]string)
	commentBlock := strings.Join(commentTexts, "\n")
	matches := jsDocParamRe.FindAllStringSubmatch(commentBlock, -1)
	for _, match := range matches {
		typeName := normalizeJsDocType(match[1])
		paramName := match[2]
		if typeName != nil {
			params[paramName] = *typeName
		}
	}
	return params
}

// ---------------------------------------------------------------------------
// extractTSDeclaration — TypeScript: const x: Foo = ..., let x: Foo
// Also: JSDoc @param annotations on function/method definitions (for .js files).
// Also: public_field_definition (class fields with type annotation).
// ---------------------------------------------------------------------------

func extractTSDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	// JSDoc @param on functions/methods — pre-populate env with param types
	if node.Type(lang) == "function_declaration" || node.Type(lang) == "method_definition" {
		jsDocParams := collectJsDocParams(node, source, lang)
		for paramName, typeName := range jsDocParams {
			if _, exists := env[paramName]; !exists {
				env[paramName] = typeName
			}
		}
		return
	}

	// Class field: `private users: User[]`
	if node.Type(lang) == "public_field_definition" {
		nameNode := node.ChildByFieldName("name", lang)
		typeAnnotation := node.ChildByFieldName("type", lang)
		if nameNode == nil || typeAnnotation == nil {
			return
		}
		varName := strings.TrimSpace(string(nameNode.Text(source)))
		if varName == "" {
			return
		}
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeAnnotation, source, lang, 0)
		if typeName != nil {
			env[varName] = *typeName
		}
		return
	}

	// lexical_declaration / variable_declaration: iterate over variable_declarator children
	for i := 0; i < int(node.NamedChildCount()); i++ {
		declarator := node.NamedChild(i)
		if declarator == nil || declarator.Type(lang) != "variable_declarator" {
			continue
		}
		nameNode := declarator.ChildByFieldName("name", lang)
		typeAnnotation := declarator.ChildByFieldName("type", lang)
		if nameNode == nil || typeAnnotation == nil {
			continue
		}
		varName := typeextractors.ExtractVarName(nameNode, source, lang)
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeAnnotation, source, lang, 0)
		if varName != nil && typeName != nil {
			env[*varName] = *typeName
		}
	}
}

// ---------------------------------------------------------------------------
// extractTSParameter — TypeScript: required_parameter / optional_parameter → name: type
// ---------------------------------------------------------------------------

func extractTSParameter(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	var nameNode *gotreesitter.Node
	var typeNode *gotreesitter.Node

	if node.Type(lang) == "required_parameter" || node.Type(lang) == "optional_parameter" {
		nameNode = node.ChildByFieldName("pattern", lang)
		if nameNode == nil {
			nameNode = node.ChildByFieldName("name", lang)
		}
		typeNode = node.ChildByFieldName("type", lang)
	} else {
		// Generic fallback
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
// extractTSInitializer — TypeScript: const x = new User()
// Unwraps as_expression / non_null_expression to find new_expression.
// ---------------------------------------------------------------------------

func extractTSInitializer(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string, classNames typeextractors.ClassNameLookup) {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		declarator := node.NamedChild(i)
		if declarator == nil || declarator.Type(lang) != "variable_declarator" {
			continue
		}
		// Only activate when there is no explicit type annotation
		if declarator.ChildByFieldName("type", lang) != nil {
			continue
		}
		valueNode := declarator.ChildByFieldName("value", lang)
		// Unwrap `new User() as T`, `new User()!`, and double-cast `new User() as unknown as T`
		for valueNode != nil && (valueNode.Type(lang) == "as_expression" || valueNode.Type(lang) == "non_null_expression") {
			valueNode = typeextractors.FirstNamedChild(valueNode, lang)
		}
		if valueNode == nil || valueNode.Type(lang) != "new_expression" {
			continue
		}
		constructorNode := valueNode.ChildByFieldName("constructor", lang)
		if constructorNode == nil {
			continue
		}
		nameNode := declarator.ChildByFieldName("name", lang)
		if nameNode == nil {
			continue
		}
		varName := typeextractors.ExtractVarName(nameNode, source, lang)
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(constructorNode, source, lang, 0)
		if varName != nil && typeName != nil {
			env[*varName] = *typeName
		}
	}
}

// ---------------------------------------------------------------------------
// scanTSConstructorBinding — TypeScript/JavaScript: const user = getUser()
// variable_declarator with call_expression value, no type annotation.
// await is unwrapped.
// ---------------------------------------------------------------------------

func scanTSConstructorBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *typeextractors.ConstructorBindingResult {
	if node.Type(lang) != "variable_declarator" {
		return nil
	}
	if typeextractors.HasTypeAnnotation(node, lang) {
		return nil
	}
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode == nil || nameNode.Type(lang) != "identifier" {
		return nil
	}
	value := typeextractors.UnwrapAwait(node.ChildByFieldName("value", lang), lang)
	if value == nil || value.Type(lang) != "call_expression" {
		return nil
	}
	calleeName := typeextractors.ExtractCalleeName(value, source, lang)
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
// For-loop helpers
// ---------------------------------------------------------------------------

// tsFunctionNodeTypes lists TS function/method node types that carry a parameters list.
var tsFunctionNodeTypes = map[string]bool{
	"function_declaration":            true,
	"function_expression":             true,
	"arrow_function":                  true,
	"method_definition":               true,
	"generator_function":              true,
	"generator_function_declaration":  true,
}

// extractTsElementTypeFromAnnotation extracts element type from a TypeScript type annotation AST node.
// Handles: type_annotation ": User[]" → array_type → type_identifier "User",
//          type_annotation ": Array<User>" → generic_type → extractGenericTypeArgs → "User".
// Falls back to text-based extraction via ExtractElementTypeFromString.
func extractTsElementTypeFromAnnotation(typeAnnotation *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	return extractTsElementTypeFromAnnotationDepth(typeAnnotation, source, lang, pos, 0)
}

func extractTsElementTypeFromAnnotationDepth(typeAnnotation *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition, depth int) *string {
	if depth > 50 {
		return nil
	}
	// Unwrap type_annotation (the node text includes ': ' prefix)
	inner := typeAnnotation
	if inner.Type(lang) == "type_annotation" {
		child := typeextractors.FirstNamedChild(inner, lang)
		if child != nil {
			inner = child
		}
	}

	// readonly User[] — readonly_type wraps array_type: unwrap and recurse
	if inner.Type(lang) == "readonly_type" {
		wrapped := typeextractors.FirstNamedChild(inner, lang)
		if wrapped != nil {
			return extractTsElementTypeFromAnnotationDepth(wrapped, source, lang, pos, depth+1)
		}
	}

	// User[] — array_type: first named child is the element type
	if inner.Type(lang) == "array_type" {
		elem := typeextractors.FirstNamedChild(inner, lang)
		if elem != nil {
			return typeextractors.ExtractSimpleTypeNameFromNode(elem, source, lang, 0)
		}
	}

	// Array<User>, Map<string, User> — generic_type
	if inner.Type(lang) == "generic_type" {
		args := typeextractors.ExtractGenericTypeArgs(inner, source, lang, 0)
		if len(args) >= 1 {
			if pos == typeextractors.TypeArgFirst {
				return &args[0]
			}
			return &args[len(args)-1]
		}
	}

	// Fallback: strip ': ' prefix from type_annotation text and use string extraction
	rawText := strings.TrimSpace(string(inner.Text(source)))
	return typeextractors.ExtractElementTypeFromString(rawText, pos)
}

// findTsLocalDeclElementType searches a statement_block (function body) for a
// variable_declarator named `iterableName` that has a type annotation, preceding the given `beforeNode`.
func findTsLocalDeclElementType(iterableName string, blockNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, beforeNode *gotreesitter.Node, pos typeextractors.TypeArgPosition) *string {
	for i := 0; i < int(blockNode.NamedChildCount()); i++ {
		stmt := blockNode.NamedChild(i)
		if stmt == nil {
			continue
		}
		// Stop when we reach the for-loop itself
		if stmt == beforeNode || stmt.StartByte() >= beforeNode.StartByte() {
			break
		}
		// Look for lexical_declaration or variable_declaration
		if stmt.Type(lang) != "lexical_declaration" && stmt.Type(lang) != "variable_declaration" {
			continue
		}
		for j := 0; j < int(stmt.NamedChildCount()); j++ {
			decl := stmt.NamedChild(j)
			if decl == nil || decl.Type(lang) != "variable_declarator" {
				continue
			}
			nameNode := decl.ChildByFieldName("name", lang)
			if nameNode == nil || strings.TrimSpace(string(nameNode.Text(source))) != iterableName {
				continue
			}
			typeAnnotation := decl.ChildByFieldName("type", lang)
			if typeAnnotation != nil {
				return extractTsElementTypeFromAnnotation(typeAnnotation, source, lang, pos)
			}
		}
	}
	return nil
}

// findTsIterableElementType walks up the AST from a for-loop node to find the enclosing
// function scope, then searches its parameter list and local declarations for a variable
// named `iterableName` with a container type annotation.
func findTsIterableElementType(iterableName string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	current := startNode.Parent()
	// Capture the immediate statement_block parent to search local declarations
	var blockNode *gotreesitter.Node
	if current != nil && current.Type(lang) == "statement_block" {
		blockNode = current
	}

	for current != nil {
		if tsFunctionNodeTypes[current.Type(lang)] {
			// Search function parameters
			paramsNode := current.ChildByFieldName("parameters", lang)
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
					param := paramsNode.NamedChild(i)
					if param == nil {
						continue
					}
					patternNode := param.ChildByFieldName("pattern", lang)
					if patternNode == nil {
						patternNode = param.ChildByFieldName("name", lang)
					}
					if patternNode != nil && strings.TrimSpace(string(patternNode.Text(source))) == iterableName {
						typeAnnotation := param.ChildByFieldName("type", lang)
						if typeAnnotation != nil {
							return extractTsElementTypeFromAnnotation(typeAnnotation, source, lang, pos)
						}
					}
				}
			}
			// Search local declarations in the function body (statement_block)
			if blockNode != nil {
				if result := findTsLocalDeclElementType(iterableName, blockNode, source, lang, startNode, pos); result != nil {
					return result
				}
			}
			break // stop at the nearest function boundary
		}
		current = current.Parent()
	}
	return nil
}

// ---------------------------------------------------------------------------
// extractTSForLoopBinding — TypeScript/JavaScript: for (const user of users)
// Both for...of and for...in use the same for_in_statement AST node in tree-sitter.
// We differentiate by checking for the 'of' keyword among the unnamed children.
// Only handles for...of; for...in produces string keys, not element types.
// ---------------------------------------------------------------------------

func extractTSForLoopBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *typeextractors.ForLoopExtractorContext) {
	if node.Type(lang) != "for_in_statement" {
		return
	}

	// Confirm this is for...of, not for...in, by scanning unnamed children for 'of' keyword
	isForOf := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && !child.IsNamed() && string(child.Text(source)) == "of" {
			isForOf = true
			break
		}
	}
	if !isForOf {
		return
	}

	// The iterable is the 'right' field
	rightNode := node.ChildByFieldName("right", lang)
	var iterableName *string
	var methodName *string
	var callExprElementType *string

	if rightNode != nil && rightNode.Type(lang) == "identifier" {
		t := strings.TrimSpace(string(rightNode.Text(source)))
		iterableName = &t
	} else if rightNode != nil && rightNode.Type(lang) == "member_expression" {
		prop := rightNode.ChildByFieldName("property", lang)
		if prop != nil {
			t := strings.TrimSpace(string(prop.Text(source)))
			iterableName = &t
		}
	} else if rightNode != nil && rightNode.Type(lang) == "call_expression" {
		fn := rightNode.ChildByFieldName("function", lang)
		if fn != nil && fn.Type(lang) == "member_expression" {
			obj := fn.ChildByFieldName("object", lang)
			prop := fn.ChildByFieldName("property", lang)
			if obj != nil && obj.Type(lang) == "identifier" {
				t := strings.TrimSpace(string(obj.Text(source)))
				iterableName = &t
			} else if obj != nil && obj.Type(lang) == "member_expression" {
				// this.repos.values() → obj = this.repos → extract 'repos'
				innerProp := obj.ChildByFieldName("property", lang)
				if innerProp != nil {
					t := strings.TrimSpace(string(innerProp.Text(source)))
					iterableName = &t
				}
			}
			if prop != nil && prop.Type(lang) == "property_identifier" {
				t := strings.TrimSpace(string(prop.Text(source)))
				methodName = &t
			}
		} else if fn != nil && fn.Type(lang) == "identifier" {
			// Direct function call: for (const user of getUsers())
			calleeText := strings.TrimSpace(string(fn.Text(source)))
			if rawReturn := ctx.ReturnTypeLookup.LookupRawReturnType(calleeText); rawReturn != nil {
				el := typeextractors.ExtractElementTypeFromString(*rawReturn, typeextractors.TypeArgLast)
				if el != nil {
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
		containerTypeName := ctx.ScopeEnv[*iterableName]
		var mn string
		if methodName != nil {
			mn = *methodName
		}
		typeArgPos := typeextractors.MethodToTypeArgPosition(mn, containerTypeName)
		elementType = typeextractors.ResolveIterableElementType(
			*iterableName, node, source, lang,
			ctx.ScopeEnv, ctx.DeclarationTypeNodes, ctx.Scope,
			extractTsElementTypeFromAnnotation, findTsIterableElementType, typeArgPos,
		)
	}
	if elementType == nil {
		return
	}

	// The loop variable is the 'left' field
	leftNode := node.ChildByFieldName("left", lang)
	if leftNode == nil {
		return
	}

	// Handle destructured for-of: for (const [k, v] of entries)
	// Bind the LAST identifier to the element type (value in [key, value] patterns)
	if leftNode.Type(lang) == "array_pattern" {
		lastChild := typeextractors.LastNamedChild(leftNode, lang)
		if lastChild != nil && lastChild.Type(lang) == "identifier" {
			ctx.ScopeEnv[strings.TrimSpace(string(lastChild.Text(source)))] = *elementType
		}
		return
	}

	if leftNode.Type(lang) == "object_pattern" {
		// Object destructuring — skip to avoid false bindings
		return
	}

	loopVarNode := leftNode
	// `const user` parses as: left → variable_declarator containing an identifier named `user`
	if loopVarNode.Type(lang) == "variable_declarator" {
		nameNode := loopVarNode.ChildByFieldName("name", lang)
		if nameNode == nil {
			nameNode = typeextractors.FirstNamedChild(loopVarNode, lang)
		}
		if nameNode != nil {
			loopVarNode = nameNode
		}
	}

	loopVarName := typeextractors.ExtractVarName(loopVarNode, source, lang)
	if loopVarName != nil {
		ctx.ScopeEnv[*loopVarName] = *elementType
	}
}

// ---------------------------------------------------------------------------
// collectDestructuredFields — collect fieldAccess items from an object_pattern's destructured properties
// ---------------------------------------------------------------------------

func collectDestructuredFields(nameNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, receiver string, scopeEnv map[string]string) []typeextractors.PendingAssignment {
	var items []typeextractors.PendingAssignment
	for j := 0; j < int(nameNode.NamedChildCount()); j++ {
		prop := nameNode.NamedChild(j)
		if prop == nil {
			continue
		}
		if prop.Type(lang) == "shorthand_property_identifier_pattern" {
			// `const { name } = obj` → shorthand: varName = fieldName
			varName := strings.TrimSpace(string(prop.Text(source)))
			if _, exists := scopeEnv[varName]; !exists {
				items = append(items, typeextractors.PendingAssignment{
					Kind:     typeextractors.PAKindFieldAccess,
					Lhs:      varName,
					Receiver: receiver,
					Field:    varName,
				})
			}
		} else if prop.Type(lang) == "pair_pattern" {
			// `const { address: addr } = obj` → pair_pattern: key=field, value=varName
			keyNode := prop.ChildByFieldName("key", lang)
			valNode := prop.ChildByFieldName("value", lang)
			if keyNode != nil && valNode != nil {
				fieldName := strings.TrimSpace(string(keyNode.Text(source)))
				varName := strings.TrimSpace(string(valNode.Text(source)))
				if _, exists := scopeEnv[varName]; !exists {
					items = append(items, typeextractors.PendingAssignment{
						Kind:     typeextractors.PAKindFieldAccess,
						Lhs:      varName,
						Receiver: receiver,
						Field:    fieldName,
					})
				}
			}
		}
	}
	return items
}

// ---------------------------------------------------------------------------
// extractTSPendingAssignment — TS/JS: const alias = u → variable_declarator
// Also handles destructuring: `const { a, b } = obj` and `const { a } = fn()`
// ---------------------------------------------------------------------------

func extractTSPendingAssignment(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []typeextractors.PendingAssignment {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != "variable_declarator" {
			continue
		}
		nameNode := child.ChildByFieldName("name", lang)
		valueNode := child.ChildByFieldName("value", lang)
		if nameNode == nil || valueNode == nil {
			continue
		}

		// Object destructuring from identifier: `const { address, name } = user`
		if nameNode.Type(lang) == "object_pattern" && valueNode.Type(lang) == "identifier" {
			receiver := strings.TrimSpace(string(valueNode.Text(source)))
			items := collectDestructuredFields(nameNode, source, lang, receiver, scopeEnv)
			if len(items) > 0 {
				return items
			}
			continue
		}

		// Object destructuring from call/await: `const { x } = fn()` or `const { x } = await fn()`
		// Emits a synthetic callResult + N fieldAccess items resolved via fixpoint iteration.
		if nameNode.Type(lang) == "object_pattern" {
			callNode := typeextractors.UnwrapAwait(valueNode, lang)
			if callNode != nil && callNode.Type(lang) == "call_expression" {
				funcNode := callNode.ChildByFieldName("function", lang)
				if funcNode != nil {
					var syntheticVar string
					var leadItem typeextractors.PendingAssignment

					if funcNode.Type(lang) == "identifier" {
						calleeText := strings.TrimSpace(string(funcNode.Text(source)))
						syntheticVar = "__destr_" + calleeText + "_" + fmt.Sprintf("%d", callNode.StartByte())
						leadItem = typeextractors.PendingAssignment{
							Kind:   typeextractors.PAKindCallResult,
							Lhs:    syntheticVar,
							Callee: calleeText,
						}
					} else if funcNode.Type(lang) == "member_expression" {
						obj := funcNode.ChildByFieldName("object", lang)
						prop := funcNode.ChildByFieldName("property", lang)
						if obj != nil && prop != nil && prop.Type(lang) == "property_identifier" &&
							(obj.Type(lang) == "identifier" || obj.Type(lang) == "this") {
							propText := strings.TrimSpace(string(prop.Text(source)))
							objText := strings.TrimSpace(string(obj.Text(source)))
							syntheticVar = "__destr_" + propText + "_" + fmt.Sprintf("%d", callNode.StartByte())
							leadItem = typeextractors.PendingAssignment{
								Kind:     typeextractors.PAKindMethodCallResult,
								Lhs:      syntheticVar,
								Receiver: objText,
								Method:   propText,
							}
						}
					}

					if syntheticVar != "" {
						fieldItems := collectDestructuredFields(nameNode, source, lang, syntheticVar, scopeEnv)
						if len(fieldItems) > 0 {
							return append([]typeextractors.PendingAssignment{leadItem}, fieldItems...)
						}
					}
				}
			}
			continue
		}

		lhs := strings.TrimSpace(string(nameNode.Text(source)))
		if _, exists := scopeEnv[lhs]; exists {
			continue
		}
		if valueNode.Type(lang) == "identifier" {
			rhs := strings.TrimSpace(string(valueNode.Text(source)))
			return []typeextractors.PendingAssignment{{Kind: typeextractors.PAKindCopy, Lhs: lhs, Rhs: rhs}}
		}
		// member_expression RHS → fieldAccess (a.field, this.field)
		if valueNode.Type(lang) == "member_expression" {
			obj := valueNode.ChildByFieldName("object", lang)
			prop := valueNode.ChildByFieldName("property", lang)
			if obj != nil && prop != nil && prop.Type(lang) == "property_identifier" &&
				(obj.Type(lang) == "identifier" || obj.Type(lang) == "this") {
				objText := strings.TrimSpace(string(obj.Text(source)))
				propText := strings.TrimSpace(string(prop.Text(source)))
				return []typeextractors.PendingAssignment{
					{Kind: typeextractors.PAKindFieldAccess, Lhs: lhs, Receiver: objText, Field: propText},
				}
			}
			continue
		}
		// Unwrap await: `const user = await fetchUser()` or `await a.getC()`
		callNode := typeextractors.UnwrapAwait(valueNode, lang)
		if callNode == nil || callNode.Type(lang) != "call_expression" {
			continue
		}
		funcNode := callNode.ChildByFieldName("function", lang)
		if funcNode == nil {
			continue
		}
		// Simple call → callResult: getUser()
		if funcNode.Type(lang) == "identifier" {
			callee := strings.TrimSpace(string(funcNode.Text(source)))
			return []typeextractors.PendingAssignment{{Kind: typeextractors.PAKindCallResult, Lhs: lhs, Callee: callee}}
		}
		// Method call with receiver → methodCallResult: a.getC()
		if funcNode.Type(lang) == "member_expression" {
			obj := funcNode.ChildByFieldName("object", lang)
			prop := funcNode.ChildByFieldName("property", lang)
			if obj != nil && prop != nil && prop.Type(lang) == "property_identifier" &&
				(obj.Type(lang) == "identifier" || obj.Type(lang) == "this") {
				objText := strings.TrimSpace(string(obj.Text(source)))
				propText := strings.TrimSpace(string(prop.Text(source)))
				return []typeextractors.PendingAssignment{
					{Kind: typeextractors.PAKindMethodCallResult, Lhs: lhs, Receiver: objText, Method: propText},
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// nullCheckKeywords — keywords that indicate a null-comparison in binary expressions
// ---------------------------------------------------------------------------

var nullCheckKeywords = map[string]bool{
	"null":      true,
	"undefined": true,
}

// findIfConsequenceBlock walks up from the binary_expression through parenthesized_expression
// to if_statement, then returns the consequence block (statement_block).
func findIfConsequenceBlock(binaryExpr *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	current := binaryExpr.Parent()
	for current != nil {
		if current.Type(lang) == "if_statement" {
			// The consequence is the first statement_block child of if_statement
			for i := 0; i < int(current.ChildCount()); i++ {
				child := current.Child(i)
				if child != nil && child.Type(lang) == "statement_block" {
					return child
				}
			}
			return nil
		}
		// Stop climbing at function/block boundaries — don't cross scope
		switch current.Type(lang) {
		case "function_declaration", "function_expression", "arrow_function", "method_definition":
			return nil
		}
		current = current.Parent()
	}
	return nil
}

// ---------------------------------------------------------------------------
// extractTSPatternBinding — TS instanceof narrowing and null-check narrowing
// instanceof: x instanceof User → bind x to User
// null-check: x !== null, x != undefined → bind x to resolved type (with narrowingRange)
// ---------------------------------------------------------------------------

func extractTSPatternBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string, declarationTypeNodes map[string]*gotreesitter.Node, scope string) *typeextractors.PatternBindingResult {
	if node.Type(lang) != "binary_expression" {
		return nil
	}

	// Check for instanceof first
	hasInstanceof := false
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if c != nil && !c.IsNamed() && string(c.Text(source)) == "instanceof" {
			hasInstanceof = true
			break
		}
	}
	if hasInstanceof {
		left := node.NamedChild(0)
		right := node.NamedChild(1)
		if left == nil || right == nil || left.Type(lang) != "identifier" || right.Type(lang) != "identifier" {
			return nil
		}
		return &typeextractors.PatternBindingResult{
			VarName:  strings.TrimSpace(string(left.Text(source))),
			TypeName: strings.TrimSpace(string(right.Text(source))),
		}
	}

	// Null-check narrowing: x !== null, x != null, x !== undefined, x != undefined
	hasNotEqual := false
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if c != nil && !c.IsNamed() {
			ct := string(c.Text(source))
			if ct == "!==" || ct == "!=" {
				hasNotEqual = true
				break
			}
		}
	}
	if !hasNotEqual {
		return nil
	}

	left := node.NamedChild(0)
	right := node.NamedChild(1)
	if left == nil || right == nil {
		return nil
	}

	// Determine which side is the variable and which is null/undefined
	var varNode *gotreesitter.Node
	leftText := strings.TrimSpace(string(left.Text(source)))
	rightText := strings.TrimSpace(string(right.Text(source)))

	if left.Type(lang) == "identifier" && nullCheckKeywords[rightText] {
		varNode = left
	} else if right.Type(lang) == "identifier" && nullCheckKeywords[leftText] {
		varNode = right
	}
	if varNode == nil {
		return nil
	}

	varName := strings.TrimSpace(string(varNode.Text(source)))
	// Look up the variable's resolved type
	resolvedType, ok := scopeEnv[varName]
	if !ok {
		return nil
	}

	// Check if the original declaration type was nullable
	typeNodeKey := scope + "\x00" + varName
	declTypeNode, ok := declarationTypeNodes[typeNodeKey]
	if !ok {
		return nil
	}
	declText := string(declTypeNode.Text(source))
	// Only narrow if the original declaration was nullable
	if !strings.Contains(declText, "null") && !strings.Contains(declText, "undefined") {
		return nil
	}

	// Find the if-body block to scope the narrowing
	ifBody := findIfConsequenceBlock(node, lang)
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

// ---------------------------------------------------------------------------
// inferTSLiteralType — infer the type of a literal AST node for TypeScript overload disambiguation
// ---------------------------------------------------------------------------

func inferTSLiteralType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	switch node.Type(lang) {
	case "number":
		t := "number"
		return &t
	case "string", "template_string":
		t := "string"
		return &t
	case "true", "false":
		t := "boolean"
		return &t
	case "null":
		t := "null"
		return &t
	case "undefined":
		t := "undefined"
		return &t
	case "regex":
		t := "RegExp"
		return &t
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// TypeScriptTypeConfig — the exported configuration variable
// ---------------------------------------------------------------------------

var TypeScriptTypeConfig = typeextractors.LanguageTypeConfig{
	DeclarationNodeTypes: []string{
		"lexical_declaration",
		"variable_declaration",
		"function_declaration",
		"method_definition",
		"public_field_definition",
	},
	ForLoopNodeTypes:        []string{"for_in_statement"},
	PatternBindingNodeTypes: []string{"binary_expression"},
	ExtractDeclaration:       extractTSDeclaration,
	ExtractParameter:         extractTSParameter,
	ExtractInitializer:       extractTSInitializer,
	ScanConstructorBinding:   scanTSConstructorBinding,
	ExtractForLoopBinding:    extractTSForLoopBinding,
	ExtractPendingAssignment: extractTSPendingAssignment,
	ExtractPatternBinding:    extractTSPatternBinding,
	InferLiteralType:         inferTSLiteralType,
}

// Ensure utils.FindChild is referenced (import anchor)
var _ = utils.FindChild