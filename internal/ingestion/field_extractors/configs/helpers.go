package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// HasKeyword checks whether a direct (unnamed) child of node has the given keyword text.
// This handles C/C++ and C# style keywords that appear as bare tokens
// (e.g. "static", "virtual", "abstract", "sealed", "async", "override", "partial").
func HasKeyword(node *gotreesitter.Node, keyword string, lang *gotreesitter.Language) bool {
	if node == nil {
		return false
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if !child.IsNamed() {
			if strings.TrimSpace(child.Type(lang)) == keyword {
				return true
			}
		}
	}
	return false
}

// HasModifier checks whether the node has a named child of modifierType
// that contains a sub-child with the given keyword text.
// This handles Java/C# style modifiers wrapped in a container node
// (e.g. modifiers { static }, modifier { virtual }).
func HasModifier(node *gotreesitter.Node, modifierType string, keyword string, lang *gotreesitter.Language) bool {
	if node == nil {
		return false
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != modifierType {
			continue
		}
		for j := 0; j < int(child.ChildCount()); j++ {
			mod := child.Child(j)
			if mod == nil {
				continue
			}
			text := strings.TrimSpace(mod.Type(lang))
			if !mod.IsNamed() && text == keyword {
				return true
			}
		}
	}
	return false
}

// FindVisibility searches for a visibility keyword in modifier children.
// It looks for the modifierType named child, then scans its sub-children
// for any visibility keyword present in visSet. Returns defaultVis if not found.
func FindVisibility(
	node *gotreesitter.Node,
	visSet map[core.FieldVisibility]bool,
	defaultVis core.FieldVisibility,
	modifierType string,
	lang *gotreesitter.Language,
) core.FieldVisibility {
	if node == nil {
		return defaultVis
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != modifierType {
			continue
		}
		for j := 0; j < int(child.ChildCount()); j++ {
			mod := child.Child(j)
			if mod == nil {
				continue
			}
			text := strings.TrimSpace(mod.Type(lang))
			if visSet[core.FieldVisibility(text)] {
				return core.FieldVisibility(text)
			}
		}
	}
	return defaultVis
}

// CollectModifierTexts collects all modifier keyword texts from modifier children
// of the given type. Returns a set (map[string]bool) of found modifier texts.
func CollectModifierTexts(node *gotreesitter.Node, modifierType string, lang *gotreesitter.Language) map[string]bool {
	result := make(map[string]bool)
	if node == nil {
		return result
	}
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != modifierType {
			continue
		}
		for j := 0; j < int(child.ChildCount()); j++ {
			mod := child.Child(j)
			if mod == nil {
				continue
			}
			text := strings.TrimSpace(mod.Type(lang))
			if text != "" {
				result[text] = true
			}
		}
	}
	return result
}

// TypeFromField extracts a type string from a named field child of the node.
// It first tries extractSimpleTypeName for a qualified name, then falls back to raw text.
// Ported from TS field-extractors/configs/helpers.ts typeFromField.
func TypeFromField(node *gotreesitter.Node, fieldName string, source []byte, lang *gotreesitter.Language) *string {
	typeNode := node.ChildByFieldName(fieldName, lang)
	if typeNode == nil {
		return nil
	}
	if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
		return t
	}
	trimmed := strings.TrimSpace(typeNode.Text(source))
	return &trimmed
}
// Used by TypeScript/JavaScript extractors to pull type info from annotations.
// Ported from TS field-extractors/configs/helpers.ts typeFromAnnotation.
func TypeFromAnnotation(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "type_annotation" {
			inner := child.NamedChild(0)
			if inner != nil {
				if t := typeextractors.ExtractSimpleTypeNameFromNode(inner, source, lang, 0); t != nil {
					return t
				}
				trimmed := strings.TrimSpace(inner.Text(source))
				return &trimmed
			}
		}
	}
	return nil
}

// PrevNamedSibling returns the previous named sibling of a node.
// gotreesitter does not expose PrevNamedSibling() directly.
func PrevNamedSibling(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	if node == nil {
		return nil
	}
	parent := node.Parent()
	if parent == nil {
		return nil
	}
	var prev *gotreesitter.Node
	for i := 0; i < int(parent.ChildCount()); i++ {
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