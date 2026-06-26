package configs

import (
	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// GoClassConfig extracts class-like declarations from Go source code.
// Go has no classes; "type X struct" → Struct, "type X interface" → Interface.
var GoClassConfig = core.ClassExtractionConfig{
	Language:             core.LangGo,
	TypeDeclarationNodes: []string{"type_declaration"},
	FileScopeNodeTypes:   []string{"package_clause"},
	ExtractName: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
		// Find the type_spec child and extract its name field.
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "type_spec" {
				if nameNode := child.ChildByFieldName("name", lang); nameNode != nil {
					t := nameNode.Text(source)
					return &t
				}
			}
		}
		return nil
	},
	ExtractType: func(node *gotreesitter.Node, lang *gotreesitter.Language) *core.ClassLikeNodeLabel {
		// Find the type_spec child and check the type field.
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child != nil && child.Type(lang) == "type_spec" {
				if typeNode := child.ChildByFieldName("type", lang); typeNode != nil {
					switch typeNode.Type(lang) {
					case "struct_type":
						lbl := core.NodeLabelStruct
						return &lbl
					case "interface_type":
						lbl := core.NodeLabelInterface
						return &lbl
					}
				}
			}
		}
		return nil
	},
}