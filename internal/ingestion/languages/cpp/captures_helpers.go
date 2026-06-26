package cpp

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
	"github.com/odvcencio/gotreesitter"
)

// ────────────────────────────────────────────────────────────────────────────
// AST helper functions
// ────────────────────────────────────────────────────────────────────────────

// findFirstDescendantOfType recursively searches for the first descendant
// of a given type.
func findFirstDescendantOfType(lang *gotreesitter.Language, node *gotreesitter.Node, targetType string) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	if node.Type(lang) == targetType {
		return node
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if c == nil {
			continue
		}
		hit := findFirstDescendantOfType(lang, c, targetType)
		if hit != nil {
			return hit
		}
	}
	return nil
}

// findChildOfType finds the first direct child matching one of the given types.
func findChildOfType(lang *gotreesitter.Language, node *gotreesitter.Node, types ...string) *gotreesitter.Node {
	for i := 0; i < int(node.ChildCount()); i++ {
		c := node.Child(i)
		if c == nil {
			continue
		}
		ct := c.Type(lang)
		for _, t := range types {
			if ct == t {
				return c
			}
		}
	}
	return nil
}

// getTypeIdentifierName gets the name of a class/struct/template_type node via its name field.
func getTypeIdentifierName(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) string {
	nameNode := node.ChildByFieldName("name", lang)
	if nameNode != nil {
		return nameNode.Text(source)
	}
	id := findFirstDescendantOfType(lang, node, "type_identifier")
	if id != nil {
		return id.Text(source)
	}
	return ""
}

// firstNamedChild returns the first named child of a node.
func firstNamedChild(lang *gotreesitter.Language, node *gotreesitter.Node) *gotreesitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		c := node.NamedChild(i)
		if c != nil {
			return c
		}
	}
	return nil
}

// ────────────────────────────────────────────────────────────────────────────
// Inheritance captures — emitCppInheritanceCaptures
// ────────────────────────────────────────────────────────────────────────────

// emitCppInheritanceCaptures walks every C++ class/struct base clause and emits
// @reference.inherits captures for each base so scope resolution can resolve
// them into EXTENDS edges.
func emitCppInheritanceCaptures(lang *gotreesitter.Language, root *gotreesitter.Node, source []byte, out *[]CaptureMatch, filePath string) {
	stack := []*gotreesitter.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil {
			continue
		}
		nt := node.Type(lang)
		if nt == "class_specifier" || nt == "struct_specifier" {
			baseClause := findChildOfType(lang, node, "base_class_clause")
			if baseClause != nil {
				for _, entry := range iterBaseClasses(lang, baseClause, source) {
					if entry.isPackExpansion {
						markClassWithPackExpandedBase(lang, filePath, node, source)
						continue
					}
					baseName := extractBaseLookupName(lang, entry.node, source)
					if baseName == "" {
						continue
					}
					qualifiedBaseName := extractQualifiedBaseName(lang, entry.node, source)
					cm := CaptureMatch{
						"@reference.inherits": utils.NodeToCapture("@reference.inherits", entry.node, source),
						"@reference.name":     utils.SyntheticCapture("@reference.name", entry.node, baseName),
					}
					if qualifiedBaseName != "" && qualifiedBaseName != baseName {
						cm["@reference.qualified-name"] = utils.SyntheticCapture(
							"@reference.qualified-name", entry.node, qualifiedBaseName,
						)
					}
					*out = append(*out, cm)
				}
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				stack = append(stack, child)
			}
		}
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Dependent base detection — detectCppDependentBases
// ────────────────────────────────────────────────────────────────────────────

// detectCppDependentBases walks the AST finding every template_declaration
// containing a class or struct definition with a dependent base.
func detectCppDependentBases(lang *gotreesitter.Language, root *gotreesitter.Node, source []byte, filePath string) {
	stack := []*gotreesitter.Node{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil {
			continue
		}
		if node.Type(lang) == "template_declaration" {
			params := collectTemplateParameterNames(lang, node, source)
			classNode := findChildOfType(lang, node, "class_specifier", "struct_specifier")
			if classNode != nil {
				className := getTypeIdentifierName(lang, classNode, source)
				if className != "" {
					baseClause := findChildOfType(lang, classNode, "base_class_clause")
					if baseClause != nil {
						for _, entry := range iterBaseClasses(lang, baseClause, source) {
							if entry.isPackExpansion {
								markClassWithPackExpandedBase(lang, filePath, classNode, source)
							}
							if entry.isPackExpansion || isBaseDependent(lang, entry.node, params, source) {
								baseName := extractBaseLookupName(lang, entry.node, source)
								baseQualifier := extractBaseLookupQualifier(lang, entry.node, source)
								if baseName != "" {
									RegisterCppDependentBase(filePath+":"+className, baseName)
									_ = baseQualifier // sidecar qualifier for future use
								}
							}
						}
					}
				}
			}
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child != nil {
				stack = append(stack, child)
			}
		}
	}
}

// collectTemplateParameterNames collects simple template parameter names from a template_declaration.
func collectTemplateParameterNames(lang *gotreesitter.Language, templateDecl *gotreesitter.Node, source []byte) map[string]bool {
	names := make(map[string]bool)
	paramList := findChildOfType(lang, templateDecl, "template_parameter_list")
	if paramList == nil {
		return names
	}
	for i := 0; i < int(paramList.ChildCount()); i++ {
		param := paramList.Child(i)
		if param == nil {
			continue
		}
		pt := param.Type(lang)
		if pt == "type_parameter_declaration" || pt == "optional_type_parameter_declaration" || pt == "variadic_type_parameter_declaration" {
			idNode := findFirstDescendantOfType(lang, param, "type_identifier")
			if idNode != nil {
				names[idNode.Text(source)] = true
			}
		} else if pt == "parameter_declaration" || pt == "optional_parameter_declaration" || pt == "variadic_parameter_declaration" {
			idNode := findFirstDescendantOfType(lang, param, "identifier")
			if idNode != nil {
				names[idNode.Text(source)] = true
			}
		} else if pt == "template_template_parameter_declaration" {
			idNode := findFirstDescendantOfType(lang, param, "type_identifier")
			if idNode != nil {
				names[idNode.Text(source)] = true
			}
		}
	}
	return names
}

// markClassWithPackExpandedBase marks a class as having a pack-expanded base.
func markClassWithPackExpandedBase(lang *gotreesitter.Language, filePath string, classNode *gotreesitter.Node, source []byte) {
	className := getTypeIdentifierName(lang, classNode, source)
	if className != "" {
		RegisterCppDependentBase(filePath+":"+className, "...")
	}
}

// ────────────────────────────────────────────────────────────────────────────
// Base class iteration and lookup name extraction
// ────────────────────────────────────────────────────────────────────────────

// cppBaseClassEntry represents a base-class entry from a base_class_clause.
type cppBaseClassEntry struct {
	node            *gotreesitter.Node
	isPackExpansion bool
}

// iterBaseClasses yields each base-class entry from a base_class_clause.
func iterBaseClasses(lang *gotreesitter.Language, baseClause *gotreesitter.Node, source []byte) []cppBaseClassEntry {
	var entries []cppBaseClassEntry
	for i := 0; i < int(baseClause.ChildCount()); i++ {
		child := baseClause.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "type_identifier" || ct == "template_type" || ct == "qualified_identifier" {
			entries = append(entries, cppBaseClassEntry{
				node:            child,
				isPackExpansion: isFollowedByPackExpansion(lang, baseClause, i, source),
			})
		}
	}
	return entries
}

// isFollowedByPackExpansion checks if a child is followed by a `...` token.
func isFollowedByPackExpansion(lang *gotreesitter.Language, baseClause *gotreesitter.Node, childIndex int, source []byte) bool {
	for i := childIndex + 1; i < int(baseClause.ChildCount()); i++ {
		sibling := baseClause.Child(i)
		if sibling == nil {
			continue
		}
		st := sibling.Type(lang)
		if st == "..." || sibling.Text(source) == "..." {
			return true
		}
		if st == "," || st == "access_specifier" {
			return false
		}
		// Named child that isn't `...` means no pack expansion
		if sibling.NamedChildCount() > 0 {
			return false
		}
	}
	return false
}

// extractBaseLookupName recursively extracts the bare lookup name of a base class node.
// Examples: `Base` → `Base`, `Base<T>` → `Base`, `outer::v1::Base<T>` → `Base`.
func extractBaseLookupName(lang *gotreesitter.Language, baseNode *gotreesitter.Node, source []byte) string {
	bt := baseNode.Type(lang)
	if bt == "type_identifier" || bt == "identifier" {
		return baseNode.Text(source)
	}
	if bt == "template_type" {
		nameNode := baseNode.ChildByFieldName("name", lang)
		if nameNode != nil {
			return extractBaseLookupName(lang, nameNode, source)
		}
		id := findFirstDescendantOfType(lang, baseNode, "type_identifier")
		if id == nil {
			id = findFirstDescendantOfType(lang, baseNode, "identifier")
		}
		if id != nil {
			return id.Text(source)
		}
	}
	if bt == "qualified_identifier" {
		nameNode := baseNode.ChildByFieldName("name", lang)
		if nameNode != nil {
			nested := extractBaseLookupName(lang, nameNode, source)
			if nested != "" {
				return nested
			}
		}
		for i := int(baseNode.ChildCount()) - 1; i >= 0; i-- {
			child := baseNode.Child(i)
			if child == nil {
				continue
			}
			nested := extractBaseLookupName(lang, child, source)
			if nested != "" {
				return nested
			}
		}
	}
	return ""
}

// extractQualifiedBaseName preserves the namespace/class qualifier while stripping template arguments.
// For unqualified bases, returns empty string.
func extractQualifiedBaseName(lang *gotreesitter.Language, baseNode *gotreesitter.Node, source []byte) string {
	bt := baseNode.Type(lang)
	if bt == "template_type" {
		nameNode := baseNode.ChildByFieldName("name", lang)
		if nameNode != nil {
			return extractQualifiedBaseName(lang, nameNode, source)
		}
		return ""
	}
	if bt == "qualified_identifier" {
		text := baseNode.Text(source)
		if !strings.Contains(text, "<") {
			return text
		}
		scopeNode := baseNode.ChildByFieldName("scope", lang)
		nameNode := baseNode.ChildByFieldName("name", lang)
		left := ""
		if scopeNode != nil {
			left = extractQualifiedBaseName(lang, scopeNode, source)
		}
		right := ""
		if nameNode != nil {
			right = extractQualifiedBaseName(lang, nameNode, source)
		}
		if left != "" && right != "" {
			return left + "::" + right
		}
		if right != "" {
			return right
		}
		return left
	}
	if bt == "namespace_identifier" || bt == "type_identifier" || bt == "identifier" {
		return baseNode.Text(source)
	}
	return ""
}

// extractBaseLookupQualifier extracts the namespace qualifier from a base class node.
func extractBaseLookupQualifier(lang *gotreesitter.Language, baseNode *gotreesitter.Node, source []byte) string {
	if baseNode.Type(lang) == "qualified_identifier" {
		scopeNode := baseNode.ChildByFieldName("scope", lang)
		if scopeNode != nil {
			return scopeNode.Text(source)
		}
	}
	if baseNode.Type(lang) == "template_type" {
		nameNode := baseNode.ChildByFieldName("name", lang)
		if nameNode != nil && nameNode.Type(lang) == "qualified_identifier" {
			scopeNode := nameNode.ChildByFieldName("scope", lang)
			if scopeNode != nil {
				return scopeNode.Text(source)
			}
		}
	}
	return ""
}

// isBaseDependent checks whether a base class is dependent on template parameters.
func isBaseDependent(lang *gotreesitter.Language, baseNode *gotreesitter.Node, templateParams map[string]bool, source []byte) bool {
	bt := baseNode.Type(lang)
	if bt != "template_type" {
		if bt == "qualified_identifier" {
			// Qualified identifier bases may contain template_type children
		} else {
			// Bare type_identifier bases — not dependent
			return false
		}
	}
	// Walk all descendants looking for template parameter references
	stack := []*gotreesitter.Node{baseNode}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil {
			continue
		}
		nt := node.Type(lang)
		if nt == "type_identifier" && templateParams[node.Text(source)] {
			return true
		}
		if nt == "decltype" || nt == "dependent_type" || nt == "template_template_parameter_declaration" {
			return true
		}
		for i := 0; i < int(node.ChildCount()); i++ {
			c := node.Child(i)
			if c != nil {
				stack = append(stack, c)
			}
		}
	}
	return false
}

// ────────────────────────────────────────────────────────────────────────────
// Argument type inference — inferCppCallArgTypes / inferCppCallArgTypeClasses
// ────────────────────────────────────────────────────────────────────────────

// inferCppCallArgTypes infers argument types from a call_expression or new_expression node.
func inferCppCallArgTypes(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) []string {
	argList := node.ChildByFieldName("arguments", lang)
	if argList == nil {
		return nil
	}
	var types []string
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "," || ct == "(" || ct == ")" {
			continue
		}
		litType := inferCppLiteralType(lang, child, source)
		if litType != "" {
			types = append(types, litType)
		} else if ct == "identifier" {
			types = append(types, lookupDeclaredTypeForIdentifier(lang, child, source))
		} else {
			types = append(types, "")
		}
	}
	return types
}

// inferCppCallArgTypeClasses infers argument type classes from a call_expression or new_expression node.
func inferCppCallArgTypeClasses(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) []shared.ParameterTypeClass {
	argList := node.ChildByFieldName("arguments", lang)
	if argList == nil {
		return nil
	}
	var classes []shared.ParameterTypeClass
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "," || ct == "(" || ct == ")" {
			continue
		}
		litType := inferCppLiteralType(lang, child, source)
		if litType != "" {
			classes = append(classes, valueTypeClass(litType))
		} else if ct == "identifier" {
			classes = append(classes, lookupDeclaredTypeClassForIdentifier(lang, child, source))
		} else {
			classes = append(classes, unknownTypeClass("unknown"))
		}
	}
	return classes
}

// inferCppBinaryOperatorArgTypes infers argument types from a binary_expression (operator call).
func inferCppBinaryOperatorArgTypes(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte, includeLeftOperand bool) []string {
	operands := binaryOperatorOperands(lang, node, includeLeftOperand)
	if len(operands) == 0 {
		return nil
	}
	var types []string
	for _, op := range operands {
		types = append(types, inferCppExpressionType(lang, op, source))
	}
	return types
}

// inferCppBinaryOperatorArgTypeClasses infers argument type classes from a binary_expression.
func inferCppBinaryOperatorArgTypeClasses(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte, includeLeftOperand bool) []shared.ParameterTypeClass {
	operands := binaryOperatorOperands(lang, node, includeLeftOperand)
	if len(operands) == 0 {
		return nil
	}
	var classes []shared.ParameterTypeClass
	for _, op := range operands {
		classes = append(classes, inferCppExpressionTypeClass(lang, op, source))
	}
	return classes
}

// binaryOperatorOperands extracts left/right operands from a binary_expression.
func binaryOperatorOperands(lang *gotreesitter.Language, node *gotreesitter.Node, includeLeftOperand bool) []*gotreesitter.Node {
	var operands []*gotreesitter.Node
	left := node.ChildByFieldName("left", lang)
	right := node.ChildByFieldName("right", lang)
	if includeLeftOperand && left != nil {
		operands = append(operands, left)
	}
	if right != nil {
		operands = append(operands, right)
	}
	return operands
}

// isPrimitiveOnlyBinaryOperator returns true if all operands are primitive types.
func isPrimitiveOnlyBinaryOperator(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) bool {
	operands := binaryOperatorOperands(lang, node, true)
	if len(operands) == 0 {
		return false
	}
	for _, op := range operands {
		if !isBuiltinOperatorType(lang, op, source) {
			return false
		}
	}
	return true
}

// isBuiltinOperatorType checks if a node's inferred type is a builtin primitive.
func isBuiltinOperatorType(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) bool {
	t := inferCppExpressionType(lang, node, source)
	switch t {
	case "bool", "char", "double", "float", "int", "long", "short", "signed", "unsigned":
		return true
	}
	return false
}

// inferCppExpressionType infers the type of a C++ expression node.
func inferCppExpressionType(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) string {
	litType := inferCppLiteralType(lang, node, source)
	if litType != "" {
		return litType
	}
	if node.Type(lang) == "identifier" {
		return lookupDeclaredTypeForIdentifier(lang, node, source)
	}
	return ""
}

// inferCppExpressionTypeClass infers the type class of a C++ expression node.
func inferCppExpressionTypeClass(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) shared.ParameterTypeClass {
	litType := inferCppLiteralType(lang, node, source)
	if litType != "" {
		return valueTypeClass(litType)
	}
	if node.Type(lang) == "identifier" {
		return lookupDeclaredTypeClassForIdentifier(lang, node, source)
	}
	return unknownTypeClass("unknown")
}

// valueTypeClass returns a value ParameterTypeClass for a base type.
func valueTypeClass(base string) shared.ParameterTypeClass {
	return shared.ParameterTypeClass{
		Base:         base,
		CV:           shared.CVNone,
		Indirection:  shared.IndirectionValue,
		PointerDepth: 0,
	}
}

// inferCppLiteralType infers the canonical type name of a C++ literal AST node.
func inferCppLiteralType(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) string {
	nt := node.Type(lang)
	switch nt {
	case "number_literal":
		text := node.Text(source)
		if strings.Contains(text, ".") || strings.Contains(text, "e") || strings.Contains(text, "E") ||
			strings.HasSuffix(text, "f") || strings.HasSuffix(text, "F") {
			return "double"
		}
		return "int"
	case "string_literal", "raw_string_literal", "concatenated_string":
		return "string"
	case "char_literal":
		return "char"
	case "true", "false":
		return "bool"
	case "null", "nullptr":
		return "null"
	default:
		return ""
	}
}

// lookupDeclaredTypeForIdentifier looks up the declared type of a variable by
// scanning sibling declarations in the enclosing compound_statement.
func lookupDeclaredTypeForIdentifier(lang *gotreesitter.Language, identNode *gotreesitter.Node, source []byte) string {
	varName := identNode.Text(source)
	// Walk up to the enclosing compound_statement or translation_unit
	scope := identNode.Parent()
	for scope != nil && scope.Type(lang) != "compound_statement" && scope.Type(lang) != "translation_unit" {
		scope = scope.Parent()
	}
	if scope == nil {
		return ""
	}

	// Check function parameters first
	paramType := lookupFunctionParameterType(lang, scope, varName, source)
	if paramType != "" {
		return paramType
	}

	// Scan declarations in the scope for a matching variable name
	for i := 0; i < int(scope.ChildCount()); i++ {
		stmt := scope.Child(i)
		if stmt == nil || stmt.Type(lang) != "declaration" {
			continue
		}
		typeNode := stmt.ChildByFieldName("type", lang)
		if typeNode == nil {
			continue
		}
		if typeNode.Type(lang) == "placeholder_type_specifier" {
			continue
		}
		declarator := stmt.ChildByFieldName("declarator", lang)
		if declarator == nil {
			continue
		}
		nameChild := declaredNameNode(lang, declarator)
		if nameChild != nil && extractDeclaratorLeafName(lang, nameChild, source) == varName {
			return normalizeCppTypeText(typeNode.Text(source))
		}
	}
	return ""
}

// lookupDeclaredTypeClassForIdentifier looks up the type class for a variable identifier.
func lookupDeclaredTypeClassForIdentifier(lang *gotreesitter.Language, identNode *gotreesitter.Node, source []byte) shared.ParameterTypeClass {
	varName := identNode.Text(source)
	scope := identNode.Parent()
	for scope != nil && scope.Type(lang) != "compound_statement" && scope.Type(lang) != "translation_unit" {
		scope = scope.Parent()
	}
	if scope == nil {
		return unknownTypeClass("unknown")
	}

	for i := 0; i < int(scope.ChildCount()); i++ {
		stmt := scope.Child(i)
		if stmt == nil || stmt.Type(lang) != "declaration" {
			continue
		}
		typeNode := stmt.ChildByFieldName("type", lang)
		if typeNode == nil {
			continue
		}
		if typeNode.Type(lang) == "placeholder_type_specifier" {
			continue
		}
		declarator := stmt.ChildByFieldName("declarator", lang)
		if declarator == nil {
			continue
		}
		nameChild := declaredNameNode(lang, declarator)
		if nameChild == nil || extractDeclaratorLeafName(lang, nameChild, source) != varName {
			continue
		}
		return ClassifyCppParameterTypeSidecar(typeNode.Text(source), nameChild.Text(source), strings.TrimSuffix(stmt.Text(source), ";"))
	}
	return unknownTypeClass("unknown")
}

// lookupFunctionParameterType looks up a variable's type from function parameters.
func lookupFunctionParameterType(lang *gotreesitter.Language, scope *gotreesitter.Node, varName string, source []byte) string {
	param := findEnclosingFunctionParameter(lang, scope, varName, source)
	if param == nil {
		return ""
	}
	typeNode := param.ChildByFieldName("type", lang)
	if typeNode == nil {
		return ""
	}
	return normalizeCppTypeText(typeNode.Text(source))
}

// findEnclosingFunctionParameter finds a parameter declaration matching varName
// in the nearest enclosing function.
func findEnclosingFunctionParameter(lang *gotreesitter.Language, scope *gotreesitter.Node, varName string, source []byte) *gotreesitter.Node {
	node := scope.Parent()
	for node != nil {
		nt := node.Type(lang)
		var params *gotreesitter.Node
		if nt == "function_declarator" {
			params = node.ChildByFieldName("parameters", lang)
		} else if nt == "function_definition" {
			decl := node.ChildByFieldName("declarator", lang)
			if decl != nil && decl.Type(lang) == "function_declarator" {
				params = decl.ChildByFieldName("parameters", lang)
			}
		}
		if params != nil {
			for i := 0; i < int(params.NamedChildCount()); i++ {
				param := params.NamedChild(i)
				if param == nil || param.Type(lang) != "parameter_declaration" {
					continue
				}
				declarator := param.ChildByFieldName("declarator", lang)
				if declarator != nil && extractDeclaratorLeafName(lang, declarator, source) == varName {
					return param
				}
			}
			return nil
		}
		if nt == "translation_unit" {
			break
		}
		node = node.Parent()
	}
	return nil
}

// declaredNameNode finds the name node from an init_declarator.
func declaredNameNode(lang *gotreesitter.Language, declarator *gotreesitter.Node) *gotreesitter.Node {
	if declarator.Type(lang) != "init_declarator" {
		return declarator
	}
	for i := 0; i < int(declarator.NamedChildCount()); i++ {
		child := declarator.NamedChild(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "identifier" {
			return child
		}
		if strings.HasSuffix(ct, "_declarator") {
			return child
		}
	}
	return declarator.ChildByFieldName("declarator", lang)
}

// extractDeclaratorLeafName extracts the leaf name from a declarator node chain.
func extractDeclaratorLeafName(lang *gotreesitter.Language, declarator *gotreesitter.Node, source []byte) string {
	cur := declarator
	hops := 8
	for cur != nil && hops > 0 {
		ct := cur.Type(lang)
		if ct == "identifier" {
			return cur.Text(source)
		}
		if ct == "pointer_declarator" || ct == "reference_declarator" || ct == "init_declarator" {
			next := cur.ChildByFieldName("declarator", lang)
			if next == nil {
				// Try named children
				for i := 0; i < int(cur.NamedChildCount()); i++ {
					c := cur.NamedChild(i)
					if c != nil {
						next = c
						break
					}
				}
			}
			cur = next
			hops--
			continue
		}
		// Fallback: return the text
		return cur.Text(source)
	}
	if cur != nil {
		return cur.Text(source)
	}
	return ""
}

// normalizeCppTypeText normalizes a type-specifier text for argument type matching.
func normalizeCppTypeText(text string) string {
	t := strings.TrimSpace(text)
	t = strings.ReplaceAll(t, "const", "")
	t = strings.ReplaceAll(t, "volatile", "")
	t = strings.ReplaceAll(t, "static", "")
	t = strings.ReplaceAll(t, "extern", "")
	t = strings.ReplaceAll(t, "mutable", "")
	t = strings.TrimSpace(t)
	// Strip namespace prefix
	if idx := strings.LastIndex(t, "::"); idx >= 0 {
		t = t[idx+2:]
	}
	// Strip pointer/reference markers
	t = strings.Map(func(r rune) rune {
		if r == '*' || r == '&' {
			return -1
		}
		return r
	}, t)
	t = strings.TrimSpace(t)
	return t
}

// ────────────────────────────────────────────────────────────────────────────
// ADL argument inference — inferCppCallAdlArgs
// ────────────────────────────────────────────────────────────────────────────

// inferCppCallAdlArgs infers ADL argument info from a free call expression.
func inferCppCallAdlArgs(lang *gotreesitter.Language, callNode *gotreesitter.Node, source []byte) []CppAdlArgInfo {
	argList := callNode.ChildByFieldName("arguments", lang)
	if argList == nil {
		return nil
	}
	var out []CppAdlArgInfo
	for i := 0; i < int(argList.ChildCount()); i++ {
		child := argList.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "," || ct == "(" || ct == ")" {
			continue
		}
		out = append(out, classifyAdlArg(lang, child, source))
	}
	return out
}

// classifyAdlArg classifies a single argument for ADL.
func classifyAdlArg(lang *gotreesitter.Language, argNode *gotreesitter.Node, source []byte) CppAdlArgInfo {
	// Literals and primitives never have associated namespaces
	switch argNode.Type(lang) {
	case "number_literal", "string_literal", "raw_string_literal", "char_literal",
		"true", "false", "null", "nullptr":
		return CppAdlArgInfo{}
	case "qualified_identifier":
		return CppAdlArgInfo{
			FunctionRefText: argNode.Text(source),
		}
	case "identifier":
		// Variable reference — look up its declared type
		result := lookupAdlIdentifierType(lang, argNode, source)
		if result == nil {
			return CppAdlArgInfo{
				FunctionRefText: argNode.Text(source),
			}
		}
		return *result
	default:
		return CppAdlArgInfo{}
	}
}

// lookupAdlIdentifierType looks up the ADL type info for an identifier.
func lookupAdlIdentifierType(lang *gotreesitter.Language, identNode *gotreesitter.Node, source []byte) *CppAdlArgInfo {
	varName := identNode.Text(source)
	scope := identNode.Parent()
	for scope != nil && scope.Type(lang) != "compound_statement" && scope.Type(lang) != "translation_unit" {
		scope = scope.Parent()
	}
	if scope == nil {
		return nil
	}

	// Scan declarations in the scope
	for i := 0; i < int(scope.ChildCount()); i++ {
		stmt := scope.Child(i)
		if stmt == nil || stmt.Type(lang) != "declaration" {
			continue
		}
		typeNode := stmt.ChildByFieldName("type", lang)
		if typeNode == nil {
			continue
		}
		if typeNode.Type(lang) == "placeholder_type_specifier" {
			continue
		}
		declarator := stmt.ChildByFieldName("declarator", lang)
		if declarator == nil {
			continue
		}
		leafName := extractDeclaratorLeafName(lang, declarator, source)
		if leafName == varName {
			// Found a matching declaration — classify its type for ADL
			typeText := normalizeCppTypeText(typeNode.Text(source))
			return &CppAdlArgInfo{
				SimpleClassName: typeText,
			}
		}
	}
	return nil
}
