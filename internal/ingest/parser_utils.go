package ingest

// Parser utility functions for extracting code structure from tree-sitter AST nodes.

import (
	"path/filepath"
	"strings"
	"unicode"

	graph "github.com/mengshi02/codetrip/internal/model"
	sitter "github.com/tree-sitter/go-tree-sitter"
)

// ─────────────────────────────────────────────────────────────────────────────
// GetLanguageFromFilename maps file extension to language ID.
// ─────────────────────────────────────────────────────────────────────────────

func GetLanguageFromFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	return LanguageID(ext)
}

// GetParserFromFilename returns the internal grammar/dialect identifier.
func GetParserFromFilename(filename string) string {
	ext := strings.ToLower(filepath.Ext(filename))
	return ParserID(ext)
}

// ─────────────────────────────────────────────────────────────────────────────
// GetDefinitionNodeFromCaptures extracts the definition node from a capture map.
// Iterates through DefinitionCaptureKeys in priority order.
// ─────────────────────────────────────────────────────────────────────────────

func GetDefinitionNodeFromCaptures(captureMap map[string]*sitter.Node) *sitter.Node {
	for _, key := range DefinitionCaptureKeys {
		if node, ok := captureMap[key]; ok {
			return node
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ExtractFunctionName extracts the function name and label from an AST node.
// Handles C/C++ qualified_identifier, Swift init/deinit, and other language patterns.
// ─────────────────────────────────────────────────────────────────────────────

func ExtractFunctionName(node *sitter.Node, source []byte) (funcName string, label string) {
	label = "Function"

	// Swift init/deinit
	if node.Kind() == "init_declaration" {
		return "init", "Constructor"
	}
	if node.Kind() == "deinit_declaration" {
		return "deinit", "Constructor"
	}

	if FunctionDeclarationTypes[node.Kind()] {
		// C/C++: function_definition -> [pointer_declarator ->] function_declarator -> qualified_identifier/identifier
		declarator := node.ChildByFieldName("declarator")
		if declarator == nil {
			// Fallback: find function_declarator child
			for i := uint(0); i < node.ChildCount(); i++ {
				child := node.Child(i)
				if child != nil && child.Kind() == "function_declarator" {
					declarator = child
					break
				}
			}
		}
		// Unwrap pointer_declarator / reference_declarator
		for declarator != nil && (declarator.Kind() == "pointer_declarator" || declarator.Kind() == "reference_declarator") {
			inner := declarator.ChildByFieldName("declarator")
			if inner == nil {
				for i := uint(0); i < declarator.ChildCount(); i++ {
					c := declarator.Child(i)
					if c != nil && (c.Kind() == "function_declarator" || c.Kind() == "pointer_declarator" || c.Kind() == "reference_declarator") {
						inner = c
						break
					}
				}
			}
			declarator = inner
		}

		if declarator != nil {
			innerDeclarator := declarator.ChildByFieldName("declarator")
			if innerDeclarator == nil {
				for i := uint(0); i < declarator.ChildCount(); i++ {
					c := declarator.Child(i)
					if c != nil && (c.Kind() == "qualified_identifier" || c.Kind() == "identifier" || c.Kind() == "parenthesized_declarator") {
						innerDeclarator = c
						break
					}
				}
			}

			if innerDeclarator != nil {
				switch innerDeclarator.Kind() {
				case "qualified_identifier":
					nameNode := innerDeclarator.ChildByFieldName("name")
					if nameNode == nil {
						for i := uint(0); i < innerDeclarator.ChildCount(); i++ {
							c := innerDeclarator.Child(i)
							if c != nil && c.Kind() == "identifier" {
								nameNode = c
								break
							}
						}
					}
					if nameNode != nil {
						funcName = nameNode.Utf8Text(source)
						label = "Method"
					}
				case "identifier":
					funcName = innerDeclarator.Utf8Text(source)
				case "parenthesized_declarator":
					nestedID := findChildByKinds(innerDeclarator, "qualified_identifier", "identifier")
					if nestedID != nil {
						if nestedID.Kind() == "qualified_identifier" {
							nameNode := nestedID.ChildByFieldName("name")
							if nameNode == nil {
								nameNode = findChildByKind(nestedID, "identifier")
							}
							if nameNode != nil {
								funcName = nameNode.Utf8Text(source)
								label = "Method"
							}
						} else {
							funcName = nestedID.Utf8Text(source)
						}
					}
				}
			}
		}

		// Fallback: try 'name' field or identifier/property_identifier/simple_identifier child
		if funcName == "" {
			nameNode := node.ChildByFieldName("name")
			if nameNode == nil {
				nameNode = findChildByKinds(node, "identifier", "property_identifier", "simple_identifier")
			}
			if nameNode != nil {
				funcName = nameNode.Utf8Text(source)
			}
		}

	} else if node.Kind() == "impl_item" {
		// Rust impl_item — find function_item child
		funcItem := findChildByKind(node, "function_item")
		if funcItem != nil {
			nameNode := funcItem.ChildByFieldName("name")
			if nameNode == nil {
				nameNode = findChildByKind(funcItem, "identifier")
			}
			if nameNode != nil {
				funcName = nameNode.Utf8Text(source)
				label = "Method"
			}
		}
	} else if node.Kind() == "method_definition" {
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = findChildByKind(node, "property_identifier")
		}
		if nameNode != nil {
			funcName = nameNode.Utf8Text(source)
			label = "Method"
		}
	} else if node.Kind() == "method_declaration" || node.Kind() == "constructor_declaration" {
		nameNode := node.ChildByFieldName("name")
		if nameNode == nil {
			nameNode = findChildByKind(node, "identifier")
		}
		if nameNode != nil {
			funcName = nameNode.Utf8Text(source)
			label = "Method"
		}
	} else if node.Kind() == "arrow_function" || node.Kind() == "function_expression" {
		parent := node.Parent()
		if parent != nil && parent.Kind() == "variable_declarator" {
			nameNode := parent.ChildByFieldName("name")
			if nameNode == nil {
				nameNode = findChildByKind(parent, "identifier")
			}
			if nameNode != nil {
				funcName = nameNode.Utf8Text(source)
			}
		}
		if funcName == "" && parent != nil && parent.Kind() == "assignment_expression" {
			left := parent.ChildByFieldName("left")
			if left != nil {
				funcName = left.Utf8Text(source)
			}
		}
	}

	return funcName, label
}

// ─────────────────────────────────────────────────────────────────────────────
// FindEnclosingClassId walks up AST to find enclosing class/struct/interface/impl.
// For Go method_declaration, extracts receiver type.
// ─────────────────────────────────────────────────────────────────────────────

func FindEnclosingClassId(node *sitter.Node, filePath string, source []byte) string {
	if node == nil {
		return ""
	}
	current := node.Parent()
	for current != nil {
		// Go: method_declaration has a receiver parameter with the struct type
		if current.Kind() == "method_declaration" {
			receiver := current.ChildByFieldName("receiver")
			if receiver != nil {
				// receiver is a parameter_list: (u *User) or (u User)
				paramDecl := findChildByKind(receiver, "parameter_declaration")
				if paramDecl != nil {
					typeNode := paramDecl.ChildByFieldName("type")
					if typeNode != nil {
						// Unwrap pointer_type (*User → User)
						inner := typeNode
						if inner.Kind() == "pointer_type" {
							innerType := inner.ChildByFieldName("type")
							if innerType != nil {
								inner = innerType
							} else {
								nc := inner.NamedChild(0)
								if nc != nil {
									inner = nc
								}
							}
						}
						if inner != nil && (inner.Kind() == "type_identifier" || inner.Kind() == "identifier") {
							return graph.GenerateID("Struct", filePath+":"+inner.Utf8Text(source))
						}
					}
				}
			}
		}

		if ClassContainerTypes[current.Kind()] {
			// Rust methods belong to the implemented concrete type. The impl block
			// remains a structural node, but using it as the callable owner prevents
			// receiver-type resolution and method-implementation enrichment.
			if current.Kind() == "impl_item" {
				typeNode := current.ChildByFieldName("type")
				if typeName := extractSimpleTypeName(typeNode, source); typeName != "" {
					return graph.GenerateID("Struct", filePath+":"+typeName)
				}
			}

			nameNode := current.ChildByFieldName("name")
			if nameNode == nil {
				nameNode = findChildByKinds(current, "type_identifier", "identifier", "name")
			}
			if nameNode != nil {
				lbl := ContainerTypeToLabel[current.Kind()]
				if lbl == "" {
					lbl = "Class"
				}
				return graph.GenerateID(lbl, filePath+":"+nameNode.Utf8Text(source))
			}
		}

		current = current.Parent()
	}
	return ""
}

// FindDirectEnclosingClassId returns the class-like owner only when the
// definition is a direct member of that container. A declaration nested inside
// another function or lambda is local state, not a class member, even though a
// class exists farther up the AST. definitionNode must be the outer node for
// the captured definition so the definition's own function node is not treated
// as an intervening scope boundary.
func FindDirectEnclosingClassId(definitionNode *sitter.Node, filePath string, source []byte) string {
	if definitionNode == nil {
		return ""
	}
	for current := definitionNode.Parent(); current != nil; current = current.Parent() {
		if FunctionNodeTypes[current.Kind()] {
			return ""
		}
		if ClassContainerTypes[current.Kind()] {
			if current.Kind() == "class_declaration" {
				for i := uint(0); i < current.ChildCount(); i++ {
					child := current.Child(i)
					if child != nil && !child.IsNamed() && child.Utf8Text(source) == "interface" {
						nameNode := current.ChildByFieldName("name")
						if nameNode == nil {
							nameNode = findChildByKinds(current, "type_identifier", "identifier", "name")
						}
						if nameNode != nil {
							return graph.GenerateID("Interface", filePath+":"+nameNode.Utf8Text(source))
						}
					}
				}
			}
			return FindEnclosingClassId(definitionNode, filePath, source)
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// InferCallForm determines if a call site is a free call, member call, or constructor.
// ─────────────────────────────────────────────────────────────────────────────

type CallForm int

const (
	CallFormFree CallForm = iota
	CallFormMember
	CallFormConstructor
)

// InferCallForm inspects AST structure between callNode and nameNode.
func InferCallForm(callNode, nameNode *sitter.Node, source []byte) CallForm {
	// 1. Constructor: callNode itself is a constructor invocation
	if ConstructorCallNodeTypes[callNode.Kind()] {
		return CallFormConstructor
	}

	// 2. Member call: nameNode's parent is a member-access wrapper
	if nameNode != nil {
		nameParent := nameNode.Parent()
		if nameParent != nil && MemberAccessNodeTypes[nameParent.Kind()] {
			return CallFormMember
		}
	}

	// 3. PHP: callNode type distinguishes member vs free
	if callNode.Kind() == "member_call_expression" || callNode.Kind() == "nullsafe_member_call_expression" {
		return CallFormMember
	}
	if callNode.Kind() == "scoped_call_expression" {
		return CallFormMember // static call Foo::bar()
	}

	// 4. Java method_invocation: member if it has an 'object' field
	if callNode.Kind() == "method_invocation" && callNode.ChildByFieldName("object") != nil {
		return CallFormMember
	}

	// 5. Scoped calls (Rust Foo::new(), C++ ns::func()): treat as free
	if nameNode != nil {
		nameParent := nameNode.Parent()
		if nameParent != nil && ScopedCallNodeTypes[nameParent.Kind()] {
			return CallFormFree
		}

		// 6. Default: if nameNode is a direct child of callNode, it's a free call
		if nameNode.Parent() == callNode || (nameParent != nil && nameParent.Parent() == callNode) {
			return CallFormFree
		}
	}

	return CallFormFree
}

// ─────────────────────────────────────────────────────────────────────────────
// ExtractReceiverName extracts the receiver identifier for member calls.
// Only captures simple identifiers — returns "" for complex expressions.
// ─────────────────────────────────────────────────────────────────────────────

func ExtractReceiverName(nameNode *sitter.Node, source []byte) string {
	if nameNode == nil {
		return ""
	}
	parent := nameNode.Parent()
	if parent == nil {
		return ""
	}

	callNode := parent.Parent()
	if callNode == nil {
		callNode = parent
	}

	var receiver *sitter.Node

	// Try standard field names across grammars
	receiver = parent.ChildByFieldName("object") // TS/JS, Python, PHP, Java
	if receiver == nil {
		receiver = parent.ChildByFieldName("value") // Rust field_expression
	}
	if receiver == nil {
		receiver = parent.ChildByFieldName("operand") // Go selector_expression
	}
	if receiver == nil {
		receiver = parent.ChildByFieldName("expression") // C# member_access_expression
	}
	if receiver == nil {
		receiver = parent.ChildByFieldName("argument") // C++ field_expression
	}

	// Java method_invocation: 'object' field is on the callNode
	if receiver == nil && callNode.Kind() == "method_invocation" {
		receiver = callNode.ChildByFieldName("object")
	}

	// PHP: member_call_expression has 'object' on the call node
	if receiver == nil && (callNode.Kind() == "member_call_expression" || callNode.Kind() == "nullsafe_member_call_expression") {
		receiver = callNode.ChildByFieldName("object")
	}

	// Kotlin/Swift: navigation_expression target
	if receiver == nil && parent.Kind() == "navigation_suffix" {
		navExpr := parent.Parent()
		if navExpr != nil && navExpr.Kind() == "navigation_expression" {
			for i := uint(0); i < navExpr.ChildCount(); i++ {
				child := navExpr.Child(i)
				if child.IsNamed() && child != parent {
					receiver = child
					break
				}
			}
		}
	}

	if receiver == nil {
		return ""
	}

	return extractReceiverBinding(receiver, source)
}

// extractReceiverBinding returns the explicitly typed base variable behind a
// receiver expression. Indexed C/C++ receivers such as keys[i].data() retain
// the element type declared on keys, so resolving the base is safe and avoids
// repository-wide same-name guessing.
func extractReceiverBinding(receiver *sitter.Node, source []byte) string {
	if receiver == nil {
		return ""
	}
	if SimpleReceiverTypes[receiver.Kind()] {
		return receiver.Utf8Text(source)
	}
	if receiver.Kind() == "subscript_expression" {
		return extractReceiverBinding(receiver.ChildByFieldName("argument"), source)
	}
	if receiver.Kind() == "attribute" {
		object := receiver.ChildByFieldName("object")
		attribute := receiver.ChildByFieldName("attribute")
		base := extractReceiverBinding(object, source)
		if base != "" && attribute != nil && attribute.Kind() == "identifier" {
			return base + "." + attribute.Utf8Text(source)
		}
	}
	if receiver.Kind() == "member_expression" {
		object := receiver.ChildByFieldName("object")
		property := receiver.ChildByFieldName("property")
		base := extractReceiverBinding(object, source)
		if base != "" && property != nil && (property.Kind() == "property_identifier" || property.Kind() == "identifier") {
			return base + "." + property.Utf8Text(source)
		}
	}
	if receiver.Kind() == "member_access_expression" {
		object := receiver.ChildByFieldName("object")
		name := receiver.ChildByFieldName("name")
		base := extractReceiverBinding(object, source)
		if base != "" && name != nil && name.Kind() == "name" {
			return base + "->" + name.Utf8Text(source)
		}
	}
	if receiver.Kind() == "field_expression" {
		value := receiver.ChildByFieldName("value")
		if value == nil {
			value = receiver.ChildByFieldName("argument")
		}
		field := receiver.ChildByFieldName("field")
		base := extractReceiverBinding(value, source)
		if base != "" && field != nil && (field.Kind() == "field_identifier" || field.Kind() == "identifier") {
			return base + "." + field.Utf8Text(source)
		}
	}
	return ""
}

// ExtractCppReceiverChain returns the explicit base binding and intermediate
// fluent methods for `base.first().second().terminal()`.
func ExtractCppReceiverChain(nameNode *sitter.Node, source []byte) (string, []string, []int) {
	if nameNode == nil || nameNode.Parent() == nil {
		return "", nil, nil
	}
	receiver := nameNode.Parent().ChildByFieldName("argument")
	var walk func(*sitter.Node) (string, []string, []int)
	walk = func(node *sitter.Node) (string, []string, []int) {
		if node == nil {
			return "", nil, nil
		}
		if SimpleReceiverTypes[node.Kind()] {
			return node.Utf8Text(source), nil, nil
		}
		if node.Kind() == "call_expression" {
			fn := node.ChildByFieldName("function")
			if fn == nil {
				return "", nil, nil
			}
			if SimpleReceiverTypes[fn.Kind()] {
				return fn.Utf8Text(source), nil, nil
			}
			if fn.Kind() == "field_expression" {
				base, methods, argCounts := walk(fn.ChildByFieldName("argument"))
				field := fn.ChildByFieldName("field")
				if base != "" && field != nil {
					return base, append(methods, field.Utf8Text(source)), append(argCounts, CountCallArguments(node))
				}
			}
		}
		return "", nil, nil
	}
	return walk(receiver)
}

// ─────────────────────────────────────────────────────────────────────────────
// CountCallArguments counts direct arguments for a call expression.
// ─────────────────────────────────────────────────────────────────────────────

func CountCallArguments(callNode *sitter.Node) int {
	if callNode == nil {
		return -1
	}

	var argsNode *sitter.Node

	// Direct field or direct child
	argsNode = callNode.ChildByFieldName("arguments")
	if argsNode == nil {
		for i := uint(0); i < callNode.ChildCount(); i++ {
			child := callNode.Child(i)
			if child != nil && CallArgumentListTypes[child.Kind()] {
				argsNode = child
				break
			}
		}
	}

	// Kotlin/Swift: one level deeper for suffix-wrapped arguments
	if argsNode == nil {
		for i := uint(0); i < callNode.ChildCount(); i++ {
			child := callNode.Child(i)
			if !child.IsNamed() {
				continue
			}
			for j := uint(0); j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc != nil && CallArgumentListTypes[gc.Kind()] {
					argsNode = gc
					break
				}
			}
			if argsNode != nil {
				break
			}
		}
	}

	if argsNode == nil {
		return -1
	}

	count := 0
	for i := uint(0); i < argsNode.ChildCount(); i++ {
		child := argsNode.Child(i)
		if !child.IsNamed() {
			continue
		}
		if child.Kind() == "comment" {
			continue
		}
		count++
	}
	return count
}

// ─────────────────────────────────────────────────────────────────────────────
// MethodSignature holds parameter count and return type information.
// ─────────────────────────────────────────────────────────────────────────────

type MethodSignature struct {
	ParameterCount *int
	ReturnType     string
}

// paramListTypes for extractMethodSignature
var paramListTypes = map[string]bool{
	"formal_parameters":         true,
	"parameters":                true,
	"parameter_list":            true,
	"function_parameters":       true,
	"method_parameters":         true,
	"function_value_parameters": true,
	"parameter_clause":          true, // Swift
}

// variadicParamTypes for variadic/rest parameter detection
var variadicParamTypes = map[string]bool{
	"variadic_parameter_declaration": true, // Go: ...string
	"variadic_parameter":             true, // Rust: extern "C" fn(...)
	"spread_parameter":               true, // Java: Object... args
	"list_splat_pattern":             true, // Python: *args
	"dictionary_splat_pattern":       true, // Python: **kwargs
}

// ExtractMethodSignature extracts parameter count and return type from an AST node.
func ExtractMethodSignature(node *sitter.Node, source []byte) MethodSignature {
	parameterCount := 0
	var returnType string
	isVariadic := false

	if node == nil {
		return MethodSignature{ParameterCount: &parameterCount, ReturnType: returnType}
	}

	// Find parameter list
	var parameterList *sitter.Node
	if paramListTypes[node.Kind()] {
		parameterList = node
	} else {
		parameterList = node.ChildByFieldName("parameters")
		if parameterList == nil {
			parameterList = findParameterList(node)
		}
	}

	if parameterList != nil && paramListTypes[parameterList.Kind()] {
		for i := uint(0); i < parameterList.NamedChildCount(); i++ {
			param := parameterList.NamedChild(i)
			if param.Kind() == "comment" {
				continue
			}
			paramText := param.Utf8Text(source)
			if parameterList.NamedChildCount() == 1 && param.Kind() == "parameter_declaration" && strings.TrimSpace(paramText) == "void" {
				continue
			}
			if paramText == "self" || paramText == "&self" || paramText == "&mut self" || param.Kind() == "self_parameter" {
				continue
			}
			if variadicParamTypes[param.Kind()] {
				isVariadic = true
				continue
			}
			// TS/JS: rest parameter
			if param.Kind() == "required_parameter" || param.Kind() == "optional_parameter" {
				for j := uint(0); j < param.ChildCount(); j++ {
					c := param.Child(j)
					if c != nil && c.Kind() == "rest_pattern" {
						isVariadic = true
						break
					}
				}
				if isVariadic {
					continue
				}
			}
			parameterCount++
		}
		// C/C++: bare `...` token
		if !isVariadic {
			for i := uint(0); i < parameterList.ChildCount(); i++ {
				c := parameterList.Child(i)
				if !c.IsNamed() && c.Utf8Text(source) == "..." {
					isVariadic = true
					break
				}
			}
		}
	}
	if parameterList == nil && (node.Kind() == "init_declaration" || node.Kind() == "function_declaration" || node.Kind() == "protocol_function_declaration") {
		for i := uint(0); i < node.NamedChildCount(); i++ {
			if child := node.NamedChild(i); child != nil && child.Kind() == "parameter" {
				parameterCount++
			}
		}
	}

	// Return type extraction
	// Go: 'result' field
	goResult := node.ChildByFieldName("result")
	if goResult != nil {
		returnType = goResult.Utf8Text(source)
	}

	// Rust: 'return_type' field (skip type_annotation)
	if returnType == "" {
		rustReturn := node.ChildByFieldName("return_type")
		if rustReturn != nil && rustReturn.Kind() != "type_annotation" {
			returnType = rustReturn.Utf8Text(source)
		}
	}

	// C/C++: 'type' field
	if returnType == "" {
		cppType := node.ChildByFieldName("type")
		if cppType != nil && cppType.Utf8Text(source) != "void" {
			returnType = cppType.Utf8Text(source)
		}
	}

	// TS/Rust/Python/C#/Kotlin: type_annotation or return_type child
	if returnType == "" {
		for i := uint(0); i < node.ChildCount(); i++ {
			child := node.Child(i)
			if child != nil && (child.Kind() == "type_annotation" || child.Kind() == "return_type") {
				for j := uint(0); j < child.NamedChildCount(); j++ {
					typeNode := child.NamedChild(j)
					if typeNode != nil {
						returnType = typeNode.Utf8Text(source)
						break
					}
				}
			}
		}
	}

	result := MethodSignature{ReturnType: returnType}
	if isVariadic {
		result.ParameterCount = nil // undefined → nil
	} else {
		result.ParameterCount = &parameterCount
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Go export checker: first letter uppercase
// ─────────────────────────────────────────────────────────────────────────────

func IsGoExported(name string) bool {
	if len(name) == 0 {
		return false
	}
	firstRune := rune(name[0])
	return unicode.IsUpper(firstRune) && unicode.IsLetter(firstRune)
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper functions for AST traversal
// ─────────────────────────────────────────────────────────────────────────────

// findChildByKind finds the first named child of the given kind.
func findChildByKind(node *sitter.Node, kind string) *sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.IsNamed() && child.Kind() == kind {
			return child
		}
	}
	return nil
}

// findChildByKinds finds the first named child matching any of the given kinds.
func findChildByKinds(node *sitter.Node, kinds ...string) *sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil || !child.IsNamed() {
			continue
		}
		for _, k := range kinds {
			if child.Kind() == k {
				return child
			}
		}
	}
	return nil
}

// findParameterList recursively searches for a parameter list node.
func findParameterList(node *sitter.Node) *sitter.Node {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && paramListTypes[child.Kind()] {
			return child
		}
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil {
			nested := findParameterList(child)
			if nested != nil {
				return nested
			}
		}
	}
	return nil
}

// FindSiblingChild finds a child of childType within a sibling of siblingType.
// Used for Kotlin AST traversal where visibility_modifier lives inside a modifiers sibling.
func FindSiblingChild(parent *sitter.Node, siblingType, childType string) *sitter.Node {
	for i := uint(0); i < parent.ChildCount(); i++ {
		sibling := parent.Child(i)
		if sibling != nil && sibling.Kind() == siblingType {
			for j := uint(0); j < sibling.ChildCount(); j++ {
				child := sibling.Child(j)
				if child != nil && child.Kind() == childType {
					return child
				}
			}
		}
	}
	return nil
}
