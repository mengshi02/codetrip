// Package typescript — Array higher-order-method callback detection (issue #1876).
//
// The HOC-wrapped-arrow declaration pattern in the JS/TS scope queries
// (`const X = call((args) => …)`) was added for React idioms
// (forwardRef / memo / useCallback). It has the same AST shape as
// an array higher-order-method call (`const x = arr.map(a => …)`),
// so those callbacks also match and produce a spurious @declaration.function
// named after the binding — duplicating the @declaration.const /
// @declaration.variable def that the same binding already gets.
//
// For an array-method callback the binding holds a *value* (the method's
// result), not a callable, so the Function def is semantically wrong.
// IsArrayMethodCallbackArrow lets the emitter (captures.go) drop that
// @declaration.function match, leaving only the value def.
//
// Shared by both the JavaScript and TypeScript capture emitters — the
// relevant grammar nodes (arrow_function, function_expression, arguments,
// call_expression, member_expression, property_identifier) are identical
// across tree-sitter-javascript and tree-sitter-typescript.
//
// Ported from TS languages/typescript/array-callback.ts.
package typescript

// ArrayCallbackMethods is the set of Array.prototype methods whose callback
// argument produces a value (the method's result), not a callable binding.
// Ported from ts-js-hoc-utils.ts ARRAY_CALLBACK_METHODS.
var ArrayCallbackMethods = map[string]bool{
	"forEach":    true,
	"map":        true,
	"filter":     true,
	"reduce":     true,
	"reduceRight": true,
	"find":       true,
	"findIndex":  true,
	"findLast":   true,
	"findLastIndex": true,
	"some":       true,
	"every":      true,
	"flatMap":    true,
	"sort":       true,
	"flat":       true,
}

// IsArrayMethodCallbackArrow returns true when node (an arrow_function or
// function_expression) is the callback argument of an array higher-order-method
// call, i.e. the enclosing call's callee is a member_expression whose property
// is one of ArrayCallbackMethods.
//
// Returns false for direct assignments (`const fn = () => {}` — parent is
// variable_declarator, not arguments) and for identifier-callee HOCs
// (`forwardRef(() => …)` — callee is an identifier, not a member_expression),
// so neither is ever suppressed.
//
// node is an interface{} placeholder for tree-sitter SyntaxNode — full
// implementation will use typed tree-sitter Node when the binding is available.
func IsArrayMethodCallbackArrow(node interface{}) bool {
	// TODO: full implementation — walk parent chain:
	//   1. node.parent must be "arguments"
	//   2. arguments.parent must be "call_expression"
	//   3. call_expression.function must be "member_expression"
	//   4. member_expression.property must be "property_identifier"
	//   5. property.text must be in ArrayCallbackMethods
	return false
}