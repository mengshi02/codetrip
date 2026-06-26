// Package configs provides per-language type extraction configurations.
// This file implements the Rust language type extractor configuration,
// ported from TS type-extractors/rust.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// ---------------------------------------------------------------------------
// Helper: findEnclosingImplType — walk up to enclosing impl_item, extract type name
// ---------------------------------------------------------------------------

func findEnclosingImplType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	current := node.Parent()
	for current != nil {
		if current.Type(lang) == "impl_item" {
			typeNode := current.ChildByFieldName("type", lang)
			if typeNode != nil {
				return typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
			}
		}
		current = current.Parent()
	}
	return nil
}

// ---------------------------------------------------------------------------
// Helper: extractStructPatternType — extract type name from struct_pattern's 'type' field
// ---------------------------------------------------------------------------

func extractStructPatternType(structPattern *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	typeNode := structPattern.ChildByFieldName("type", lang)
	if typeNode == nil {
		return nil
	}
	return typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
}

// ---------------------------------------------------------------------------
// Helper: extractCapturedPatternBindings — recursively scan for captured_pattern nodes
// x @ StructType { .. } → varName: typeName
// Also recurses into tuple_struct_pattern for nested captured_patterns.
// ---------------------------------------------------------------------------

func extractCapturedPatternBindings(pattern *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string, depth int) {
	if depth > 50 {
		return
	}
	if pattern.Type(lang) == "captured_pattern" {
		nameNode := typeextractors.FirstNamedChild(pattern, lang)
		if nameNode == nil || nameNode.Type(lang) != "identifier" {
			return
		}
		for i := 0; i < int(pattern.NamedChildCount()); i++ {
			child := pattern.NamedChild(i)
			if child != nil && child.Type(lang) == "struct_pattern" {
				typeName := extractStructPatternType(child, source, lang)
				if typeName != nil {
					varName := strings.TrimSpace(string(nameNode.Text(source)))
					env[varName] = *typeName
				}
				return
			}
		}
		return
	}
	// Recurse into tuple_struct_pattern children for nested captured_patterns
	// e.g., Some(user @ User { .. })
	if pattern.Type(lang) == "tuple_struct_pattern" {
		for i := 0; i < int(pattern.NamedChildCount()); i++ {
			child := pattern.NamedChild(i)
			if child != nil {
				extractCapturedPatternBindings(child, source, lang, env, depth+1)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// extractRustDeclaration — Rust: let x: Foo = ... | if let / while let pattern bindings
// ---------------------------------------------------------------------------

func extractRustDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	if node.Type(lang) == "let_condition" {
		// if let / while let: extract type bindings from pattern matching.
		// captured_pattern: `if let user @ User { .. } = expr` → user: User
		// tuple_struct_pattern with nested captured_pattern:
		//   `if let Some(user @ User { .. }) = expr` → user: User
		pattern := node.ChildByFieldName("pattern", lang)
		if pattern == nil {
			return
		}
		extractCapturedPatternBindings(pattern, source, lang, env, 0)
		return
	}

	// Standard let_declaration: let x: Foo = ...
	pattern := node.ChildByFieldName("pattern", lang)
	typeNode := node.ChildByFieldName("type", lang)
	if pattern == nil || typeNode == nil {
		return
	}
	varName := typeextractors.ExtractVarName(pattern, source, lang)
	typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
	if varName != nil && typeName != nil {
		env[*varName] = *typeName
	}
}

// ---------------------------------------------------------------------------
// extractRustInitializer — Rust: let x = User::new(), let x = User { ... }
// ---------------------------------------------------------------------------

func extractRustInitializer(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string, classNames typeextractors.ClassNameLookup) {
	// Skip if there's an explicit type annotation — Tier 0 already handled it
	if node.ChildByFieldName("type", lang) != nil {
		return
	}
	pattern := node.ChildByFieldName("pattern", lang)
	value := node.ChildByFieldName("value", lang)
	if pattern == nil || value == nil {
		return
	}

	// Rust struct literal: let user = User { name: "alice", age: 30 }
	if value.Type(lang) == "struct_expression" {
		typeNode := value.ChildByFieldName("name", lang)
		if typeNode == nil {
			return
		}
		rawType := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
		if rawType == nil {
			return
		}
		// Resolve Self to the actual struct/enum name from the enclosing impl block
		typeName := rawType
		if *rawType == "Self" {
			if t := findEnclosingImplType(node, source, lang); t != nil {
				typeName = t
			}
		}
		varName := typeextractors.ExtractVarName(pattern, source, lang)
		if varName != nil && typeName != nil {
			env[*varName] = *typeName
		}
		return
	}

	// Unit struct instantiation: let svc = UserService; (bare identifier)
	if value.Type(lang) == "identifier" {
		idText := strings.TrimSpace(string(value.Text(source)))
		if classNames.Has(idText) {
			varName := typeextractors.ExtractVarName(pattern, source, lang)
			if varName != nil {
				env[*varName] = idText
			}
			return
		}
	}

	if value.Type(lang) != "call_expression" {
		return
	}
	funcNode := value.ChildByFieldName("function", lang)
	if funcNode == nil || funcNode.Type(lang) != "scoped_identifier" {
		return
	}
	nameField := funcNode.ChildByFieldName("name", lang)
	// Only match ::new() and ::default()
	if nameField == nil {
		return
	}
	nameText := strings.TrimSpace(string(nameField.Text(source)))
	if nameText != "new" && nameText != "default" {
		return
	}
	pathField := funcNode.ChildByFieldName("path", lang)
	if pathField == nil {
		return
	}
	rawType := typeextractors.ExtractSimpleTypeNameFromNode(pathField, source, lang, 0)
	if rawType == nil {
		return
	}
	// Resolve Self to the actual struct/enum name from the enclosing impl block
	typeName := rawType
	if *rawType == "Self" {
		if t := findEnclosingImplType(node, source, lang); t != nil {
			typeName = t
		}
	}
	varName := typeextractors.ExtractVarName(pattern, source, lang)
	if varName != nil && typeName != nil {
		env[*varName] = *typeName
	}
}

// ---------------------------------------------------------------------------
// extractRustParameter — Rust: parameter → pattern: type
// ---------------------------------------------------------------------------

func extractRustParameter(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	var nameNode *gotreesitter.Node
	var typeNode *gotreesitter.Node

	if node.Type(lang) == "parameter" {
		nameNode = node.ChildByFieldName("pattern", lang)
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
// scanRustConstructorBinding — Rust: let user = get_user("alice")
// Skips annotated declarations and scoped_identifier callees named new/default.
// Unwraps mut_pattern for the inner identifier.
// Unwraps .await: let user = get_user().await
// ---------------------------------------------------------------------------

func scanRustConstructorBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *typeextractors.ConstructorBindingResult {
	if node.Type(lang) != "let_declaration" {
		return nil
	}
	if typeextractors.HasTypeAnnotation(node, lang) {
		return nil
	}
	patternNode := node.ChildByFieldName("pattern", lang)
	if patternNode == nil {
		return nil
	}
	if patternNode.Type(lang) == "mut_pattern" {
		patternNode = typeextractors.FirstNamedChild(patternNode, lang)
		if patternNode == nil {
			return nil
		}
	}
	if patternNode.Type(lang) != "identifier" {
		return nil
	}
	// Unwrap .await: let user = get_user().await → await_expression wraps call_expression
	value := typeextractors.UnwrapAwait(node.ChildByFieldName("value", lang), lang)
	if value == nil || value.Type(lang) != "call_expression" {
		return nil
	}
	funcNode := value.ChildByFieldName("function", lang)
	if funcNode == nil {
		return nil
	}
	if funcNode.Type(lang) == "scoped_identifier" {
		methodName := typeextractors.LastNamedChild(funcNode, lang)
		if methodName != nil {
			mt := strings.TrimSpace(string(methodName.Text(source)))
			if mt == "new" || mt == "default" {
				return nil
			}
		}
	}
	calleeName := typeextractors.ExtractSimpleTypeNameFromNode(funcNode, source, lang, 0)
	if calleeName == nil {
		return nil
	}
	varName := strings.TrimSpace(string(patternNode.Text(source)))
	return &typeextractors.ConstructorBindingResult{
		VarName:    varName,
		CalleeName: *calleeName,
	}
}

// ---------------------------------------------------------------------------
// extractRustPendingAssignment — Rust: let alias = u; / struct destructuring
// Also handles: let Point { x, y } = p → N fieldAccess items
// ---------------------------------------------------------------------------

func extractRustPendingAssignment(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []typeextractors.PendingAssignment {
	if node.Type(lang) != "let_declaration" {
		return nil
	}
	pattern := node.ChildByFieldName("pattern", lang)
	value := node.ChildByFieldName("value", lang)
	if pattern == nil || value == nil {
		return nil
	}

	// Struct pattern destructuring: `let Point { x, y } = receiver`
	if pattern.Type(lang) == "struct_pattern" && value.Type(lang) == "identifier" {
		receiver := strings.TrimSpace(string(value.Text(source)))
		var items []typeextractors.PendingAssignment
		for j := 0; j < int(pattern.NamedChildCount()); j++ {
			field := pattern.NamedChild(j)
			if field == nil {
				continue
			}
			if field.Type(lang) == "field_pattern" {
				nameNode := field.ChildByFieldName("name", lang)
				patNode := field.ChildByFieldName("pattern", lang)
				if nameNode != nil && patNode != nil {
					fieldName := strings.TrimSpace(string(nameNode.Text(source)))
					varName := typeextractors.ExtractVarName(patNode, source, lang)
					if varName != nil {
						if _, exists := scopeEnv[*varName]; !exists {
							items = append(items, typeextractors.PendingAssignment{
								Kind:     typeextractors.PAKindFieldAccess,
								Lhs:      *varName,
								Receiver: receiver,
								Field:    fieldName,
							})
						}
					}
				} else if nameNode != nil {
					// Shorthand: `Point { x }` → varName = fieldName
					varName := strings.TrimSpace(string(nameNode.Text(source)))
					if _, exists := scopeEnv[varName]; !exists {
						items = append(items, typeextractors.PendingAssignment{
							Kind:     typeextractors.PAKindFieldAccess,
							Lhs:      varName,
							Receiver: receiver,
							Field:    varName,
						})
					}
				}
			}
		}
		if len(items) > 0 {
			return items
		}
		return nil
	}

	lhs := typeextractors.ExtractVarName(pattern, source, lang)
	if lhs == nil {
		return nil
	}
	if _, exists := scopeEnv[*lhs]; exists {
		return nil
	}
	// Unwrap Rust .await: let user = get_user().await → call_expression
	unwrapped := typeextractors.UnwrapAwait(value, lang)
	if unwrapped == nil {
		unwrapped = value
	}
	if unwrapped.Type(lang) == "identifier" {
		rhs := strings.TrimSpace(string(unwrapped.Text(source)))
		return []typeextractors.PendingAssignment{{Kind: typeextractors.PAKindCopy, Lhs: *lhs, Rhs: rhs}}
	}
	// field_expression RHS → fieldAccess (a.field)
	if unwrapped.Type(lang) == "field_expression" {
		obj := typeextractors.FirstNamedChild(unwrapped, lang)
		field := typeextractors.LastNamedChild(unwrapped, lang)
		if obj != nil && obj.Type(lang) == "identifier" && field != nil && field.Type(lang) == "field_identifier" {
			objText := strings.TrimSpace(string(obj.Text(source)))
			fieldText := strings.TrimSpace(string(field.Text(source)))
			return []typeextractors.PendingAssignment{
				{Kind: typeextractors.PAKindFieldAccess, Lhs: *lhs, Receiver: objText, Field: fieldText},
			}
		}
	}
	// call_expression RHS → callResult (simple calls only)
	if unwrapped.Type(lang) == "call_expression" {
		funcNode := unwrapped.ChildByFieldName("function", lang)
		if funcNode != nil && funcNode.Type(lang) == "identifier" {
			callee := strings.TrimSpace(string(funcNode.Text(source)))
			return []typeextractors.PendingAssignment{{Kind: typeextractors.PAKindCallResult, Lhs: *lhs, Callee: callee}}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// extractRustPatternBinding — Rust: if let Some(x) = opt / if let Ok(x) = res
// Also handles match_arm patterns.
// Complements captured_pattern support in extractRustDeclaration.
// ---------------------------------------------------------------------------

func extractRustPatternBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string, declarationTypeNodes map[string]*gotreesitter.Node, scope string) *typeextractors.PatternBindingResult {
	var patternNode *gotreesitter.Node
	var valueNode *gotreesitter.Node

	if node.Type(lang) == "let_condition" {
		patternNode = node.ChildByFieldName("pattern", lang)
		valueNode = node.ChildByFieldName("value", lang)
	} else if node.Type(lang) == "match_arm" {
		// match_arm → pattern field is match_pattern wrapping the actual pattern
		matchPatternNode := node.ChildByFieldName("pattern", lang)
		if matchPatternNode != nil && matchPatternNode.Type(lang) == "match_pattern" {
			patternNode = typeextractors.FirstNamedChild(matchPatternNode, lang)
		} else {
			patternNode = matchPatternNode
		}
		// source variable is in the parent match_expression's 'value' field
		matchExpr := node.Parent()
		if matchExpr != nil {
			matchExpr = matchExpr.Parent() // match_arm → match_block → match_expression
		}
		if matchExpr != nil && matchExpr.Type(lang) == "match_expression" {
			valueNode = matchExpr.ChildByFieldName("value", lang)
		}
	}
	if patternNode == nil || valueNode == nil {
		return nil
	}

	// Only handle tuple_struct_pattern: Some(x) or Ok(x)
	if patternNode.Type(lang) != "tuple_struct_pattern" {
		return nil
	}

	// Extract the wrapper type name: Some | Ok | Err
	wrapperTypeNode := patternNode.ChildByFieldName("type", lang)
	if wrapperTypeNode == nil {
		return nil
	}
	wrapperName := typeextractors.ExtractSimpleTypeNameFromNode(wrapperTypeNode, source, lang, 0)
	if wrapperName == nil {
		return nil
	}
	if *wrapperName != "Some" && *wrapperName != "Ok" && *wrapperName != "Err" {
		return nil
	}

	// Extract the inner variable name from the tuple_struct_pattern
	var innerVar *string
	for i := 0; i < int(patternNode.NamedChildCount()); i++ {
		child := patternNode.NamedChild(i)
		if child == nil {
			continue
		}
		// Skip the type node itself
		if child == wrapperTypeNode {
			continue
		}
		if child.Type(lang) == "identifier" {
			t := strings.TrimSpace(string(child.Text(source)))
			innerVar = &t
			break
		}
	}
	if innerVar == nil {
		return nil
	}

	// The value must be a simple identifier so we can look it up in scopeEnv
	if valueNode.Type(lang) != "identifier" {
		return nil
	}
	sourceVarName := strings.TrimSpace(string(valueNode.Text(source)))

	// For Some(x): Option<T> is already unwrapped to T in scopeEnv (via NULLABLE_WRAPPER_TYPES).
	if *wrapperName == "Some" {
		if innerType, ok := scopeEnv[sourceVarName]; ok {
			return &typeextractors.PatternBindingResult{VarName: *innerVar, TypeName: innerType}
		}
		return nil
	}

	// wrapperName === 'Ok' or 'Err': look up the Result<T, E> type AST node.
	// Ok(x) → extract T (typeArgs[0]), Err(e) → extract E (typeArgs[1]).
	typeNodeKey := scope + "\x00" + sourceVarName
	typeAstNode, ok := declarationTypeNodes[typeNodeKey]
	if !ok {
		return nil
	}
	typeArgs := typeextractors.ExtractGenericTypeArgs(typeAstNode, source, lang, 0)
	argIndex := 0
	if *wrapperName == "Err" {
		argIndex = 1
	}
	if len(typeArgs) < argIndex+1 {
		return nil
	}
	return &typeextractors.PatternBindingResult{VarName: *innerVar, TypeName: typeArgs[argIndex]}
}

// ---------------------------------------------------------------------------
// For-loop helpers
// ---------------------------------------------------------------------------

// extractRustElementTypeFromTypeNode extracts element type from a Rust type annotation AST node.
// Handles: generic_type (Vec<User>), reference_type (&[User]), array_type ([User; N]), slice_type ([User]).
func extractRustElementTypeFromTypeNode(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	return extractRustElementTypeFromTypeNodeDepth(typeNode, source, lang, pos, 0)
}

func extractRustElementTypeFromTypeNodeDepth(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition, depth int) *string {
	if depth > 50 {
		return nil
	}
	// generic_type: Vec<User>, HashMap<K, V> — extract type arg based on position
	if typeNode.Type(lang) == "generic_type" {
		args := typeextractors.ExtractGenericTypeArgs(typeNode, source, lang, 0)
		if len(args) >= 1 {
			if pos == typeextractors.TypeArgFirst {
				return &args[0]
			}
			return &args[len(args)-1]
		}
	}
	// reference_type: &[User] or &Vec<User> — unwrap the reference and recurse
	if typeNode.Type(lang) == "reference_type" {
		inner := typeextractors.LastNamedChild(typeNode, lang)
		if inner != nil {
			return extractRustElementTypeFromTypeNodeDepth(inner, source, lang, pos, depth+1)
		}
	}
	// array_type: [User; N] — element is the first child
	if typeNode.Type(lang) == "array_type" {
		elemNode := typeextractors.FirstNamedChild(typeNode, lang)
		if elemNode != nil {
			return typeextractors.ExtractSimpleTypeNameFromNode(elemNode, source, lang, 0)
		}
	}
	return nil
}

// findRustParamElementType walks up from a for-loop to the enclosing function_item
// and searches parameters for one named `iterableName`.
func findRustParamElementType(iterableName string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	current := startNode.Parent()
	for current != nil {
		if current.Type(lang) == "function_item" {
			paramsNode := current.ChildByFieldName("parameters", lang)
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
					param := paramsNode.NamedChild(i)
					if param == nil || param.Type(lang) != "parameter" {
						continue
					}
					nameNode := param.ChildByFieldName("pattern", lang)
					if nameNode == nil {
						continue
					}
					// Unwrap reference patterns: &users, &mut users
					identNode := nameNode
					if identNode.Type(lang) == "reference_pattern" {
						inner := typeextractors.LastNamedChild(identNode, lang)
						if inner != nil {
							identNode = inner
						}
					}
					if identNode.Type(lang) == "mut_pattern" {
						inner := typeextractors.FirstNamedChild(identNode, lang)
						if inner != nil {
							identNode = inner
						}
					}
					if strings.TrimSpace(string(identNode.Text(source))) != iterableName {
						continue
					}
					typeNode := param.ChildByFieldName("type", lang)
					if typeNode != nil {
						return extractRustElementTypeFromTypeNode(typeNode, source, lang, pos)
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
// extractRustForLoopBinding — Rust: for user in &users
// Unwraps reference_expression (&users, &mut users) to get the iterable name.
// ---------------------------------------------------------------------------

func extractRustForLoopBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *typeextractors.ForLoopExtractorContext) {
	if node.Type(lang) != "for_expression" {
		return
	}

	patternNode := node.ChildByFieldName("pattern", lang)
	valueNode := node.ChildByFieldName("value", lang)
	if patternNode == nil || valueNode == nil {
		return
	}

	var iterableName *string
	var methodName *string
	var callExprElementType *string

	if valueNode.Type(lang) == "reference_expression" {
		inner := typeextractors.LastNamedChild(valueNode, lang)
		if inner != nil && inner.Type(lang) == "identifier" {
			t := strings.TrimSpace(string(inner.Text(source)))
			iterableName = &t
		}
	} else if valueNode.Type(lang) == "identifier" {
		t := strings.TrimSpace(string(valueNode.Text(source)))
		iterableName = &t
	} else if valueNode.Type(lang) == "field_expression" {
		prop := typeextractors.LastNamedChild(valueNode, lang)
		if prop != nil {
			t := strings.TrimSpace(string(prop.Text(source)))
			iterableName = &t
		}
	} else if valueNode.Type(lang) == "call_expression" {
		funcExpr := valueNode.ChildByFieldName("function", lang)
		if funcExpr != nil && funcExpr.Type(lang) == "field_expression" {
			// users.iter() → field_expression > identifier + field_identifier
			obj := typeextractors.FirstNamedChild(funcExpr, lang)
			if obj != nil && obj.Type(lang) == "identifier" {
				t := strings.TrimSpace(string(obj.Text(source)))
				iterableName = &t
			}
			field := typeextractors.LastNamedChild(funcExpr, lang)
			if field != nil && field.Type(lang) == "field_identifier" {
				t := strings.TrimSpace(string(field.Text(source)))
				methodName = &t
			}
		} else if funcExpr != nil && funcExpr.Type(lang) == "identifier" {
			// Direct function call: for user in get_users()
			calleeText := strings.TrimSpace(string(funcExpr.Text(source)))
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
			extractRustElementTypeFromTypeNode, findRustParamElementType, typeArgPos,
		)
	}
	if elementType == nil {
		return
	}

	loopVarName := typeextractors.ExtractVarName(patternNode, source, lang)
	if loopVarName != nil {
		ctx.ScopeEnv[*loopVarName] = *elementType
	}
}

// ---------------------------------------------------------------------------
// RustTypeConfig — the exported configuration variable
// ---------------------------------------------------------------------------

var RustTypeConfig = typeextractors.LanguageTypeConfig{
	DeclarationNodeTypes:    []string{"let_declaration", "let_condition"},
	ForLoopNodeTypes:        []string{"for_expression"},
	PatternBindingNodeTypes: []string{"let_condition", "match_arm"},
	ExtractDeclaration:       extractRustDeclaration,
	ExtractParameter:         extractRustParameter,
	ExtractInitializer:       extractRustInitializer,
	ScanConstructorBinding:   scanRustConstructorBinding,
	ExtractForLoopBinding:    extractRustForLoopBinding,
	ExtractPendingAssignment: extractRustPendingAssignment,
	ExtractPatternBinding:    extractRustPatternBinding,
}

// Ensure utils.FindChild is referenced (import anchor)
var _ = utils.FindChild