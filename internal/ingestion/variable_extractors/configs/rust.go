// rust.go — Rust variable extraction config.
//
// Handles module-scoped const, static, and let declarations:
//   - `const MAX_SIZE: usize = 100;`
//   - `static COUNTER: AtomicUsize = AtomicUsize::new(0);`
//   - `static mut BUFFER: Vec<u8> = Vec::new();`
//   - `let x = 5;` (block-scoped)
//
// tree-sitter-rust uses:
//   - const_item → name, type
//   - static_item → name, type
//   - let_declaration → identifier, type
//
// Ported from TS variable-extractors/configs/rust.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// rustHasVisibilityModifier checks if the node has a visibility_modifier child.
func rustHasVisibilityModifier(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(lang) == "visibility_modifier" {
			return true
		}
	}
	return false
}

// rustHasMutKeyword checks if the node has a `mut` keyword among its direct children.
func rustHasMutKeyword(node *gotreesitter.Node, lang *gotreesitter.Language) bool {
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child != nil && !child.IsNamed() && strings.TrimSpace(child.Type(lang)) == "mut" {
			return true
		}
	}
	return false
}

// RustVariableConfig is the variable extraction configuration for Rust.
var RustVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangRust,
	ConstNodeTypes:    []string{"const_item"},
	StaticNodeTypes:   []string{"static_item"},
	VariableNodeTypes: []string{"let_declaration"},

	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
		name := node.ChildByFieldName("name", lang)
		if name != nil {
			return name.Text(source)
		}
		// Fallback: first identifier child
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "identifier" {
				return child.Text(source)
			}
		}
		return ""
	},

	ExtractType: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		typeNode := node.ChildByFieldName("type", lang)
		if typeNode != nil {
			if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
				return t
			}
			trimmed := strings.TrimSpace(typeNode.Text(source))
			return &trimmed
		}
		return nil
	},

	ExtractVisibility: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) core.VariableVisibility {
		if rustHasVisibilityModifier(node, lang) {
			return core.VisibilityPublic
		}
		return core.VisibilityPrivate
	},

	IsConst: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return node.Type(lang) == "const_item"
	},

	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return node.Type(lang) == "static_item"
	},

	IsMutable: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		nodeType := node.Type(lang)
		if nodeType == "const_item" {
			return false
		}
		if nodeType == "static_item" {
			return rustHasMutKeyword(node, lang)
		}
		// let_declaration: check for mut keyword
		if nodeType == "let_declaration" {
			return rustHasMutKeyword(node, lang)
		}
		return true
	},
}