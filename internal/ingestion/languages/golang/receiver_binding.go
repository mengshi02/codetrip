// Package golang — Go method receiver binding synthesis.
// Go method declarations have an explicit receiver parameter:
//
//	func (r *T) Method() { ... }
//
// The receiver type T creates a synthetic @type-binding.self capture
// that is used to link the method's Function scope to the owning type.
// Ported from TS languages/go/receiver-binding.ts.
package golang

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/utils"
	"github.com/odvcencio/gotreesitter"
)

// SynthesizeGoReceiverBindingFromNode extracts the receiver type from a
// method_declaration AST node and returns a synthetic CaptureMatch with
// @type-binding.self and @type-binding.name entries.
//
// For `func (r *MyType) Method()`, this produces:
//   - @type-binding.self → anchored on the method_declaration
//   - @type-binding.name → "MyType"
//
// Mirrors TS synthesizeGoReceiverBinding(node).
func SynthesizeGoReceiverBindingFromNode(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) CaptureMatch {
	if node == nil || node.Type(lang) != "method_declaration" {
		return nil
	}

	// The receiver is the first named child of type parameter_list
	// In Go's tree-sitter grammar, the receiver is a parameter_declaration
	// inside the method_declaration's receiver field.
	receiverNode := node.ChildByFieldName("receiver", lang)
	if receiverNode == nil {
		return nil
	}

	// Extract the type from the receiver parameter.
	// Tree-sitter Go grammar: receiver → parameter_list → parameter_declaration
	// The type is in the "type" field of parameter_declaration.
	typeName := extractReceiverTypeName(lang, receiverNode, source)
	if typeName == "" {
		return nil
	}

	return CaptureMatch{
		"@type-binding.self": utils.SyntheticCapture("@type-binding.self", node, node.Text(source)),
		"@type-binding.name": utils.SyntheticCapture("@type-binding.name", receiverNode, typeName),
	}
}

// extractReceiverTypeName walks the receiver parameter list to find the type name.
// Handles: (r T), (r *T), (r []T), (r *[]T), etc.
func extractReceiverTypeName(lang *gotreesitter.Language, receiverNode *gotreesitter.Node, source []byte) string {
	// receiver field is a parameter_list; walk its children to find the type
	for i := 0; i < int(receiverNode.NamedChildCount()); i++ {
		param := receiverNode.NamedChild(i)
		if param == nil {
			continue
		}
		// parameter_declaration has a "type" field
		typeNode := param.ChildByFieldName("type", lang)
		if typeNode != nil {
			return normalizeReceiverTypeName(typeNode.Text(source))
		}
		// Fallback: if no "type" field, the last child is typically the type
		// for simple receivers like (t MyType)
		lastIdx := int(param.NamedChildCount()) - 1
		if lastIdx >= 0 {
			lastChild := param.NamedChild(lastIdx)
			if lastChild != nil {
				return normalizeReceiverTypeName(lastChild.Text(source))
			}
		}
	}
	return ""
}

// normalizeReceiverTypeName strips pointer/pointer wrappers from receiver type names.
// *MyType → MyType, **MyType → MyType, etc.
func normalizeReceiverTypeName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimPrefix(name, "*")
	name = strings.TrimSpace(name)
	return name
}