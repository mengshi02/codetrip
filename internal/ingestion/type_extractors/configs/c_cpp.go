// Package configs provides per-language type extraction configurations.
// This file implements the C/C++ language type extractor configuration,
// ported from TS type-extractors/c-cpp.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// C/C++ smart pointer factory function names that create a typed object.
var smartPtrFactories = map[string]bool{
	"make_shared":                true,
	"make_unique":                true,
	"make_shared_for_overwrite":  true,
}

// Smart pointer wrapper type names. When the declared type is a smart pointer,
// the inner template type is extracted for virtual dispatch comparison.
var smartPtrWrappers = map[string]bool{
	"shared_ptr": true,
	"unique_ptr": true,
	"weak_ptr":   true,
}

// extractFirstTemplateTypeArg extracts the first type name from a
// template_argument_list child. Unwraps type_descriptor wrappers common in
// tree-sitter-cpp ASTs. Returns nil if no template arguments or no type found.
func extractFirstTemplateTypeArg(parentNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	var argsNode *gotreesitter.Node
	for i := 0; i < int(parentNode.ChildCount()); i++ {
		c := parentNode.Child(i)
		if c != nil && c.Type(lang) == "template_argument_list" {
			argsNode = c
			break
		}
	}
	if argsNode == nil {
		return nil
	}
	argNode := typeextractors.FirstNamedChild(argsNode, lang)
	if argNode == nil {
		return nil
	}
	if argNode.Type(lang) == "type_descriptor" {
		inner := argNode.ChildByFieldName("type", lang)
		if inner != nil {
			argNode = inner
		}
	}
	return typeextractors.ExtractSimpleTypeNameFromNode(argNode, source, lang, 0)
}

// ---------------------------------------------------------------------------
// extractDeclaration — C++: Type x = ...; Type* x; Type& x;
// ---------------------------------------------------------------------------

func cppExtractDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return
	}
	typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
	if typeName == nil {
		return
	}

	declarator := node.ChildByFieldName("declarator", lang)
	if declarator == nil {
		return
	}

	// init_declarator: Type x = value
	var nameNode *gotreesitter.Node
	if declarator.Type(lang) == "init_declarator" {
		nameNode = declarator.ChildByFieldName("declarator", lang)
	} else {
		nameNode = declarator
	}
	if nameNode == nil {
		return
	}

	// Handle pointer/reference declarators
	finalName := nameNode
	if nameNode.Type(lang) == "pointer_declarator" || nameNode.Type(lang) == "reference_declarator" {
		finalName = typeextractors.FirstNamedChild(nameNode, lang)
	}
	if finalName == nil {
		return
	}

	varName := typeextractors.ExtractVarName(finalName, source, lang)
	if varName != nil {
		env[*varName] = *typeName
	}
}

// ---------------------------------------------------------------------------
// extractInitializer — C++: auto x = new User(); auto x = User();
// ---------------------------------------------------------------------------

func cppExtractInitializer(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string, classNames typeextractors.ClassNameLookup) {
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return
	}

	// Only handle auto/placeholder — typed declarations are handled by extractDeclaration
	typeText := strings.TrimSpace(string(typeNode.Text(source)))
	if typeText != "auto" && typeText != "decltype(auto)" && typeNode.Type(lang) != "placeholder_type_specifier" {
		return
	}

	declarator := node.ChildByFieldName("declarator", lang)
	if declarator == nil {
		return
	}

	// Must be an init_declarator (i.e., has an initializer value)
	if declarator.Type(lang) != "init_declarator" {
		return
	}

	value := declarator.ChildByFieldName("value", lang)
	if value == nil {
		return
	}

	// Resolve the variable name, unwrapping pointer/reference declarators
	nameNode := declarator.ChildByFieldName("declarator", lang)
	if nameNode == nil {
		return
	}
	finalName := nameNode
	if nameNode.Type(lang) == "pointer_declarator" || nameNode.Type(lang) == "reference_declarator" {
		finalName = typeextractors.FirstNamedChild(nameNode, lang)
	}
	if finalName == nil {
		return
	}
	varName := typeextractors.ExtractVarName(finalName, source, lang)
	if varName == nil {
		return
	}

	// auto x = new User() — new_expression
	if value.Type(lang) == "new_expression" {
		ctorType := value.ChildByFieldName("type", lang)
		if ctorType != nil {
			typeName := typeextractors.ExtractSimpleTypeNameFromNode(ctorType, source, lang, 0)
			if typeName != nil {
				env[*varName] = *typeName
			}
		}
		return
	}

	// auto x = User() — call_expression
	if value.Type(lang) == "call_expression" {
		fn := value.ChildByFieldName("function", lang)
		if fn == nil {
			return
		}
		if fn.Type(lang) == "type_identifier" {
			t := strings.TrimSpace(string(fn.Text(source)))
			if t != "" {
				env[*varName] = t
			}
		} else if fn.Type(lang) == "identifier" {
			text := strings.TrimSpace(string(fn.Text(source)))
			if text != "" && classNames.Has(text) {
				env[*varName] = text
			}
		} else {
			// auto x = std::make_shared<Dog>() — smart pointer factory
			var templateFunc *gotreesitter.Node
			if fn.Type(lang) == "template_function" {
				templateFunc = fn
			} else if fn.Type(lang) == "qualified_identifier" || fn.Type(lang) == "scoped_identifier" {
				for i := 0; i < int(fn.NamedChildCount()); i++ {
					c := fn.NamedChild(i)
					if c != nil && c.Type(lang) == "template_function" {
						templateFunc = c
						break
					}
				}
			}
			if templateFunc != nil {
				nameNd := typeextractors.FirstNamedChild(templateFunc, lang)
				if nameNd != nil {
					var funcName string
					if nameNd.Type(lang) == "qualified_identifier" || nameNd.Type(lang) == "scoped_identifier" {
						last := typeextractors.LastNamedChild(nameNd, lang)
						if last != nil {
							funcName = strings.TrimSpace(string(last.Text(source)))
						}
					} else {
						funcName = strings.TrimSpace(string(nameNd.Text(source)))
					}
					if smartPtrFactories[funcName] {
						typeName := extractFirstTemplateTypeArg(templateFunc, source, lang)
						if typeName != nil {
							env[*varName] = *typeName
						}
					}
				}
			}
		}
		return
	}

	// auto x = User{} — compound_literal_expression (brace initialization)
	if value.Type(lang) == "compound_literal_expression" {
		typeId := typeextractors.FirstNamedChild(value, lang)
		if typeId != nil {
			typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeId, source, lang, 0)
			if typeName != nil {
				env[*varName] = *typeName
			}
		}
	}
}

// ---------------------------------------------------------------------------
// extractParameter — C/C++: parameter_declaration → type declarator
// ---------------------------------------------------------------------------

func cppExtractParameter(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	var nameNode *gotreesitter.Node
	var typeNode *gotreesitter.Node

	if node.Type(lang) == "parameter_declaration" {
		typeNode = node.ChildByFieldName("type", lang)
		declarator := node.ChildByFieldName("declarator", lang)
		if declarator != nil {
			if declarator.Type(lang) == "pointer_declarator" || declarator.Type(lang) == "reference_declarator" {
				nameNode = typeextractors.FirstNamedChild(declarator, lang)
			} else {
				nameNode = declarator
			}
		}
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
// scanConstructorBinding — C/C++: auto x = User() where function is identifier
// ---------------------------------------------------------------------------

func cppScanConstructorBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *typeextractors.ConstructorBindingResult {
	if node.Type(lang) != "declaration" {
		return nil
	}
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return nil
	}
	typeText := strings.TrimSpace(string(typeNode.Text(source)))
	if typeText != "auto" && typeText != "decltype(auto)" && typeNode.Type(lang) != "placeholder_type_specifier" {
		return nil
	}
	declarator := node.ChildByFieldName("declarator", lang)
	if declarator == nil || declarator.Type(lang) != "init_declarator" {
		return nil
	}
	value := declarator.ChildByFieldName("value", lang)
	if value == nil || value.Type(lang) != "call_expression" {
		return nil
	}
	fn := value.ChildByFieldName("function", lang)
	if fn == nil {
		return nil
	}

	resolveFinalName := func() *string {
		nameNd := declarator.ChildByFieldName("declarator", lang)
		if nameNd == nil {
			return nil
		}
		finalName := nameNd
		if nameNd.Type(lang) == "pointer_declarator" || nameNd.Type(lang) == "reference_declarator" {
			finalName = typeextractors.FirstNamedChild(nameNd, lang)
		}
		if finalName == nil {
			return nil
		}
		t := strings.TrimSpace(string(finalName.Text(source)))
		if t == "" {
			return nil
		}
		return &t
	}

	if fn.Type(lang) == "qualified_identifier" || fn.Type(lang) == "scoped_identifier" {
		last := typeextractors.LastNamedChild(fn, lang)
		if last == nil {
			return nil
		}
		vn := resolveFinalName()
		if vn == nil {
			return nil
		}
		calleeName := strings.TrimSpace(string(last.Text(source)))
		return &typeextractors.ConstructorBindingResult{
			VarName:    *vn,
			CalleeName: calleeName,
		}
	}
	if fn.Type(lang) != "identifier" {
		return nil
	}
	vn := resolveFinalName()
	if vn == nil {
		return nil
	}
	calleeName := strings.TrimSpace(string(fn.Text(source)))
	return &typeextractors.ConstructorBindingResult{
		VarName:    *vn,
		CalleeName: calleeName,
	}
}

// ---------------------------------------------------------------------------
// extractPendingAssignment — C++: auto alias = user
// ---------------------------------------------------------------------------

func cppExtractPendingAssignment(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []typeextractors.PendingAssignment {
	if node.Type(lang) != "declaration" {
		return nil
	}
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode == nil {
		return nil
	}
	typeText := strings.TrimSpace(string(typeNode.Text(source)))
	if typeText != "auto" && typeText != "decltype(auto)" && typeNode.Type(lang) != "placeholder_type_specifier" {
		return nil
	}
	declarator := node.ChildByFieldName("declarator", lang)
	if declarator == nil || declarator.Type(lang) != "init_declarator" {
		return nil
	}
	value := declarator.ChildByFieldName("value", lang)
	if value == nil {
		return nil
	}
	nameNode := declarator.ChildByFieldName("declarator", lang)
	if nameNode == nil {
		return nil
	}
	finalName := nameNode
	if nameNode.Type(lang) == "pointer_declarator" || nameNode.Type(lang) == "reference_declarator" {
		finalName = typeextractors.FirstNamedChild(nameNode, lang)
	}
	if finalName == nil {
		return nil
	}
	lhs := typeextractors.ExtractVarName(finalName, source, lang)
	if lhs == nil {
		return nil
	}
	if _, ok := scopeEnv[*lhs]; ok {
		return nil
	}

	// identifier RHS → copy
	if value.Type(lang) == "identifier" {
		rhs := strings.TrimSpace(string(value.Text(source)))
		return []typeextractors.PendingAssignment{{
			Kind: typeextractors.PAKindCopy,
			Lhs:  *lhs,
			Rhs:  rhs,
		}}
	}

	// field_expression RHS → fieldAccess
	if value.Type(lang) == "field_expression" {
		obj := typeextractors.FirstNamedChild(value, lang)
		field := typeextractors.LastNamedChild(value, lang)
		if obj != nil && obj.Type(lang) == "identifier" && field != nil && field.Type(lang) == "field_identifier" {
			return []typeextractors.PendingAssignment{{
				Kind:     typeextractors.PAKindFieldAccess,
				Lhs:      *lhs,
				Receiver: strings.TrimSpace(string(obj.Text(source))),
				Field:    strings.TrimSpace(string(field.Text(source))),
			}}
		}
	}

	// call_expression RHS
	if value.Type(lang) == "call_expression" {
		funcNode := value.ChildByFieldName("function", lang)
		if funcNode != nil && funcNode.Type(lang) == "identifier" {
			return []typeextractors.PendingAssignment{{
				Kind:   typeextractors.PAKindCallResult,
				Lhs:    *lhs,
				Callee: strings.TrimSpace(string(funcNode.Text(source))),
			}}
		}
		// method call: call_expression → function: field_expression
		if funcNode != nil && funcNode.Type(lang) == "field_expression" {
			obj := typeextractors.FirstNamedChild(funcNode, lang)
			field := typeextractors.LastNamedChild(funcNode, lang)
			if obj != nil && obj.Type(lang) == "identifier" && field != nil && field.Type(lang) == "field_identifier" {
				return []typeextractors.PendingAssignment{{
					Kind:     typeextractors.PAKindMethodCallResult,
					Lhs:      *lhs,
					Receiver: strings.TrimSpace(string(obj.Text(source))),
					Method:   strings.TrimSpace(string(field.Text(source))),
				}}
			}
		}
	}

	return nil
}

// ---------------------------------------------------------------------------
// For-loop Tier 1c
// ---------------------------------------------------------------------------

// extractCppTemplateTypeArgs extracts type arguments from a C++ template_type node.
// C++ template_type uses template_argument_list (not type_arguments), and each
// argument is a type_descriptor with a 'type' field containing the type_specifier.
func extractCppTemplateTypeArgs(templateTypeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
	argsNode := templateTypeNode.ChildByFieldName("arguments", lang)
	if argsNode == nil || argsNode.Type(lang) != "template_argument_list" {
		return nil
	}
	var result []string
	for i := 0; i < int(argsNode.NamedChildCount()); i++ {
		argNode := argsNode.NamedChild(i)
		if argNode == nil {
			continue
		}
		if argNode.Type(lang) == "type_descriptor" {
			inner := argNode.ChildByFieldName("type", lang)
			if inner != nil {
				argNode = inner
			}
		}
		name := typeextractors.ExtractSimpleTypeNameFromNode(argNode, source, lang, 0)
		if name != nil {
			result = append(result, *name)
		}
	}
	return result
}

// extractCppElementTypeFromTypeNode extracts element type from a C++ type annotation AST node.
// Handles: template_type (vector<User>, map<string, User>), pointer/reference types.
func extractCppElementTypeFromTypeNode(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition, depth int) *string {
	if depth > 50 || typeNode == nil {
		return nil
	}
	// template_type: vector<User>, map<string, User>
	if typeNode.Type(lang) == "template_type" {
		args := extractCppTemplateTypeArgs(typeNode, source, lang)
		if len(args) >= 1 {
			if pos == typeextractors.TypeArgFirst {
				return &args[0]
			}
			return &args[len(args)-1]
		}
	}
	// reference/pointer types: unwrap and recurse
	if typeNode.Type(lang) == "reference_type" || typeNode.Type(lang) == "pointer_type" || typeNode.Type(lang) == "type_descriptor" {
		inner := typeextractors.LastNamedChild(typeNode, lang)
		if inner != nil {
			return extractCppElementTypeFromTypeNode(inner, source, lang, pos, depth+1)
		}
	}
	// qualified/scoped types: std::vector<User> → unwrap to template_type child
	if typeNode.Type(lang) == "qualified_identifier" || typeNode.Type(lang) == "scoped_type_identifier" {
		inner := typeextractors.LastNamedChild(typeNode, lang)
		if inner != nil {
			return extractCppElementTypeFromTypeNode(inner, source, lang, pos, depth+1)
		}
	}
	return nil
}

// findCppParamElementType walks up from a for-range-loop to the enclosing
// function_definition and searches parameters for one named iterableName.
func findCppParamElementType(iterableName string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	current := startNode.Parent()
	for current != nil {
		if current.Type(lang) == "function_definition" {
			declarator := current.ChildByFieldName("declarator", lang)
			if declarator == nil {
				break
			}
			paramsNode := declarator.ChildByFieldName("parameters", lang)
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
					param := paramsNode.NamedChild(i)
					if param == nil || param.Type(lang) != "parameter_declaration" {
						continue
					}
					paramDeclarator := param.ChildByFieldName("declarator", lang)
					if paramDeclarator == nil {
						continue
					}
					// Unwrap reference/pointer declarators
					identNode := paramDeclarator
					if identNode.Type(lang) == "reference_declarator" || identNode.Type(lang) == "pointer_declarator" {
						child := typeextractors.FirstNamedChild(identNode, lang)
						if child != nil {
							identNode = child
						}
					}
					if strings.TrimSpace(string(identNode.Text(source))) != iterableName {
						continue
					}
					typeNd := param.ChildByFieldName("type", lang)
					if typeNd != nil {
						return extractCppElementTypeFromTypeNode(typeNd, source, lang, pos, 0)
					}
				}
			}
			break
		}
		current = current.Parent()
	}
	return nil
}

// cppExtractForLoopBinding — C++: for (auto& user : users)
func cppExtractForLoopBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *typeextractors.ForLoopExtractorContext) {
	if node.Type(lang) != "for_range_loop" {
		return
	}

	typeNode := node.ChildByFieldName("type", lang)
	declaratorNode := node.ChildByFieldName("declarator", lang)
	rightNode := node.ChildByFieldName("right", lang)
	if typeNode == nil || declaratorNode == nil || rightNode == nil {
		return
	}

	// Unwrap reference/pointer declarator to get the loop variable name
	nameNode := declaratorNode
	if nameNode.Type(lang) == "reference_declarator" || nameNode.Type(lang) == "pointer_declarator" {
		child := typeextractors.FirstNamedChild(nameNode, lang)
		if child != nil {
			nameNode = child
		}
	}

	// Handle structured bindings: auto& [key, value]
	var loopVarName *string
	if nameNode.Type(lang) == "structured_binding_declarator" {
		lastChild := typeextractors.LastNamedChild(nameNode, lang)
		if lastChild != nil && lastChild.Type(lang) == "identifier" {
			t := strings.TrimSpace(string(lastChild.Text(source)))
			loopVarName = &t
		}
	} else if declaratorNode.Type(lang) == "structured_binding_declarator" {
		lastChild := typeextractors.LastNamedChild(declaratorNode, lang)
		if lastChild != nil && lastChild.Type(lang) == "identifier" {
			t := strings.TrimSpace(string(lastChild.Text(source)))
			loopVarName = &t
		}
	}

	var varName string
	if loopVarName != nil {
		varName = *loopVarName
	} else {
		vn := typeextractors.ExtractVarName(nameNode, source, lang)
		if vn == nil {
			return
		}
		varName = *vn
	}

	// Check if the type is auto/placeholder
	typeText := strings.TrimSpace(string(typeNode.Text(source)))
	isAuto := typeNode.Type(lang) == "placeholder_type_specifier" ||
		typeText == "auto" || typeText == "const auto" || typeText == "decltype(auto)"

	if !isAuto {
		// Explicit type: for (User& user : users)
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0)
		if typeName != nil {
			ctx.ScopeEnv[varName] = *typeName
		}
		return
	}

	// auto/const auto/auto& — resolve from the iterable's container type
	var iterableName *string
	var methodName *string

	if rightNode.Type(lang) == "identifier" {
		t := strings.TrimSpace(string(rightNode.Text(source)))
		iterableName = &t
	} else if rightNode.Type(lang) == "field_expression" {
		prop := typeextractors.LastNamedChild(rightNode, lang)
		if prop != nil {
			t := strings.TrimSpace(string(prop.Text(source)))
			iterableName = &t
		}
	} else if rightNode.Type(lang) == "call_expression" {
		fieldExpr := rightNode.ChildByFieldName("function", lang)
		if fieldExpr != nil && fieldExpr.Type(lang) == "field_expression" {
			obj := typeextractors.FirstNamedChild(fieldExpr, lang)
			if obj != nil && obj.Type(lang) == "identifier" {
				t := strings.TrimSpace(string(obj.Text(source)))
				iterableName = &t
			}
			field := typeextractors.LastNamedChild(fieldExpr, lang)
			if field != nil && field.Type(lang) == "field_identifier" {
				t := strings.TrimSpace(string(field.Text(source)))
				methodName = &t
			}
		}
	} else if rightNode.Type(lang) == "pointer_expression" {
		operand := typeextractors.LastNamedChild(rightNode, lang)
		if operand != nil && operand.Type(lang) == "identifier" {
			t := strings.TrimSpace(string(operand.Text(source)))
			iterableName = &t
		}
	}
	if iterableName == nil {
		return
	}

	containerTypeName := ""
	if t, ok := ctx.ScopeEnv[*iterableName]; ok {
		containerTypeName = t
	}
	typeArgPos := typeextractors.MethodToTypeArgPosition("", containerTypeName)
	if methodName != nil {
		typeArgPos = typeextractors.MethodToTypeArgPosition(*methodName, containerTypeName)
	}

	extractFromTypeNode := func(typeNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
		return extractCppElementTypeFromTypeNode(typeNd, src, l, pos, 0)
	}
	findParamElementType := func(name string, startNd *gotreesitter.Node, src []byte, l *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
		return findCppParamElementType(name, startNd, src, l, pos)
	}

	elementType := typeextractors.ResolveIterableElementType(
		*iterableName, node, source, lang,
		ctx.ScopeEnv, ctx.DeclarationTypeNodes, ctx.Scope,
		extractFromTypeNode, findParamElementType, typeArgPos,
	)
	if elementType != nil {
		ctx.ScopeEnv[varName] = *elementType
	}
}

// ---------------------------------------------------------------------------
// inferLiteralType — C++ literal type inference
// ---------------------------------------------------------------------------

func cppInferLiteralType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	switch node.Type(lang) {
	case "number_literal":
		t := strings.TrimSpace(string(node.Text(source)))
		// Float suffixes
		if strings.HasSuffix(t, "f") || strings.HasSuffix(t, "F") {
			s := "float"
			return &s
		}
		if strings.Contains(t, ".") || strings.Contains(t, "e") || strings.Contains(t, "E") {
			s := "double"
			return &s
		}
		// Long suffix
		if strings.HasSuffix(t, "L") || strings.HasSuffix(t, "l") || strings.HasSuffix(t, "LL") || strings.HasSuffix(t, "ll") {
			s := "long"
			return &s
		}
		s := "int"
		return &s
	case "string_literal", "raw_string_literal", "concatenated_string":
		s := "string"
		return &s
	case "char_literal":
		s := "char"
		return &s
	case "true", "false":
		s := "bool"
		return &s
	case "null", "nullptr":
		s := "null"
		return &s
	}
	return nil
}

// ---------------------------------------------------------------------------
// detectCppConstructorType — smart pointer factory calls
// ---------------------------------------------------------------------------

func cppDetectConstructorType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, classNames typeextractors.ClassNameLookup) *string {
	declarator := node.ChildByFieldName("declarator", lang)
	if declarator == nil || declarator.Type(lang) != "init_declarator" {
		return nil
	}
	value := declarator.ChildByFieldName("value", lang)
	if value == nil || value.Type(lang) != "call_expression" {
		return nil
	}
	fn := value.ChildByFieldName("function", lang)
	if fn == nil || fn.Type(lang) != "template_function" {
		return nil
	}
	nameNd := typeextractors.FirstNamedChild(fn, lang)
	if nameNd == nil {
		return nil
	}
	var funcName string
	if nameNd.Type(lang) == "qualified_identifier" || nameNd.Type(lang) == "scoped_identifier" {
		last := typeextractors.LastNamedChild(nameNd, lang)
		if last != nil {
			funcName = strings.TrimSpace(string(last.Text(source)))
		}
	} else {
		funcName = strings.TrimSpace(string(nameNd.Text(source)))
	}
	if !smartPtrFactories[funcName] {
		return nil
	}
	return extractFirstTemplateTypeArg(fn, source, lang)
}

// ---------------------------------------------------------------------------
// unwrapCppDeclaredType — unwrap smart pointer declared type
// ---------------------------------------------------------------------------

func cppUnwrapDeclaredType(declaredType string, typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	if !smartPtrWrappers[declaredType] {
		return &declaredType
	}
	if typeNode.Type(lang) != "template_type" {
		return &declaredType
	}
	result := extractFirstTemplateTypeArg(typeNode, source, lang)
	if result != nil {
		return result
	}
	return &declaredType
}

// ---------------------------------------------------------------------------
// C/C++ TypeConfig
// ---------------------------------------------------------------------------

// CCppTypeConfig is the C/C++ language type extractor configuration.
var CCppTypeConfig = typeextractors.LanguageTypeConfig{
	DeclarationNodeTypes:     []string{"declaration"},
	ForLoopNodeTypes:         []string{"for_range_loop"},
	ExtractDeclaration:       cppExtractDeclaration,
	ExtractParameter:         cppExtractParameter,
	ExtractInitializer:       cppExtractInitializer,
	ScanConstructorBinding:   cppScanConstructorBinding,
	ExtractForLoopBinding:    cppExtractForLoopBinding,
	ExtractPendingAssignment: cppExtractPendingAssignment,
	InferLiteralType:         cppInferLiteralType,
	DetectConstructorType:    cppDetectConstructorType,
	UnwrapDeclaredType:       cppUnwrapDeclaredType,
}

// Ensure utils import is used (FindChild alias in shared.go).
var _ = utils.FindChild