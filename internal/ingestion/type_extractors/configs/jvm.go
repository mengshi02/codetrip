// Package configs provides per-language type extraction configurations.
// This file implements the Java and Kotlin language type extractor configurations,
// ported from TS type-extractors/jvm.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// ═══════════════════════════════════════════════════════════════════════════════
// Java
// ═══════════════════════════════════════════════════════════════════════════════

// ---------------------------------------------------------------------------
// extractJavaDeclaration — Java: Type x = ...; Type x;
// ---------------------------------------------------------------------------

func extractJavaDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return
	}
	typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
	if typeName == nil || *typeName == "var" {
		return // skip Java 10 var — handled by extractInitializer
	}

	// Find variable_declarator children
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != "variable_declarator" {
			continue
		}
		nameNode := child.ChildByFieldName("name", lang)
		if nameNode != nil {
			varName := typeextractors.ExtractVarName(nameNode, source, lang)
			if varName != nil {
				env[*varName] = *typeName
			}
		}
	}
}

// ---------------------------------------------------------------------------
// extractJavaInitializer — Java 10+: var x = new User()
// ---------------------------------------------------------------------------

func extractJavaInitializer(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string, classNames typeextractors.ClassNameLookup) {
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
		// Skip declarators that already have a binding from extractDeclaration
		varName := typeextractors.ExtractVarName(nameNode, source, lang)
		if varName == nil {
			continue
		}
		if _, exists := env[*varName]; exists {
			continue
		}
		if valueNode.Type(lang) != "object_creation_expression" {
			continue
		}
		ctorType := valueNode.ChildByFieldName("type", lang)
		if ctorType == nil {
			continue
		}
		tn := typeextractors.ExtractSimpleTypeNameFromNode(ctorType, source, lang, 0)
		if tn != nil {
			env[*varName] = *tn
		}
	}
}

// ---------------------------------------------------------------------------
// extractJavaParameter — Java: formal_parameter → type name
// ---------------------------------------------------------------------------

func extractJavaParameter(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	var nameNode *gotreesitter.Node
	var typeNode *gotreesitter.Node

	if node.Type(lang) == "formal_parameter" {
		typeNode = node.ChildByFieldName("type", lang)
		nameNode = node.ChildByFieldName("name", lang)
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
// scanJavaConstructorBinding — Java: var x = SomeFactory.create()
// ---------------------------------------------------------------------------

func scanJavaConstructorBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *typeextractors.ConstructorBindingResult {
	if node.Type(lang) != "local_variable_declaration" {
		return nil
	}
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return nil
	}
	if strings.TrimSpace(string(typeNode.Text(source))) != "var" {
		return nil
	}
	declarator := typeextractors.FindChildByType(node, "variable_declarator", lang)
	if declarator == nil {
		return nil
	}
	nameNode := declarator.ChildByFieldName("name", lang)
	value := declarator.ChildByFieldName("value", lang)
	if nameNode == nil || value == nil {
		return nil
	}
	if value.Type(lang) == "object_creation_expression" {
		return nil
	}
	if value.Type(lang) != "method_invocation" {
		return nil
	}
	methodName := value.ChildByFieldName("name", lang)
	if methodName == nil {
		return nil
	}
	return &typeextractors.ConstructorBindingResult{
		VarName:    strings.TrimSpace(string(nameNode.Text(source))),
		CalleeName: strings.TrimSpace(string(methodName.Text(source))),
	}
}

// ---------------------------------------------------------------------------
// Java for-loop element type helpers
// ---------------------------------------------------------------------------

// extractJavaElementTypeFromTypeNode extracts element type from a Java type annotation AST node.
// Handles generic_type (List<User>), array_type (User[]).
func extractJavaElementTypeFromTypeNode(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	if typeNode.Type(lang) == "generic_type" {
		args := typeextractors.ExtractGenericTypeArgs(typeNode, source, lang, 0)
		if len(args) >= 1 {
			if pos == typeextractors.TypeArgFirst {
				return &args[0]
			}
			return &args[len(args)-1]
		}
	}
	if typeNode.Type(lang) == "array_type" {
		elemNode := typeextractors.FirstNamedChild(typeNode, lang)
		if elemNode != nil {
			return typeextractors.ExtractSimpleTypeNameFromNode(elemNode, source, lang, 0)
		}
	}
	return nil
}

// findJavaParamElementType walks up from a for-each to the enclosing method_declaration and searches parameters.
func findJavaParamElementType(iterableName string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	current := startNode.Parent()
	for current != nil {
		if current.Type(lang) == "method_declaration" || current.Type(lang) == "constructor_declaration" {
			paramsNode := current.ChildByFieldName("parameters", lang)
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
					param := paramsNode.NamedChild(i)
					if param == nil || param.Type(lang) != "formal_parameter" {
						continue
					}
					nameNode := param.ChildByFieldName("name", lang)
					if nameNode == nil || strings.TrimSpace(string(nameNode.Text(source))) != iterableName {
						continue
					}
					typeNode := param.ChildByFieldName("type", lang)
					if typeNode != nil {
						return extractJavaElementTypeFromTypeNode(typeNode, source, lang, pos)
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
// extractJavaForLoopBinding — Java: for (User user : users)
// ---------------------------------------------------------------------------

func extractJavaForLoopBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *typeextractors.ForLoopExtractorContext) {
	typeNode := node.ChildByFieldName("type", lang)
	nameNode := node.ChildByFieldName("name", lang)
	if typeNode == nil || nameNode == nil {
		return
	}
	varName := typeextractors.ExtractVarName(nameNode, source, lang)
	if varName == nil {
		return
	}

	// Explicit type: for (User user : users)
	typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
	if typeName != nil && *typeName != "var" {
		ctx.ScopeEnv[*varName] = *typeName
		return
	}

	// Tier 1c: var — resolve from iterable's container type
	iterableNode := node.ChildByFieldName("value", lang)
	if iterableNode == nil {
		return
	}

	var iterableName *string
	var methodName *string
	var callExprElementType *string

	if iterableNode.Type(lang) == "identifier" {
		t := strings.TrimSpace(string(iterableNode.Text(source)))
		iterableName = &t
	} else if iterableNode.Type(lang) == "field_access" {
		field := iterableNode.ChildByFieldName("field", lang)
		if field != nil {
			t := strings.TrimSpace(string(field.Text(source)))
			iterableName = &t
		}
	} else if iterableNode.Type(lang) == "method_invocation" {
		obj := iterableNode.ChildByFieldName("object", lang)
		name := iterableNode.ChildByFieldName("name", lang)
		if obj != nil && obj.Type(lang) == "identifier" {
			t := strings.TrimSpace(string(obj.Text(source)))
			iterableName = &t
		} else if obj != nil && obj.Type(lang) == "field_access" {
			innerField := obj.ChildByFieldName("field", lang)
			if innerField != nil {
				t := strings.TrimSpace(string(innerField.Text(source)))
				iterableName = &t
			}
		} else if obj == nil && name != nil {
			// Direct function call: for (var u : getUsers())
			if ctx.ReturnTypeLookup != nil {
				rawReturn := ctx.ReturnTypeLookup.LookupRawReturnType(strings.TrimSpace(string(name.Text(source))))
				if rawReturn != nil {
					el := typeextractors.ExtractElementTypeFromString(*rawReturn, typeextractors.TypeArgLast)
					callExprElementType = el
				}
			}
		}
		if name != nil {
			t := strings.TrimSpace(string(name.Text(source)))
			methodName = &t
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
		methodNameStr := ""
		if methodName != nil {
			methodNameStr = *methodName
		}
		typeArgPos := typeextractors.MethodToTypeArgPosition(methodNameStr, containerTypeName)

		extractFromTypeNode := func(typeNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
			return extractJavaElementTypeFromTypeNode(typeNd, src, l, pos)
		}
		findParamElementType := func(name string, startNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
			return findJavaParamElementType(name, startNd, src, l, pos)
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
// extractJavaPendingAssignment — Java: var alias = u
// ---------------------------------------------------------------------------

func extractJavaPendingAssignment(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []typeextractors.PendingAssignment {
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
		lhs := strings.TrimSpace(string(nameNode.Text(source)))
		if _, exists := scopeEnv[lhs]; exists {
			continue
		}
		// identifier / simple_identifier RHS → copy
		if valueNode.Type(lang) == "identifier" || valueNode.Type(lang) == "simple_identifier" {
			return []typeextractors.PendingAssignment{{
				Kind: typeextractors.PAKindCopy,
				Lhs:  lhs,
				Rhs:  strings.TrimSpace(string(valueNode.Text(source))),
			}}
		}
		// field_access RHS → fieldAccess (a.field)
		if valueNode.Type(lang) == "field_access" {
			obj := valueNode.ChildByFieldName("object", lang)
			field := valueNode.ChildByFieldName("field", lang)
			if obj != nil && obj.Type(lang) == "identifier" && field != nil {
				return []typeextractors.PendingAssignment{{
					Kind:     typeextractors.PAKindFieldAccess,
					Lhs:      lhs,
					Receiver: strings.TrimSpace(string(obj.Text(source))),
					Field:    strings.TrimSpace(string(field.Text(source))),
				}}
			}
		}
		// method_invocation RHS
		if valueNode.Type(lang) == "method_invocation" {
			objField := valueNode.ChildByFieldName("object", lang)
			if objField == nil {
				// No receiver → callResult
				nameField := valueNode.ChildByFieldName("name", lang)
				if nameField != nil && nameField.Type(lang) == "identifier" {
					return []typeextractors.PendingAssignment{{
						Kind:   typeextractors.PAKindCallResult,
						Lhs:    lhs,
						Callee: strings.TrimSpace(string(nameField.Text(source))),
					}}
				}
			} else if objField.Type(lang) == "identifier" {
				// With receiver → methodCallResult
				nameField := valueNode.ChildByFieldName("name", lang)
				if nameField != nil && nameField.Type(lang) == "identifier" {
					return []typeextractors.PendingAssignment{{
						Kind:     typeextractors.PAKindMethodCallResult,
						Lhs:      lhs,
						Receiver: strings.TrimSpace(string(objField.Text(source))),
						Method:   strings.TrimSpace(string(nameField.Text(source))),
					}}
				}
			}
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// extractJavaPatternBinding — Java 16+ instanceof pattern variable
// ---------------------------------------------------------------------------

func extractJavaPatternBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string, declarationTypeNodes map[string]*gotreesitter.Node, scope string) *typeextractors.PatternBindingResult {
	if node.Type(lang) == "type_pattern" {
		// Java 17+ switch pattern: case User u -> ...
		// type_pattern has positional children (NO named fields):
		//   namedChild(0) = type (type_identifier, e.g., User)
		//   namedChild(1) = identifier (e.g., u)
		var typeNd *gotreesitter.Node
		var nameNd *gotreesitter.Node
		if node.NamedChildCount() >= 1 {
			typeNd = node.NamedChild(0)
		}
		if node.NamedChildCount() >= 2 {
			nameNd = node.NamedChild(1)
		}
		if typeNd == nil || nameNd == nil {
			return nil
		}
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNd, source, lang, 0)
		varName := typeextractors.ExtractVarName(nameNd, source, lang)
		if typeName == nil || varName == nil {
			return nil
		}
		return &typeextractors.PatternBindingResult{
			VarName:  *varName,
			TypeName: *typeName,
		}
	}
	if node.Type(lang) != "instanceof_expression" {
		return nil
	}
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode == nil {
		return nil
	}
	typeNode := node.ChildByFieldName("right", lang)
	if typeNode == nil {
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

// ---------------------------------------------------------------------------
// inferJvmLiteralType — Infer literal type for Java/Kotlin overload disambiguation
// ---------------------------------------------------------------------------

func inferJvmLiteralType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	nodeType := node.Type(lang)
	switch nodeType {
	case "decimal_integer_literal", "integer_literal", "hex_integer_literal",
		"octal_integer_literal", "binary_integer_literal":
		text := strings.TrimSpace(string(node.Text(source)))
		if strings.HasSuffix(text, "L") || strings.HasSuffix(text, "l") {
			s := "long"
			return &s
		}
		s := "int"
		return &s
	case "decimal_floating_point_literal", "real_literal":
		text := strings.TrimSpace(string(node.Text(source)))
		if strings.HasSuffix(text, "f") || strings.HasSuffix(text, "F") {
			s := "float"
			return &s
		}
		s := "double"
		return &s
	case "string_literal", "line_string_literal", "multi_line_string_literal":
		s := "String"
		return &s
	case "character_literal":
		s := "char"
		return &s
	case "true", "false", "boolean_literal":
		s := "boolean"
		return &s
	case "null_literal":
		s := "null"
		return &s
	default:
		return nil
	}
}

// ---------------------------------------------------------------------------
// JavaTypeConfig
// ---------------------------------------------------------------------------

// JavaTypeConfig is the per-language type extraction configuration for Java.
var JavaTypeConfig = typeextractors.LanguageTypeConfig{
	DeclarationNodeTypes: []string{
		"local_variable_declaration",
		"field_declaration",
	},
	ForLoopNodeTypes: []string{"enhanced_for_statement"},
	PatternBindingNodeTypes: []string{
		"instanceof_expression",
		"type_pattern",
	},
	ExtractDeclaration:       extractJavaDeclaration,
	ExtractParameter:         extractJavaParameter,
	ExtractInitializer:       extractJavaInitializer,
	ScanConstructorBinding:   scanJavaConstructorBinding,
	ExtractForLoopBinding:    extractJavaForLoopBinding,
	ExtractPendingAssignment: extractJavaPendingAssignment,
	ExtractPatternBinding:    extractJavaPatternBinding,
	InferLiteralType:         inferJvmLiteralType,
}

// ═══════════════════════════════════════════════════════════════════════════════
// Kotlin
// ═══════════════════════════════════════════════════════════════════════════════

// ---------------------------------------------------------------------------
// extractKotlinDeclaration — Kotlin: val x: Foo = ...
// ---------------------------------------------------------------------------

func extractKotlinDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	if node.Type(lang) == "property_declaration" {
		// Kotlin property_declaration: name/type are inside a variable_declaration child
		varDecl := typeextractors.FindChildByType(node, "variable_declaration", lang)
		if varDecl != nil {
			nameNode := typeextractors.FindChildByType(varDecl, "simple_identifier", lang)
			typeNode := typeextractors.FindChildByType(varDecl, "user_type", lang)
			if typeNode == nil {
				typeNode = typeextractors.FindChildByType(varDecl, "nullable_type", lang)
			}
			if nameNode == nil || typeNode == nil {
				return
			}
			varName := typeextractors.ExtractVarName(nameNode, source, lang)
			typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
			if varName != nil && typeName != nil {
				env[*varName] = *typeName
			}
			return
		}
		// Fallback: try direct fields
		nameNode := node.ChildByFieldName("name", lang)
		if nameNode == nil {
			nameNode = typeextractors.FindChildByType(node, "simple_identifier", lang)
		}
		typeNode := node.ChildByFieldName("type", lang)
		if typeNode == nil {
			typeNode = typeextractors.FindChildByType(node, "user_type", lang)
		}
		if nameNode == nil || typeNode == nil {
			return
		}
		varName := typeextractors.ExtractVarName(nameNode, source, lang)
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
		if varName != nil && typeName != nil {
			env[*varName] = *typeName
		}
	} else if node.Type(lang) == "variable_declaration" {
		// variable_declaration directly inside functions
		nameNode := typeextractors.FindChildByType(node, "simple_identifier", lang)
		typeNode := typeextractors.FindChildByType(node, "user_type", lang)
		if nameNode != nil && typeNode != nil {
			varName := typeextractors.ExtractVarName(nameNode, source, lang)
			typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
			if varName != nil && typeName != nil {
				env[*varName] = *typeName
			}
		}
	}
}

// ---------------------------------------------------------------------------
// extractKotlinParameter — Kotlin: parameter / formal_parameter → type name
// ---------------------------------------------------------------------------

func extractKotlinParameter(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	var nameNode *gotreesitter.Node
	var typeNode *gotreesitter.Node

	if node.Type(lang) == "formal_parameter" {
		typeNode = node.ChildByFieldName("type", lang)
		nameNode = node.ChildByFieldName("name", lang)
	} else {
		nameNode = node.ChildByFieldName("name", lang)
		if nameNode == nil {
			nameNode = node.ChildByFieldName("pattern", lang)
		}
		typeNode = node.ChildByFieldName("type", lang)
	}

	// Fallback: Kotlin `parameter` nodes use positional children, not named fields
	if nameNode == nil {
		nameNode = typeextractors.FindChildByType(node, "simple_identifier", lang)
	}
	if typeNode == nil {
		typeNode = typeextractors.FindChildByType(node, "user_type", lang)
		if typeNode == nil {
			typeNode = typeextractors.FindChildByType(node, "nullable_type", lang)
		}
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
// findKotlinConstructorCallee — Find constructor callee name in Kotlin property_declaration
// ---------------------------------------------------------------------------

func findKotlinConstructorCallee(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, classNames typeextractors.ClassNameLookup) *string {
	if node.Type(lang) != "property_declaration" {
		return nil
	}
	value := node.ChildByFieldName("value", lang)
	if value == nil {
		value = typeextractors.FindChildByType(node, "call_expression", lang)
	}
	if value == nil || value.Type(lang) != "call_expression" {
		return nil
	}
	callee := typeextractors.FirstNamedChild(value, lang)
	if callee == nil || callee.Type(lang) != "simple_identifier" {
		return nil
	}
	calleeName := strings.TrimSpace(string(callee.Text(source)))
	if calleeName == "" || !classNames.Has(calleeName) {
		return nil
	}
	return &calleeName
}

// ---------------------------------------------------------------------------
// extractKotlinInitializer — Kotlin: val user = User()
// ---------------------------------------------------------------------------

func extractKotlinInitializer(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string, classNames typeextractors.ClassNameLookup) {
	// Skip if there's an explicit type annotation — Tier 0 already handled it
	varDecl := typeextractors.FindChildByType(node, "variable_declaration", lang)
	if varDecl != nil && typeextractors.FindChildByType(varDecl, "user_type", lang) != nil {
		return
	}

	calleeName := findKotlinConstructorCallee(node, source, lang, classNames)
	if calleeName == nil {
		return
	}

	// Extract the variable name from the variable_declaration inside property_declaration
	var nameNode *gotreesitter.Node
	if varDecl != nil {
		nameNode = typeextractors.FindChildByType(varDecl, "simple_identifier", lang)
	} else {
		nameNode = typeextractors.FindChildByType(node, "simple_identifier", lang)
	}
	if nameNode == nil {
		return
	}

	varName := typeextractors.ExtractVarName(nameNode, source, lang)
	if varName != nil {
		env[*varName] = *calleeName
	}
}

// ---------------------------------------------------------------------------
// detectKotlinConstructorType — Kotlin: detect constructor type from call_expression
// ---------------------------------------------------------------------------

func detectKotlinConstructorType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, classNames typeextractors.ClassNameLookup) *string {
	return findKotlinConstructorCallee(node, source, lang, classNames)
}

// ---------------------------------------------------------------------------
// scanKotlinConstructorBinding — Kotlin: val x = User(...)
// ---------------------------------------------------------------------------

func scanKotlinConstructorBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *typeextractors.ConstructorBindingResult {
	if node.Type(lang) != "property_declaration" {
		return nil
	}
	varDecl := typeextractors.FindChildByType(node, "variable_declaration", lang)
	if varDecl == nil {
		return nil
	}
	if typeextractors.FindChildByType(varDecl, "user_type", lang) != nil {
		return nil
	}
	callExpr := typeextractors.FindChildByType(node, "call_expression", lang)
	if callExpr == nil {
		return nil
	}
	callee := typeextractors.FirstNamedChild(callExpr, lang)
	if callee == nil {
		return nil
	}

	var calleeName *string
	if callee.Type(lang) == "simple_identifier" {
		t := strings.TrimSpace(string(callee.Text(source)))
		calleeName = &t
	} else if callee.Type(lang) == "navigation_expression" {
		// Extract method name from qualified call: service.getUser() → getUser
		suffix := typeextractors.LastNamedChild(callee, lang)
		if suffix != nil && suffix.Type(lang) == "navigation_suffix" {
			methodName := typeextractors.LastNamedChild(suffix, lang)
			if methodName != nil && methodName.Type(lang) == "simple_identifier" {
				t := strings.TrimSpace(string(methodName.Text(source)))
				calleeName = &t
			}
		}
	}
	if calleeName == nil {
		return nil
	}
	nameNode := typeextractors.FindChildByType(varDecl, "simple_identifier", lang)
	if nameNode == nil {
		return nil
	}
	return &typeextractors.ConstructorBindingResult{
		VarName:    strings.TrimSpace(string(nameNode.Text(source))),
		CalleeName: *calleeName,
	}
}

// ---------------------------------------------------------------------------
// Kotlin for-loop element type helpers
// ---------------------------------------------------------------------------

// extractKotlinElementTypeFromTypeNode extracts element type from a Kotlin type annotation AST node.
// Kotlin: user_type → [type_identifier, type_arguments → [type_projection → user_type]]
// Handles the type_projection wrapper that Kotlin uses for generic type arguments.
func extractKotlinElementTypeFromTypeNode(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	if typeNode.Type(lang) == "user_type" {
		argsNode := typeextractors.FindChildByType(typeNode, "type_arguments", lang)
		if argsNode != nil && argsNode.NamedChildCount() >= 1 {
			var targetArg *gotreesitter.Node
			if pos == typeextractors.TypeArgFirst {
				targetArg = argsNode.NamedChild(0)
			} else {
				targetArg = argsNode.NamedChild(argsNode.NamedChildCount() - 1)
			}
			if targetArg == nil {
				return nil
			}
			// Kotlin wraps type args in type_projection — unwrap to get the inner type
			inner := targetArg
			if targetArg.Type(lang) == "type_projection" {
				inner = typeextractors.FirstNamedChild(targetArg, lang)
			}
			if inner != nil {
				return typeextractors.ExtractSimpleTypeNameFromNode(inner, source, lang, 0)
			}
		}
	}
	return nil
}

// findKotlinParamElementType walks up from a for-loop to the enclosing function_declaration and searches parameters.
// Kotlin parameters use positional children (simple_identifier, user_type), not named fields.
func findKotlinParamElementType(iterableName string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	current := startNode.Parent()
	for current != nil {
		if current.Type(lang) == "function_declaration" {
			paramsNode := typeextractors.FindChildByType(current, "function_value_parameters", lang)
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
					param := paramsNode.NamedChild(i)
					if param == nil || param.Type(lang) != "parameter" {
						continue
					}
					nameNode := typeextractors.FindChildByType(param, "simple_identifier", lang)
					if nameNode == nil || strings.TrimSpace(string(nameNode.Text(source))) != iterableName {
						continue
					}
					typeNode := typeextractors.FindChildByType(param, "user_type", lang)
					if typeNode != nil {
						return extractKotlinElementTypeFromTypeNode(typeNode, source, lang, pos)
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
// extractKotlinForLoopBinding — Kotlin: for (user: User in users)
// ---------------------------------------------------------------------------

func extractKotlinForLoopBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *typeextractors.ForLoopExtractorContext) {
	varDecl := typeextractors.FindChildByType(node, "variable_declaration", lang)
	if varDecl == nil {
		return
	}
	nameNode := typeextractors.FindChildByType(varDecl, "simple_identifier", lang)
	if nameNode == nil {
		return
	}
	varName := typeextractors.ExtractVarName(nameNode, source, lang)
	if varName == nil {
		return
	}

	// Explicit type annotation: for (user: User in users)
	typeNode := typeextractors.FindChildByType(varDecl, "user_type", lang)
	if typeNode != nil {
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
		if typeName != nil {
			ctx.ScopeEnv[*varName] = *typeName
		}
		return
	}

	// Tier 1c: no annotation — resolve from iterable's container type
	// Kotlin for-loop children: [variable_declaration, iterable_expr, control_structure_body]
	// The iterable is the second named child of the for_statement (after variable_declaration)
	var iterableName *string
	var methodName *string
	var fallbackIterableName *string
	var callExprElementType *string
	foundVarDecl := false

	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == varDecl {
			foundVarDecl = true
			continue
		}
		if !foundVarDecl || child == nil {
			continue
		}
		if child.Type(lang) == "simple_identifier" {
			t := strings.TrimSpace(string(child.Text(source)))
			iterableName = &t
			break
		}
		if child.Type(lang) == "navigation_expression" {
			// data.keys → navigation_expression > simple_identifier(data) + navigation_suffix > simple_identifier(keys)
			obj := typeextractors.FirstNamedChild(child, lang)
			suffix := typeextractors.FindChildByType(child, "navigation_suffix", lang)
			var prop *gotreesitter.Node
			hasCallSuffix := false
			if suffix != nil {
				prop = typeextractors.FindChildByType(suffix, "simple_identifier", lang)
				if typeextractors.FindChildByType(suffix, "call_suffix", lang) != nil {
					hasCallSuffix = true
				}
			}
			if obj != nil && obj.Type(lang) == "simple_identifier" {
				t := strings.TrimSpace(string(obj.Text(source)))
				iterableName = &t
			}
			if prop != nil {
				t := strings.TrimSpace(string(prop.Text(source)))
				methodName = &t
			}
			if !hasCallSuffix && prop != nil {
				t := strings.TrimSpace(string(prop.Text(source)))
				fallbackIterableName = &t
			}
			break
		}
		if child.Type(lang) == "call_expression" {
			// data.values() → call_expression > navigation_expression > simple_identifier + navigation_suffix
			callee := typeextractors.FirstNamedChild(child, lang)
			if callee != nil && callee.Type(lang) == "navigation_expression" {
				obj := typeextractors.FirstNamedChild(callee, lang)
				if obj != nil && obj.Type(lang) == "simple_identifier" {
					t := strings.TrimSpace(string(obj.Text(source)))
					iterableName = &t
				}
				suffix := typeextractors.FindChildByType(callee, "navigation_suffix", lang)
				if suffix != nil {
					prop := typeextractors.FindChildByType(suffix, "simple_identifier", lang)
					if prop != nil {
						t := strings.TrimSpace(string(prop.Text(source)))
						methodName = &t
					}
				}
			} else if callee != nil && callee.Type(lang) == "simple_identifier" {
				// Direct function call: for (u in getUsers())
				if ctx.ReturnTypeLookup != nil {
					rawReturn := ctx.ReturnTypeLookup.LookupRawReturnType(strings.TrimSpace(string(callee.Text(source))))
					if rawReturn != nil {
						el := typeextractors.ExtractElementTypeFromString(*rawReturn, typeextractors.TypeArgLast)
						callExprElementType = el
					}
				}
			}
			break
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
		// Fallback: if object has no type in scope, try the property as the iterable name
		if containerTypeName == "" && fallbackIterableName != nil {
			iterableName = fallbackIterableName
			methodName = nil
			containerTypeName = ctx.ScopeEnv[*iterableName]
		}
		methodNameStr := ""
		if methodName != nil {
			methodNameStr = *methodName
		}
		typeArgPos := typeextractors.MethodToTypeArgPosition(methodNameStr, containerTypeName)

		extractFromTypeNode := func(typeNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
			return extractKotlinElementTypeFromTypeNode(typeNd, src, l, pos)
		}
		findParamElementType := func(name string, startNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
			return findKotlinParamElementType(name, startNd, src, l, pos)
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
// findAncestorByType — Walk up from a node to find an ancestor of a given type
// ---------------------------------------------------------------------------

func findAncestorByType(node *gotreesitter.Node, ancestorType string, lang *gotreesitter.Language) *gotreesitter.Node {
	current := node.Parent()
	for current != nil {
		if current.Type(lang) == ancestorType {
			return current
		}
		current = current.Parent()
	}
	return nil
}

// ---------------------------------------------------------------------------
// extractKotlinPendingAssignment — Kotlin: val alias = u
// ---------------------------------------------------------------------------

func extractKotlinPendingAssignment(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []typeextractors.PendingAssignment {
	if node.Type(lang) == "property_declaration" {
		// Find the variable name from variable_declaration child
		varDecl := typeextractors.FindChildByType(node, "variable_declaration", lang)
		if varDecl == nil {
			return nil
		}
		nameNode := typeextractors.FirstNamedChild(varDecl, lang)
		if nameNode == nil || nameNode.Type(lang) != "simple_identifier" {
			return nil
		}
		lhs := strings.TrimSpace(string(nameNode.Text(source)))
		if _, exists := scopeEnv[lhs]; exists {
			return nil
		}
		// Find the RHS after the "=" token
		foundEq := false
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			if child.Type(lang) == "=" {
				foundEq = true
				continue
			}
			if foundEq && child.Type(lang) == "simple_identifier" {
				return []typeextractors.PendingAssignment{{
					Kind: typeextractors.PAKindCopy,
					Lhs:  lhs,
					Rhs:  strings.TrimSpace(string(child.Text(source))),
				}}
			}
			// navigation_expression RHS → fieldAccess (a.field)
			if foundEq && child.Type(lang) == "navigation_expression" {
				recv := typeextractors.FirstNamedChild(child, lang)
				suffix := typeextractors.LastNamedChild(child, lang)
				fieldNode := suffix
				if suffix != nil && suffix.Type(lang) == "navigation_suffix" {
					fieldNode = typeextractors.LastNamedChild(suffix, lang)
				}
				if recv != nil && recv.Type(lang) == "simple_identifier" && fieldNode != nil && fieldNode.Type(lang) == "simple_identifier" {
					return []typeextractors.PendingAssignment{{
						Kind:     typeextractors.PAKindFieldAccess,
						Lhs:      lhs,
						Receiver: strings.TrimSpace(string(recv.Text(source))),
						Field:    strings.TrimSpace(string(fieldNode.Text(source))),
					}}
				}
			}
			// call_expression RHS
			if foundEq && child.Type(lang) == "call_expression" {
				calleeNode := typeextractors.FirstNamedChild(child, lang)
				if calleeNode != nil && calleeNode.Type(lang) == "simple_identifier" {
					return []typeextractors.PendingAssignment{{
						Kind:   typeextractors.PAKindCallResult,
						Lhs:    lhs,
						Callee: strings.TrimSpace(string(calleeNode.Text(source))),
					}}
				}
				// navigation_expression callee → methodCallResult (a.method())
				if calleeNode != nil && calleeNode.Type(lang) == "navigation_expression" {
					recv := typeextractors.FirstNamedChild(calleeNode, lang)
					suffix := typeextractors.LastNamedChild(calleeNode, lang)
					methodNode := suffix
					if suffix != nil && suffix.Type(lang) == "navigation_suffix" {
						methodNode = typeextractors.LastNamedChild(suffix, lang)
					}
					if recv != nil && recv.Type(lang) == "simple_identifier" && methodNode != nil && methodNode.Type(lang) == "simple_identifier" {
						return []typeextractors.PendingAssignment{{
							Kind:     typeextractors.PAKindMethodCallResult,
							Lhs:      lhs,
							Receiver: strings.TrimSpace(string(recv.Text(source))),
							Method:   strings.TrimSpace(string(methodNode.Text(source))),
						}}
					}
				}
			}
		}
		return nil
	}

	if node.Type(lang) == "variable_declaration" {
		// variable_declaration directly inside functions: simple_identifier children
		nameNode := typeextractors.FindChildByType(node, "simple_identifier", lang)
		if nameNode == nil {
			return nil
		}
		lhs := strings.TrimSpace(string(nameNode.Text(source)))
		if _, exists := scopeEnv[lhs]; exists {
			return nil
		}
		// Look for RHS after "=" in the parent (property_declaration)
		parent := node.Parent()
		if parent == nil {
			return nil
		}
		foundEq := false
		for i := 0; i < int(parent.ChildCount()); i++ {
			child := parent.Child(i)
			if child == nil {
				continue
			}
			if child.Type(lang) == "=" {
				foundEq = true
				continue
			}
			if foundEq && child.Type(lang) == "simple_identifier" {
				return []typeextractors.PendingAssignment{{
					Kind: typeextractors.PAKindCopy,
					Lhs:  lhs,
					Rhs:  strings.TrimSpace(string(child.Text(source))),
				}}
			}
			if foundEq && child.Type(lang) == "navigation_expression" {
				recv := typeextractors.FirstNamedChild(child, lang)
				suffix := typeextractors.LastNamedChild(child, lang)
				fieldNode := suffix
				if suffix != nil && suffix.Type(lang) == "navigation_suffix" {
					fieldNode = typeextractors.LastNamedChild(suffix, lang)
				}
				if recv != nil && recv.Type(lang) == "simple_identifier" && fieldNode != nil && fieldNode.Type(lang) == "simple_identifier" {
					return []typeextractors.PendingAssignment{{
						Kind:     typeextractors.PAKindFieldAccess,
						Lhs:      lhs,
						Receiver: strings.TrimSpace(string(recv.Text(source))),
						Field:    strings.TrimSpace(string(fieldNode.Text(source))),
					}}
				}
			}
			if foundEq && child.Type(lang) == "call_expression" {
				calleeNode := typeextractors.FirstNamedChild(child, lang)
				if calleeNode != nil && calleeNode.Type(lang) == "simple_identifier" {
					return []typeextractors.PendingAssignment{{
						Kind:   typeextractors.PAKindCallResult,
						Lhs:    lhs,
						Callee: strings.TrimSpace(string(calleeNode.Text(source))),
					}}
				}
				if calleeNode != nil && calleeNode.Type(lang) == "navigation_expression" {
					recv := typeextractors.FirstNamedChild(calleeNode, lang)
					suffix := typeextractors.LastNamedChild(calleeNode, lang)
					methodNode := suffix
					if suffix != nil && suffix.Type(lang) == "navigation_suffix" {
						methodNode = typeextractors.LastNamedChild(suffix, lang)
					}
					if recv != nil && recv.Type(lang) == "simple_identifier" && methodNode != nil && methodNode.Type(lang) == "simple_identifier" {
						return []typeextractors.PendingAssignment{{
							Kind:     typeextractors.PAKindMethodCallResult,
							Lhs:      lhs,
							Receiver: strings.TrimSpace(string(recv.Text(source))),
							Method:   strings.TrimSpace(string(methodNode.Text(source))),
						}}
					}
				}
			}
		}
		return nil
	}

	return nil
}

// ---------------------------------------------------------------------------
// extractKotlinPatternBinding — Kotlin when/is smart casts + null-check narrowing
// ---------------------------------------------------------------------------

func extractKotlinPatternBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string, declarationTypeNodes map[string]*gotreesitter.Node, scope string) *typeextractors.PatternBindingResult {
	// Kotlin when/is smart casts
	if node.Type(lang) == "type_test" {
		typeNd := typeextractors.LastNamedChild(node, lang)
		if typeNd == nil {
			return nil
		}
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNd, source, lang, 0)
		if typeName == nil {
			return nil
		}
		whenExpr := findAncestorByType(node, "when_expression", lang)
		if whenExpr == nil {
			return nil
		}
		var whenSubject *gotreesitter.Node
		if whenExpr.NamedChildCount() >= 1 {
			whenSubject = whenExpr.NamedChild(0)
		}
		subject := whenSubject
		if whenSubject != nil {
			first := typeextractors.FirstNamedChild(whenSubject, lang)
			if first != nil {
				subject = first
			}
		}
		if subject == nil {
			return nil
		}
		varName := typeextractors.ExtractVarName(subject, source, lang)
		if varName == nil {
			return nil
		}
		return &typeextractors.PatternBindingResult{
			VarName:  *varName,
			TypeName: *typeName,
		}
	}

	// Null-check narrowing: if (x != null) { ... }
	if node.Type(lang) == "equality_expression" {
		// Check for != operator
		hasNeq := false
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c != nil && !c.IsNamed() && strings.TrimSpace(string(c.Text(source))) == "!=" {
				hasNeq = true
				break
			}
		}
		if !hasNeq {
			return nil
		}

		// Scan for variable and null
		var varNode *gotreesitter.Node
		hasNull := false
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c == nil {
				continue
			}
			if c.Type(lang) == "simple_identifier" {
				varNode = c
			}
			if !c.IsNamed() && strings.TrimSpace(string(c.Text(source))) == "null" {
				hasNull = true
			}
		}
		if varNode == nil || !hasNull {
			return nil
		}

		varName := strings.TrimSpace(string(varNode.Text(source)))
		resolvedType, ok := scopeEnv[varName]
		if !ok {
			return nil
		}

		// Check if the original declaration type was nullable (ends with ?)
		key := scope + "\x00" + varName
		declTypeNode := declarationTypeNodes[key]
		if declTypeNode == nil {
			return nil
		}
		declText := strings.TrimSpace(string(declTypeNode.Text(source)))
		if !strings.Contains(declText, "?") && !strings.Contains(declText, "null") {
			return nil
		}

		// Find the if-body: walk up to if_expression, then find control_structure_body
		ifExpr := findAncestorByType(node, "if_expression", lang)
		if ifExpr == nil {
			return nil
		}
		for i := 0; i < int(ifExpr.ChildCount()); i++ {
			child := ifExpr.Child(i)
			if child != nil && child.Type(lang) == "control_structure_body" {
				return &typeextractors.PatternBindingResult{
					VarName:  varName,
					TypeName: resolvedType,
					NarrowingRange: &typeextractors.NarrowingRange{
						StartIndex: child.StartByte(),
						EndIndex:   child.EndByte(),
					},
				}
			}
		}
		return nil
	}

	return nil
}

// ---------------------------------------------------------------------------
// KotlinTypeConfig — getDeclarationTypeNode
// ---------------------------------------------------------------------------

// kotlinGetDeclarationTypeNode locates the type-annotation node for a Kotlin declaration.
func kotlinGetDeclarationTypeNode(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	// Kotlin property_declaration wraps the actual declaration in variable_declaration.
	// The type is commonly a user_type / nullable_type child (positional, not 'type' field).
	varDecl := node
	if node.Type(lang) == "property_declaration" {
		vd := typeextractors.FindChildByType(node, "variable_declaration", lang)
		if vd != nil {
			varDecl = vd
		}
	}
	tn := varDecl.ChildByFieldName("type", lang)
	if tn == nil {
		tn = typeextractors.FindChildByType(varDecl, "user_type", lang)
	}
	if tn == nil {
		tn = typeextractors.FindChildByType(varDecl, "nullable_type", lang)
	}
	if tn != nil {
		return tn
	}
	// Final fallback on the node itself
	tn = node.ChildByFieldName("type", lang)
	if tn == nil {
		tn = typeextractors.FindChildByType(node, "user_type", lang)
	}
	return tn
}

// ---------------------------------------------------------------------------
// KotlinTypeConfig
// ---------------------------------------------------------------------------

// KotlinTypeConfig is the per-language type extraction configuration for Kotlin.
var KotlinTypeConfig = typeextractors.LanguageTypeConfig{
	AllowPatternBindingOverwrite: true,
	DeclarationNodeTypes: []string{
		"property_declaration",
		"variable_declaration",
	},
	GetDeclarationTypeNode: kotlinGetDeclarationTypeNode,
	ForLoopNodeTypes:       []string{"for_statement"},
	PatternBindingNodeTypes: []string{
		"type_test",
		"equality_expression",
	},
	ExtractDeclaration:       extractKotlinDeclaration,
	ExtractParameter:         extractKotlinParameter,
	ExtractInitializer:       extractKotlinInitializer,
	ScanConstructorBinding:   scanKotlinConstructorBinding,
	ExtractForLoopBinding:    extractKotlinForLoopBinding,
	ExtractPendingAssignment: extractKotlinPendingAssignment,
	ExtractPatternBinding:    extractKotlinPatternBinding,
	InferLiteralType:         inferJvmLiteralType,
	DetectConstructorType:    detectKotlinConstructorType,
}

// Ensure utils import is used.
var _ = utils.FindChild
