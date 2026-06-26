package configs

import (
	"github.com/odvcencio/gotreesitter"
)

// strPtr returns a pointer to the given string.
func strPtr(s string) *string { return &s }

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

// prevNamedSibling returns the previous named sibling of target by walking
// the parent's children. gotreesitter does not expose PrevNamedSibling() directly.
func prevNamedSibling(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
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

// prevSibling returns the previous sibling (named or anonymous) of a node.
// gotreesitter does not expose PrevSibling() directly.
func prevSibling(node *gotreesitter.Node) *gotreesitter.Node {
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
		prev = child
	}
	return nil
}