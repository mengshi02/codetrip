package utils

import (
	"github.com/odvcencio/gotreesitter"
)

// ── Call-expression node types ────────────────────────────────────────────

// CallExpressionTypes represents AST node types for call expressions across
// supported languages.
var CallExpressionTypes = map[string]bool{
	"call_expression":                 true, // TS/JS/C/C++/Go/Rust
	"method_invocation":               true, // Java
	"member_call_expression":          true, // PHP
	"nullsafe_member_call_expression": true, // PHP ?.
	"call":                            true, // Python/Ruby
	"invocation_expression":           true, // C#
}

// CallArgumentListTypes are argument list node types shared between
// call-analysis and ast-helpers.
var CallArgumentListTypes = map[string]bool{
	"arguments":               true, // TS/JS/Java/C/C++
	"argument_list":           true, // Python/Go/Rust
	"value_arguments":         true, // Kotlin
	"parenthesized_arguments": true, // Swift
}

// MaxChainDepth is the hard limit on chain depth to prevent runaway recursion.
// For a.b().c().d(), the chain has depth 2 (b and c before d).
const MaxChainDepth = 3

// ── Argument counting ─────────────────────────────────────────────────────

// CountCallArguments counts direct arguments for a call expression across
// common tree-sitter grammars. Returns nil when the argument container
// cannot be located cheaply.
func CountCallArguments(callNode *gotreesitter.Node, lang *gotreesitter.Language) *int {
	if callNode == nil {
		return nil
	}

	// Direct field or direct child (most languages)
	argsNode := callNode.ChildByFieldName("arguments", lang)
	if argsNode == nil {
		// Search children for argument list types
		for i := 0; i < callNode.ChildCount(); i++ {
			child := callNode.Child(i)
			if child != nil && CallArgumentListTypes[child.Type(lang)] {
				argsNode = child
				break
			}
		}
	}

	// Kotlin/Swift: call_expression → call_suffix → value_arguments
	// Search one level deeper for languages that wrap arguments in a suffix node
	if argsNode == nil {
		for i := 0; i < callNode.ChildCount(); i++ {
			child := callNode.Child(i)
			if child == nil || !child.IsNamed() {
				continue
			}
			for j := 0; j < child.ChildCount(); j++ {
				gc := child.Child(j)
				if gc != nil && CallArgumentListTypes[gc.Type(lang)] {
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
		return nil
	}

	count := 0
	for i := 0; i < argsNode.ChildCount(); i++ {
		child := argsNode.Child(i)
		if child == nil || !child.IsNamed() {
			continue
		}
		if child.Type(lang) == "comment" {
			continue
		}
		count++
	}

	return &count
}

// ── Call-form discrimination ──────────────────────────────────────────────

// MemberAccessNodeTypes indicates a member-access wrapper around the callee name.
var MemberAccessNodeTypes = map[string]bool{
	"member_expression":                 true, // TS/JS: obj.method()
	"attribute":                         true, // Python: obj.method()
	"member_access_expression":          true, // C#: obj.Method()
	"field_expression":                  true, // Rust/C++: obj.method() / ptr->method()
	"selector_expression":               true, // Go: obj.Method()
	"navigation_suffix":                 true, // Kotlin/Swift: obj.method()
	"member_binding_expression":         true, // C#: user?.Method()
	"unconditional_assignable_selector": true, // Dart: obj.method()
}

// ConstructorCallNodeTypes are call node types inherently constructor invocations.
var ConstructorCallNodeTypes = map[string]bool{
	"constructor_invocation":              true, // Kotlin: Foo()
	"new_expression":                      true, // TS/JS/C++: new Foo()
	"object_creation_expression":          true, // Java/C#/PHP: new Foo()
	"implicit_object_creation_expression": true, // C# 9: User u = new(...)
	"composite_literal":                   true, // Go: User{...}
	"struct_expression":                   true, // Rust: User { ... }
}

// ScopedCallNodeTypes are AST node types for scoped/qualified calls.
var ScopedCallNodeTypes = map[string]bool{
	"scoped_identifier":    true, // Rust: Foo::new()
	"qualified_identifier": true, // C++: ns::func()
}

// CallForm represents the form of a call site.
type CallForm string

const (
	CallFormFree        CallForm = "free"
	CallFormMember      CallForm = "member"
	CallFormConstructor CallForm = "constructor"
)

// callFormPtr returns a pointer to a CallForm value.
// Needed because Go constants cannot have their address taken.
func callFormPtr(f CallForm) *CallForm { return &f }

// InferCallForm infers whether a captured call site is a free call,
// member call, or constructor. Returns nil if the form cannot be determined.
func InferCallForm(callNode *gotreesitter.Node, nameNode *gotreesitter.Node, lang *gotreesitter.Language) *CallForm {
	// 1. Constructor: callNode itself is a constructor invocation
	if ConstructorCallNodeTypes[callNode.Type(lang)] {
		return callFormPtr(CallFormConstructor)
	}

	// 2. Member call: nameNode's parent is a member-access wrapper
	nameParent := nameNode.Parent()
	if nameParent != nil && MemberAccessNodeTypes[nameParent.Type(lang)] {
		return callFormPtr(CallFormMember)
	}

	// 3. PHP: callNode itself distinguishes member vs free calls
	if callNode.Type(lang) == "member_call_expression" ||
		callNode.Type(lang) == "nullsafe_member_call_expression" {
		return callFormPtr(CallFormMember)
	}
	if callNode.Type(lang) == "scoped_call_expression" {
		return callFormPtr(CallFormMember) // static call Foo::bar()
	}

	// 4. Java method_invocation: member if it has an 'object' field
	if callNode.Type(lang) == "method_invocation" && callNode.ChildByFieldName("object", lang) != nil {
		return callFormPtr(CallFormMember)
	}

	// 4b. Ruby call with receiver: obj.method
	if callNode.Type(lang) == "call" && callNode.ChildByFieldName("receiver", lang) != nil {
		return callFormPtr(CallFormMember)
	}

	// 5. Scoped calls (Rust Foo::new(), C++ ns::func()): treat as free
	if nameParent != nil && ScopedCallNodeTypes[nameParent.Type(lang)] {
		return callFormPtr(CallFormFree)
	}

	// 6. Default: if nameNode is a direct child of callNode, it's a free call
	if nameNode.Parent() == callNode {
		return callFormPtr(CallFormFree)
	}
	if nameParent != nil && nameParent.Parent() == callNode {
		return callFormPtr(CallFormFree)
	}

	return nil
}

// ── Receiver extraction ───────────────────────────────────────────────────

// SimpleReceiverTypes are identifier types accepted as simple receivers.
var SimpleReceiverTypes = map[string]bool{
	"identifier":        true,
	"simple_identifier": true,
	"variable_name":     true, // PHP $variable
	"name":              true, // PHP name node
	"this":              true, // TS/JS/Java/C#
	"self":              true, // Rust/Python
	"super":             true, // TS/JS/Java/Kotlin/Ruby
	"super_expression":  true, // Kotlin wraps super
	"base":              true, // C# base.Method()
	"parent":            true, // PHP parent::method()
	"constant":          true, // Ruby CONSTANT.method()
}

// extractReceiverNode is the shared helper that finds the receiver AST node.
func extractReceiverNode(nameNode *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	parent := nameNode.Parent()
	if parent == nil {
		return nil
	}

	callNode := parent.Parent()
	if callNode == nil {
		callNode = parent
	}

	var receiver *gotreesitter.Node

	// Try standard field names across grammars
	receiver = parent.ChildByFieldName("object", lang)
	if receiver == nil {
		receiver = parent.ChildByFieldName("value", lang)
	}
	if receiver == nil {
		receiver = parent.ChildByFieldName("operand", lang)
	}
	if receiver == nil {
		receiver = parent.ChildByFieldName("expression", lang)
	}
	if receiver == nil {
		receiver = parent.ChildByFieldName("argument", lang)
	}

	// Java method_invocation: 'object' field is on callNode
	if receiver == nil && callNode.Type(lang) == "method_invocation" {
		receiver = callNode.ChildByFieldName("object", lang)
	}

	// PHP: member_call_expression has 'object' on the call node
	if receiver == nil &&
		(callNode.Type(lang) == "member_call_expression" ||
			callNode.Type(lang) == "nullsafe_member_call_expression") {
		receiver = callNode.ChildByFieldName("object", lang)
	}

	// Ruby: call node has 'receiver' field
	if receiver == nil && parent.Type(lang) == "call" {
		receiver = parent.ChildByFieldName("receiver", lang)
	}

	// PHP scoped_call_expression (parent::method(), self::method())
	if receiver == nil &&
		(parent.Type(lang) == "scoped_call_expression" || callNode.Type(lang) == "scoped_call_expression") {
		var scopedCall *gotreesitter.Node
		if parent.Type(lang) == "scoped_call_expression" {
			scopedCall = parent
		} else {
			scopedCall = callNode
		}
		receiver = scopedCall.ChildByFieldName("scope", lang)
		if receiver != nil && receiver.Type(lang) == "relative_scope" {
			receiver = receiver.Child(0) // unwrap to get keyword
		}
	}

	// Dart: unconditional_assignable_selector
	if receiver == nil && parent.Type(lang) == "unconditional_assignable_selector" {
		selectorNode := parent.Parent()
		if selectorNode != nil {
			receiver = prevNamedSibling(selectorNode, nameNode, lang)
		}
	}

	// C# null-conditional: user?.Save()
	if receiver == nil && parent.Type(lang) == "member_binding_expression" {
		condAccess := parent.Parent()
		if condAccess != nil && condAccess.Type(lang) == "conditional_access_expression" {
			receiver = firstNamedChild(condAccess, lang)
		}
	}

	// Kotlin/Swift: navigation_expression target
	if receiver == nil && parent.Type(lang) == "navigation_suffix" {
		navExpr := parent.Parent()
		if navExpr != nil && navExpr.Type(lang) == "navigation_expression" {
			for i := 0; i < navExpr.NamedChildCount(); i++ {
				child := navExpr.NamedChild(i)
				if child != nil && child != parent {
					receiver = child
					break
				}
			}
		}
	}

	return receiver
}

// firstNamedChild returns the first named child of a node.
// gotreesitter does not expose FirstNamedChild() directly.
func firstNamedChild(n *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if n == nil {
		return nil
	}
	for i := 0; i < n.ChildCount(); i++ {
		child := n.Child(i)
		if child != nil && child.IsNamed() {
			return child
		}
	}
	return nil
}

// prevNamedSibling returns the previous named sibling of target in parent's children.
// gotreesitter does not expose PrevNamedSibling() directly.
func prevNamedSibling(parent *gotreesitter.Node, target *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if parent == nil || target == nil {
		return nil
	}
	var prev *gotreesitter.Node
	for i := 0; i < parent.ChildCount(); i++ {
		child := parent.Child(i)
		if child == target {
			return prev
		}
		if child != nil && child.IsNamed() {
			prev = child
		}
	}
	return nil
}

// ExtractReceiverName extracts the receiver identifier for member calls.
// Only captures simple identifiers — returns nil for complex expressions.
func ExtractReceiverName(nameNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	receiver := extractReceiverNode(nameNode, lang)
	if receiver == nil {
		return nil
	}

	// Only capture simple identifiers
	if SimpleReceiverTypes[receiver.Type(lang)] {
		text := receiver.Text(source)
		return &text
	}

	// Python super().method(): receiver is call node `super()`
	if receiver.Type(lang) == "call" {
		funcNode := receiver.ChildByFieldName("function", lang)
		if funcNode != nil && funcNode.Text(source) == "super" {
			s := "super"
			return &s
		}
	}

	return nil
}

// ExtractReceiverNode extracts the raw receiver AST node for a member call.
// Unlike ExtractReceiverName, returns the node regardless of its type.
func ExtractReceiverNode(nameNode *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	return extractReceiverNode(nameNode, lang)
}

// ── Chained-call extraction ───────────────────────────────────────────────

// FieldAccessNodeTypes represents member/field access node types across languages.
var FieldAccessNodeTypes = map[string]bool{
	"member_expression":         true, // TS/JS
	"member_access_expression":  true, // C#
	"selector_expression":       true, // Go
	"field_expression":          true, // Rust/C++
	"field_access":              true, // Java
	"attribute":                 true, // Python
	"navigation_expression":     true, // Kotlin/Swift
	"member_binding_expression": true, // C# null-conditional
}

// MixedChainStep represents one step in a mixed receiver chain.
type MixedChainStep struct {
	Kind string // "field" or "call"
	Name string
}

// CallChainResult holds the result of extracting a call chain.
type CallChainResult struct {
	Chain            []string
	BaseReceiverName *string
}

// MixedChainResult holds the result of extracting a mixed chain.
type MixedChainResult struct {
	Chain            []MixedChainStep
	BaseReceiverName *string
}

// lastNamedChild returns the last named child of a node.
// gotreesitter does not expose LastNamedChild() directly.
func lastNamedChild(n *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if n == nil {
		return nil
	}
	count := n.NamedChildCount()
	if count == 0 {
		return nil
	}
	return n.NamedChild(count - 1)
}

// prevNamedSiblingOf returns the previous named sibling of the given node.
// gotreesitter does not expose PrevNamedSibling() directly.
func prevNamedSiblingOf(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	parent := node.Parent()
	if parent == nil {
		return nil
	}
	var prev *gotreesitter.Node
	for i := 0; i < parent.ChildCount(); i++ {
		child := parent.Child(i)
		if child == node {
			return prev
		}
		if child != nil && child.IsNamed() {
			prev = child
		}
	}
	return nil
}

// ExtractCallChain walks a receiver AST node that is itself a call expression,
// accumulating the chain of intermediate method names up to MaxChainDepth.
func ExtractCallChain(receiverCallNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *CallChainResult {
	chain := make([]string, 0)
	current := receiverCallNode

	for CallExpressionTypes[current.Type(lang)] && len(chain) < MaxChainDepth {
		// Extract the method name from this call node
		funcNode := current.ChildByFieldName("function", lang)
		if funcNode == nil {
			funcNode = current.ChildByFieldName("name", lang)
		}
		if funcNode == nil {
			funcNode = current.ChildByFieldName("method", lang) // Ruby
		}

		var methodName *string
		var innerReceiver *gotreesitter.Node

		if funcNode != nil {
			lnc := lastNamedChild(funcNode, lang)
			text := funcNode.Text(source)
			if lnc != nil {
				text = lnc.Text(source)
			}
			methodName = &text
		}

		// Kotlin/Swift: call_expression → navigation_expression
		if funcNode == nil && current.Type(lang) == "call_expression" {
			callee := firstNamedChild(current, lang)
			if callee != nil && callee.Type(lang) == "navigation_expression" {
				suffix := lastNamedChild(callee, lang)
				if suffix != nil && suffix.Type(lang) == "navigation_suffix" {
					suffixLast := lastNamedChild(suffix, lang)
					if suffixLast != nil {
						text := suffixLast.Text(source)
						methodName = &text
					}
					// Find the receiver part of navigation_expression
					for i := 0; i < callee.NamedChildCount(); i++ {
						child := callee.NamedChild(i)
						if child != nil && child.Type(lang) != "navigation_suffix" {
							innerReceiver = child
							break
						}
					}
				}
			}
		}

		if methodName == nil {
			break
		}
		chain = append([]string{*methodName}, chain...) // prepend (outermost-last)

		// Walk into the receiver of this call
		if innerReceiver == nil && funcNode != nil {
			innerReceiver = funcNode.ChildByFieldName("object", lang)
			if innerReceiver == nil {
				innerReceiver = funcNode.ChildByFieldName("value", lang)
			}
			if innerReceiver == nil {
				innerReceiver = funcNode.ChildByFieldName("operand", lang)
			}
			if innerReceiver == nil {
				innerReceiver = funcNode.ChildByFieldName("expression", lang)
			}
		}
		// Java method_invocation: object field is on the call node
		if innerReceiver == nil && current.Type(lang) == "method_invocation" {
			innerReceiver = current.ChildByFieldName("object", lang)
		}
		// PHP member_call_expression
		if innerReceiver == nil &&
			(current.Type(lang) == "member_call_expression" ||
				current.Type(lang) == "nullsafe_member_call_expression") {
			innerReceiver = current.ChildByFieldName("object", lang)
		}
		// Ruby: receiver field on call node
		if innerReceiver == nil && current.Type(lang) == "call" {
			innerReceiver = current.ChildByFieldName("receiver", lang)
		}

		if innerReceiver == nil {
			break
		}

		if CallExpressionTypes[innerReceiver.Type(lang)] {
			current = innerReceiver // continue walking
		} else {
			text := innerReceiver.Text(source)
			return &CallChainResult{Chain: chain, BaseReceiverName: &text}
		}
	}

	if len(chain) > 0 {
		return &CallChainResult{Chain: chain, BaseReceiverName: nil}
	}
	return nil
}

// ExtractMixedChain walks a receiver AST node that may interleave field
// accesses and method calls, building a unified chain up to MaxChainDepth.
func ExtractMixedChain(receiverNode *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *MixedChainResult {
	chain := make([]MixedChainStep, 0)
	current := receiverNode

	for len(chain) < MaxChainDepth {
		if CallExpressionTypes[current.Type(lang)] {
			// ── Call expression ────────────────────────
			funcNode := current.ChildByFieldName("function", lang)
			if funcNode == nil {
				funcNode = current.ChildByFieldName("name", lang)
			}
			if funcNode == nil {
				funcNode = current.ChildByFieldName("method", lang)
			}

			var methodName *string
			var innerReceiver *gotreesitter.Node

			if funcNode != nil {
				lnc := lastNamedChild(funcNode, lang)
				text := funcNode.Text(source)
				if lnc != nil {
					text = lnc.Text(source)
				}
				methodName = &text
			}

			// Kotlin/Swift: navigation_expression inside call_expression
			if funcNode == nil && current.Type(lang) == "call_expression" {
				callee := firstNamedChild(current, lang)
				if callee != nil && callee.Type(lang) == "navigation_expression" {
					suffix := lastNamedChild(callee, lang)
					if suffix != nil && suffix.Type(lang) == "navigation_suffix" {
						suffixLast := lastNamedChild(suffix, lang)
						if suffixLast != nil {
							text := suffixLast.Text(source)
							methodName = &text
						}
						for i := 0; i < callee.NamedChildCount(); i++ {
							child := callee.NamedChild(i)
							if child != nil && child.Type(lang) != "navigation_suffix" {
								innerReceiver = child
								break
							}
						}
					}
				}
			}

			if methodName == nil {
				break
			}
			// prepend
			chain = append([]MixedChainStep{{Kind: "call", Name: *methodName}}, chain...)

			if innerReceiver == nil && funcNode != nil {
				innerReceiver = funcNode.ChildByFieldName("object", lang)
				if innerReceiver == nil {
					innerReceiver = funcNode.ChildByFieldName("value", lang)
				}
				if innerReceiver == nil {
					innerReceiver = funcNode.ChildByFieldName("operand", lang)
				}
				if innerReceiver == nil {
					innerReceiver = funcNode.ChildByFieldName("argument", lang)
				}
				if innerReceiver == nil {
					innerReceiver = funcNode.ChildByFieldName("expression", lang)
				}
			}
			if innerReceiver == nil && current.Type(lang) == "method_invocation" {
				innerReceiver = current.ChildByFieldName("object", lang)
			}
			if innerReceiver == nil &&
				(current.Type(lang) == "member_call_expression" ||
					current.Type(lang) == "nullsafe_member_call_expression") {
				innerReceiver = current.ChildByFieldName("object", lang)
			}
			if innerReceiver == nil && current.Type(lang) == "call" {
				innerReceiver = current.ChildByFieldName("receiver", lang)
			}

			if innerReceiver == nil {
				break
			}

			if CallExpressionTypes[innerReceiver.Type(lang)] || FieldAccessNodeTypes[innerReceiver.Type(lang)] {
				current = innerReceiver
			} else {
				text := innerReceiver.Text(source)
				return &MixedChainResult{Chain: chain, BaseReceiverName: &text}
			}

		} else if FieldAccessNodeTypes[current.Type(lang)] {
			// ── Field/member access ────────────────────────
			var propertyName *string
			var innerObject *gotreesitter.Node

			if current.Type(lang) == "navigation_expression" {
				for i := 0; i < current.ChildCount(); i++ {
					child := current.Child(i)
					if child == nil {
						continue
					}
					if child.Type(lang) == "navigation_suffix" {
						for j := 0; j < child.ChildCount(); j++ {
							sc := child.Child(j)
							if sc != nil && sc.IsNamed() && sc.Type(lang) != "." {
								text := sc.Text(source)
								propertyName = &text
								break
							}
						}
					} else if child.IsNamed() && innerObject == nil {
						innerObject = child
					}
				}
			} else if current.Type(lang) == "attribute" {
				innerObject = current.ChildByFieldName("object", lang)
				attrNode := current.ChildByFieldName("attribute", lang)
				if attrNode != nil {
					text := attrNode.Text(source)
					propertyName = &text
				}
			} else {
				innerObject = current.ChildByFieldName("object", lang)
				if innerObject == nil {
					innerObject = current.ChildByFieldName("value", lang)
				}
				if innerObject == nil {
					innerObject = current.ChildByFieldName("operand", lang)
				}
				if innerObject == nil {
					innerObject = current.ChildByFieldName("argument", lang)
				}
				if innerObject == nil {
					innerObject = current.ChildByFieldName("expression", lang)
				}
				propNode := current.ChildByFieldName("property", lang)
				if propNode == nil {
					propNode = current.ChildByFieldName("field", lang)
				}
				if propNode == nil {
					propNode = current.ChildByFieldName("name", lang)
				}
				if propNode != nil {
					text := propNode.Text(source)
					propertyName = &text
				}
			}

			if propertyName == nil {
				break
			}
			// prepend
			chain = append([]MixedChainStep{{Kind: "field", Name: *propertyName}}, chain...)

			if innerObject == nil {
				break
			}

			if CallExpressionTypes[innerObject.Type(lang)] || FieldAccessNodeTypes[innerObject.Type(lang)] {
				current = innerObject
			} else {
				text := innerObject.Text(source)
				return &MixedChainResult{Chain: chain, BaseReceiverName: &text}
			}

		} else if current.Type(lang) == "selector" {
			// ── Dart: flat selector siblings ──────────────
			var uas *gotreesitter.Node
			for i := 0; i < current.NamedChildCount(); i++ {
				c := current.NamedChild(i)
				if c != nil && c.Type(lang) == "unconditional_assignable_selector" {
					uas = c
					break
				}
			}
			var propName *string
			if uas != nil {
				for i := 0; i < uas.NamedChildCount(); i++ {
					c := uas.NamedChild(i)
					if c != nil && c.Type(lang) == "identifier" {
						text := c.Text(source)
						propName = &text
						break
					}
				}
			}
			if propName == nil {
				break
			}
			chain = append([]MixedChainStep{{Kind: "field", Name: *propName}}, chain...)

			prev := prevNamedSiblingOf(current, lang)
			if prev == nil {
				break
			}
			if prev.Type(lang) == "selector" {
				current = prev
			} else {
				text := prev.Text(source)
				return &MixedChainResult{Chain: chain, BaseReceiverName: &text}
			}

		} else {
			// Simple identifier — base receiver
			if len(chain) > 0 {
				text := current.Text(source)
				return &MixedChainResult{Chain: chain, BaseReceiverName: &text}
			}
			return nil
		}
	}

	if len(chain) > 0 {
		return &MixedChainResult{Chain: chain, BaseReceiverName: nil}
	}
	return nil
}

// ── Call argument type extraction ──────────────────────────────────────────

// ExtractCallArgTypes extracts argument types per call position.
// Returns nil if no argument list can be found or all types are unknown.
func ExtractCallArgTypes(
	callNode *gotreesitter.Node,
	source []byte,
	lang *gotreesitter.Language,
	inferLiteralType func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string,
	typeEnvLookup func(varName string, callNode *gotreesitter.Node) *string,
) []interface{} {
	var argList *gotreesitter.Node

	argList = callNode.ChildByFieldName("arguments", lang)
	if argList == nil {
		// Search children
		for i := 0; i < callNode.ChildCount(); i++ {
			c := callNode.Child(i)
			if c != nil &&
				(c.Type(lang) == "arguments" || c.Type(lang) == "argument_list" || c.Type(lang) == "value_arguments") {
				argList = c
				break
			}
		}
	}
	if argList == nil {
		// Kotlin: call_suffix → value_arguments
		for i := 0; i < callNode.ChildCount(); i++ {
			c := callNode.Child(i)
			if c != nil && c.Type(lang) == "call_suffix" {
				for j := 0; j < c.ChildCount(); j++ {
					gc := c.Child(j)
					if gc != nil && gc.Type(lang) == "value_arguments" {
						argList = gc
						break
					}
				}
				break
			}
		}
	}
	if argList == nil {
		return nil
	}

	argTypes := make([]interface{}, 0)
	for i := 0; i < argList.NamedChildCount(); i++ {
		arg := argList.NamedChild(i)
		if arg == nil || arg.Type(lang) == "comment" {
			continue
		}
		// Get the value node
		valueNode := arg.ChildByFieldName("value", lang)
		if valueNode == nil {
			valueNode = arg.ChildByFieldName("expression", lang)
		}
		if valueNode == nil && (arg.Type(lang) == "argument" || arg.Type(lang) == "value_argument") {
			valueNode = firstNamedChild(arg, lang)
			if valueNode == nil {
				valueNode = arg
			}
		} else if valueNode == nil {
			valueNode = arg
		}

		var inferred *string
		inferred = inferLiteralType(valueNode, source, lang)
		if inferred == nil && typeEnvLookup != nil && valueNode.Type(lang) == "identifier" {
			inferred = typeEnvLookup(valueNode.Text(source), callNode)
		}
		argTypes = append(argTypes, inferred) // nil means unknown type
	}

	// Return nil if ALL types are unknown
	allUnknown := true
	for _, t := range argTypes {
		if t != nil {
			allUnknown = false
			break
		}
	}
	if allUnknown {
		return nil
	}
	return argTypes
}
