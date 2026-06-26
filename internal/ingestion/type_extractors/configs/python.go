// Package configs provides per-language type extraction configurations.
// This file implements the Python language type extractor configuration,
// ported from TS type-extractors/python.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
)

// extractPythonDeclaration — Python: x: Foo = ... (PEP 484 annotated assignment)
// or x: Foo (standalone annotation).
// 双形态：assignment (有值) + expression_statement (独立注解)
// type wrapper 解包：FirstNamedChild，fallback 到 raw text
func extractPythonDeclaration(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	if node.Type(lang) == "expression_statement" {
		typeChild := typeextractors.FirstNamedChild(node, lang)
		if typeChild == nil || typeChild.Type(lang) != "type" {
			return
		}
		nameNode := typeChild.ChildByFieldName("name", lang)
		typeNode := typeChild.ChildByFieldName("type", lang)
		if nameNode == nil || typeNode == nil {
			return
		}
		varName := typeextractors.ExtractVarName(nameNode, source, lang)
		inner := typeNode
		if typeNode.Type(lang) == "type" {
			if fc := typeextractors.FirstNamedChild(typeNode, lang); fc != nil {
				inner = fc
			}
		}
		typeName := typeextractors.ExtractSimpleTypeNameFromNode(inner, source, lang, 0)
		if typeName == nil {
			t := strings.TrimSpace(string(inner.Text(source)))
			typeName = &t
		}
		if varName != nil && typeName != nil {
			env[*varName] = *typeName
		}
		return
	}
	// Annotated assignment: left : type = value
	left := node.ChildByFieldName("left", lang)
	typeNode := node.ChildByFieldName("type", lang)
	if left == nil || typeNode == nil {
		return
	}
	varName := typeextractors.ExtractVarName(left, source, lang)
	inner := typeNode
	if typeNode.Type(lang) == "type" {
		if fc := typeextractors.FirstNamedChild(typeNode, lang); fc != nil {
			inner = fc
		}
	}
	typeName := typeextractors.ExtractSimpleTypeNameFromNode(inner, source, lang, 0)
	if typeName == nil {
		t := strings.TrimSpace(string(inner.Text(source)))
		typeName = &t
	}
	if varName != nil && typeName != nil {
		env[*varName] = *typeName
	}
}

// extractPythonParameter — Python: parameter with type annotation
// typed_parameter: name is a positional child (identifier), not a named field
func extractPythonParameter(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string) {
	var nameNode *gotreesitter.Node
	var typeNode *gotreesitter.Node
	if node.Type(lang) == "parameter" {
		nameNode = node.ChildByFieldName("name", lang)
		typeNode = node.ChildByFieldName("type", lang)
	} else {
		nameNode = node.ChildByFieldName("name", lang)
		typeNode = node.ChildByFieldName("type", lang)
		if nameNode == nil && node.Type(lang) == "typed_parameter" {
			if fc := typeextractors.FirstNamedChild(node, lang); fc != nil && fc.Type(lang) == "identifier" {
				nameNode = fc
			}
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

// extractPythonInitializer — Python: user = User("alice")
// Also handles walrus operator: if (user := User("alice"))
func extractPythonInitializer(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, env map[string]string, classNames typeextractors.ClassNameLookup) {
	var left *gotreesitter.Node
	var right *gotreesitter.Node
	if node.Type(lang) == "named_expression" {
		left = node.ChildByFieldName("name", lang)
		right = node.ChildByFieldName("value", lang)
	} else if node.Type(lang) == "assignment" {
		left = node.ChildByFieldName("left", lang)
		right = node.ChildByFieldName("right", lang)
		if node.ChildByFieldName("type", lang) != nil {
			return
		}
	} else {
		return
	}
	if left == nil || right == nil {
		return
	}
	varName := typeextractors.ExtractVarName(left, source, lang)
	if varName == nil {
		return
	}
	if _, exists := env[*varName]; exists {
		return
	}
	if right.Type(lang) != "call" {
		return
	}
	funcNode := right.ChildByFieldName("function", lang)
	if funcNode == nil {
		return
	}
	calleeName := typeextractors.ExtractSimpleTypeNameFromNode(funcNode, source, lang, 0)
	if calleeName == nil {
		return
	}
	if classNames.Has(*calleeName) {
		env[*varName] = *calleeName
	}
}

// scanPythonConstructorBinding — Python: user = User("alice")
// Returns {varName, calleeName} without checking classNames.
func scanPythonConstructorBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *typeextractors.ConstructorBindingResult {
	var left *gotreesitter.Node
	var right *gotreesitter.Node
	if node.Type(lang) == "named_expression" {
		left = node.ChildByFieldName("name", lang)
		right = node.ChildByFieldName("value", lang)
	} else if node.Type(lang) == "assignment" {
		left = node.ChildByFieldName("left", lang)
		right = node.ChildByFieldName("right", lang)
		if node.ChildByFieldName("type", lang) != nil {
			return nil
		}
	} else {
		return nil
	}
	if left == nil || right == nil {
		return nil
	}
	if left.Type(lang) != "identifier" {
		return nil
	}
	if right.Type(lang) != "call" {
		return nil
	}
	funcNode := right.ChildByFieldName("function", lang)
	if funcNode == nil {
		return nil
	}
	calleeName := typeextractors.ExtractSimpleTypeNameFromNode(funcNode, source, lang, 0)
	if calleeName == nil {
		return nil
	}
	return &typeextractors.ConstructorBindingResult{
		VarName:    strings.TrimSpace(string(left.Text(source))),
		CalleeName: *calleeName,
	}
}

// extractPyElementTypeFromAnnotation extracts element type from a Python type annotation AST node.
// Handles: subscript "List[User]", generic_type dict[str, User], falls back to text-based.
func extractPyElementTypeFromAnnotation(typeNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	inner := typeNode
	if typeNode.Type(lang) == "type" {
		if fc := typeextractors.FirstNamedChild(typeNode, lang); fc != nil {
			inner = fc
		}
	}
	if inner.Type(lang) == "subscript" {
		text := string(inner.Text(source))
		return typeextractors.ExtractElementTypeFromString(text, pos)
	}
	if inner.Type(lang) == "generic_type" {
		args := typeextractors.ExtractGenericTypeArgs(inner, source, lang, 0)
		if len(args) >= 1 {
			if pos == typeextractors.TypeArgFirst {
				return &args[0]
			}
			return &args[len(args)-1]
		}
		for i := 0; i < int(inner.NamedChildCount()); i++ {
			child := inner.NamedChild(i)
			if child != nil && child.Type(lang) == "type_parameter" {
				if pos == typeextractors.TypeArgFirst {
					firstArg := typeextractors.FirstNamedChild(child, lang)
					if firstArg != nil {
						return typeextractors.ExtractSimpleTypeNameFromNode(firstArg, source, lang, 0)
					}
				} else {
					lastArg := typeextractors.LastNamedChild(child, lang)
					if lastArg != nil {
						return typeextractors.ExtractSimpleTypeNameFromNode(lastArg, source, lang, 0)
					}
				}
			}
		}
	}
	text := string(inner.Text(source))
	return typeextractors.ExtractElementTypeFromString(text, pos)
}

// findPyParamElementType walks up to find the enclosing function definition,
// then searches its parameters for one named iterableName.
func findPyParamElementType(iterableName string, startNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language, pos typeextractors.TypeArgPosition) *string {
	current := startNode.Parent()
	for current != nil {
		if current.Type(lang) == "function_definition" {
			paramsNode := current.ChildByFieldName("parameters", lang)
			if paramsNode != nil {
				for i := 0; i < int(paramsNode.NamedChildCount()); i++ {
					param := paramsNode.NamedChild(i)
					if param == nil {
						continue
					}
					nameNode := param.ChildByFieldName("name", lang)
					if nameNode == nil {
						if fc := typeextractors.FirstNamedChild(param, lang); fc != nil && fc.Type(lang) == "identifier" {
							nameNode = fc
						}
					}
					if nameNode == nil || strings.TrimSpace(string(nameNode.Text(source))) != iterableName {
						continue
					}
					typeAnnotation := param.ChildByFieldName("type", lang)
					if typeAnnotation == nil && param.NamedChildCount() >= 2 {
						typeAnnotation = param.NamedChild(param.NamedChildCount() - 1)
					}
					if typeAnnotation != nil && typeAnnotation != nameNode {
						return extractPyElementTypeFromAnnotation(typeAnnotation, source, lang, pos)
					}
				}
			}
			break
		}
		current = current.Parent()
	}
	return nil
}

func extractPyMethodCall(callNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) (iterableName *string, methodName *string) {
	fn := callNode.ChildByFieldName("function", lang)
	if fn == nil || fn.Type(lang) != "attribute" {
		return nil, nil
	}
	obj := typeextractors.FirstNamedChild(fn, lang)
	if obj == nil || obj.Type(lang) != "identifier" {
		return nil, nil
	}
	objName := strings.TrimSpace(string(obj.Text(source)))
	iterableName = &objName
	method := typeextractors.LastNamedChild(fn, lang)
	if method != nil && method.Type(lang) == "identifier" && method != obj {
		t := strings.TrimSpace(string(method.Text(source)))
		methodName = &t
	}
	return iterableName, methodName
}

func collectPatternIdentifiers(pattern *gotreesitter.Node, lang *gotreesitter.Language) []*gotreesitter.Node {
	var vars []*gotreesitter.Node
	for i := 0; i < int(pattern.NamedChildCount()); i++ {
		child := pattern.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "identifier" {
			vars = append(vars, child)
		} else if child.Type(lang) == "tuple_pattern" {
			vars = append(vars, collectPatternIdentifiers(child, lang)...)
		}
	}
	return vars
}

func extractPythonForLoopBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, ctx *typeextractors.ForLoopExtractorContext) {
	if node.Type(lang) != "for_statement" {
		return
	}
	rightNode := node.ChildByFieldName("right", lang)
	var iterableName *string
	var methodName *string
	var callExprElementType *string
	isEnumerate := false

	if rightNode != nil && rightNode.Type(lang) == "identifier" {
		t := strings.TrimSpace(string(rightNode.Text(source)))
		iterableName = &t
	} else if rightNode != nil && rightNode.Type(lang) == "attribute" {
		prop := typeextractors.LastNamedChild(rightNode, lang)
		if prop != nil {
			t := strings.TrimSpace(string(prop.Text(source)))
			iterableName = &t
		}
	} else if rightNode != nil && rightNode.Type(lang) == "call" {
		fn := rightNode.ChildByFieldName("function", lang)
		if fn != nil && fn.Type(lang) == "identifier" && strings.TrimSpace(string(fn.Text(source))) == "enumerate" {
			isEnumerate = true
			argsNode := rightNode.ChildByFieldName("arguments", lang)
			var innerArg *gotreesitter.Node
			if argsNode != nil {
				innerArg = typeextractors.FirstNamedChild(argsNode, lang)
			}
			if innerArg != nil && innerArg.Type(lang) == "identifier" {
				t := strings.TrimSpace(string(innerArg.Text(source)))
				iterableName = &t
			} else if innerArg != nil && innerArg.Type(lang) == "call" {
				iterableName, methodName = extractPyMethodCall(innerArg, source, lang)
			}
		} else if fn != nil && fn.Type(lang) == "attribute" {
			iterableName, methodName = extractPyMethodCall(rightNode, source, lang)
		} else if fn != nil && fn.Type(lang) == "identifier" {
			calleeName := strings.TrimSpace(string(fn.Text(source)))
			if rawReturn := ctx.ReturnTypeLookup.LookupRawReturnType(calleeName); rawReturn != nil {
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
			extractPyElementTypeFromAnnotation, findPyParamElementType, typeArgPos,
		)
	}
	if elementType == nil {
		return
	}

	leftNode := node.ChildByFieldName("left", lang)
	if leftNode == nil {
		return
	}
	if leftNode.Type(lang) == "pattern_list" || leftNode.Type(lang) == "tuple_pattern" {
		vars := collectPatternIdentifiers(leftNode, lang)
		if len(vars) > 0 && (!isEnumerate || len(vars) > 1) {
			lastVar := vars[len(vars)-1]
			varName := strings.TrimSpace(string(lastVar.Text(source)))
			ctx.ScopeEnv[varName] = *elementType
		}
		return
	}
	loopVarName := typeextractors.ExtractVarName(leftNode, source, lang)
	if loopVarName != nil {
		ctx.ScopeEnv[*loopVarName] = *elementType
	}
}

func extractPythonPendingAssignment(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string) []typeextractors.PendingAssignment {
	var left *gotreesitter.Node
	var right *gotreesitter.Node

	if node.Type(lang) == "assignment" {
		left = node.ChildByFieldName("left", lang)
		right = node.ChildByFieldName("right", lang)
	} else if node.Type(lang) == "named_expression" {
		left = node.ChildByFieldName("name", lang)
		right = node.ChildByFieldName("value", lang)
	} else {
		return nil
	}
	if left == nil || right == nil {
		return nil
	}
	if left.Type(lang) != "identifier" {
		return nil
	}
	lhs := strings.TrimSpace(string(left.Text(source)))
	if _, exists := scopeEnv[lhs]; exists {
		return nil
	}

	if right.Type(lang) == "identifier" {
		rhs := strings.TrimSpace(string(right.Text(source)))
		return []typeextractors.PendingAssignment{{
			Kind: typeextractors.PAKindCopy,
			Lhs:  lhs,
			Rhs:  rhs,
		}}
	}
	if right.Type(lang) == "attribute" {
		obj := typeextractors.FirstNamedChild(right, lang)
		field := typeextractors.LastNamedChild(right, lang)
		if obj != nil && obj.Type(lang) == "identifier" && field != nil && field.Type(lang) == "identifier" && obj != field {
			return []typeextractors.PendingAssignment{{
				Kind:     typeextractors.PAKindFieldAccess,
				Lhs:      lhs,
				Receiver: strings.TrimSpace(string(obj.Text(source))),
				Field:    strings.TrimSpace(string(field.Text(source))),
			}}
		}
	}
	if right.Type(lang) == "call" {
		funcNode := right.ChildByFieldName("function", lang)
		if funcNode != nil && funcNode.Type(lang) == "identifier" {
			return []typeextractors.PendingAssignment{{
				Kind:   typeextractors.PAKindCallResult,
				Lhs:    lhs,
				Callee: strings.TrimSpace(string(funcNode.Text(source))),
			}}
		}
		if funcNode != nil && funcNode.Type(lang) == "attribute" {
			obj := typeextractors.FirstNamedChild(funcNode, lang)
			method := typeextractors.LastNamedChild(funcNode, lang)
			if obj != nil && obj.Type(lang) == "identifier" && method != nil && method.Type(lang) == "identifier" && obj != method {
				return []typeextractors.PendingAssignment{{
					Kind:     typeextractors.PAKindMethodCallResult,
					Lhs:      lhs,
					Receiver: strings.TrimSpace(string(obj.Text(source))),
					Method:   strings.TrimSpace(string(method.Text(source))),
				}}
			}
		}
	}
	return nil
}

func extractPythonPatternBinding(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language, scopeEnv map[string]string, declarationTypeNodes map[string]*gotreesitter.Node, scope string) *typeextractors.PatternBindingResult {
	if node.Type(lang) != "as_pattern" {
		return nil
	}
	if node.NamedChildCount() < 2 {
		return nil
	}

	patternChild := node.NamedChild(0)
	var varNameNode *gotreesitter.Node
	aliasField := node.ChildByFieldName("alias", lang)
	if aliasField != nil {
		varNameNode = aliasField
	} else {
		varNameNode = node.NamedChild(node.NamedChildCount() - 1)
	}
	if patternChild == nil || varNameNode == nil {
		return nil
	}
	if varNameNode.Type(lang) != "identifier" {
		return nil
	}
	varName := strings.TrimSpace(string(varNameNode.Text(source)))
	if _, exists := scopeEnv[varName]; exists {
		return nil
	}

	var classPattern *gotreesitter.Node
	if patternChild.Type(lang) == "class_pattern" {
		classPattern = patternChild
	} else if patternChild.Type(lang) == "case_pattern" {
		for j := 0; j < int(patternChild.NamedChildCount()); j++ {
			inner := patternChild.NamedChild(j)
			if inner != nil && inner.Type(lang) == "class_pattern" {
				classPattern = inner
				break
			}
		}
	}
	if classPattern == nil {
		return nil
	}

	classNameNode := typeextractors.FirstNamedChild(classPattern, lang)
	if classNameNode == nil || (classNameNode.Type(lang) != "dotted_name" && classNameNode.Type(lang) != "identifier") {
		return nil
	}
	typeName := strings.TrimSpace(string(classNameNode.Text(source)))
	if typeName == "" {
		return nil
	}
	return &typeextractors.PatternBindingResult{
		VarName:  varName,
		TypeName: typeName,
	}
}

var PythonTypeConfig = typeextractors.LanguageTypeConfig{
	DeclarationNodeTypes: []string{
		"assignment",
		"named_expression",
		"expression_statement",
	},
	ForLoopNodeTypes: []string{
		"for_statement",
	},
	PatternBindingNodeTypes: []string{
		"as_pattern",
	},
	ExtractDeclaration:       extractPythonDeclaration,
	ExtractParameter:         extractPythonParameter,
	ExtractInitializer:       extractPythonInitializer,
	ScanConstructorBinding:   scanPythonConstructorBinding,
	ExtractForLoopBinding:    extractPythonForLoopBinding,
	ExtractPendingAssignment: extractPythonPendingAssignment,
	ExtractPatternBinding:    extractPythonPatternBinding,
}

var _ = utils.FindChild
