// c_cpp.go — C/C++ variable extraction config.
//
// Handles global/namespace-scoped variable declarations:
//   - `int x = 5;`
//   - `const int MAX = 100;`
//   - `static int counter = 0;`
//   - `constexpr int SIZE = 10;` (C++)
//   - `extern int shared;`
//
// tree-sitter-c/cpp uses declaration for variable declarations.
//
// Ported from TS variable-extractors/configs/c-cpp.ts.
package configs

import (
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	fieldconfigs "github.com/mengshi02/codetrip/internal/ingestion/field_extractors/configs"
)

// CVariableConfig is the variable extraction configuration for C.
var CVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangC,
	ConstNodeTypes:    []string{},
	StaticNodeTypes:   []string{},
	VariableNodeTypes: []string{"declaration"},

	ExtractName:  extractCVarName,
	ExtractNames: extractCVarNames,
	ExtractType:  extractCVarType,

	ExtractVisibility: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) core.VariableVisibility {
		// C/C++ visibility is file-scoped by default (static = file-private)
		if fieldconfigs.HasKeyword(node, "static", lang) {
			return core.VisibilityPrivate
		}
		if fieldconfigs.HasKeyword(node, "extern", lang) {
			return core.VisibilityPublic
		}
		return core.VisibilityPublic
	},

	IsConst: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return fieldconfigs.HasKeyword(node, "const", lang) || fieldconfigs.HasKeyword(node, "constexpr", lang)
	},
	IsStatic: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return fieldconfigs.HasKeyword(node, "static", lang)
	},
	IsMutable: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return !fieldconfigs.HasKeyword(node, "const", lang) && !fieldconfigs.HasKeyword(node, "constexpr", lang)
	},
}

// CppVariableConfig is the variable extraction configuration for C++.
// Shares the same extraction logic as C, but with C++ language tag.
var CppVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangCpp,
	ConstNodeTypes:    []string{},
	StaticNodeTypes:   []string{},
	VariableNodeTypes: []string{"declaration"},

	ExtractName:  extractCVarName,
	ExtractNames: extractCVarNames,
	ExtractType:  extractCVarType,

	ExtractVisibility: CVariableConfig.ExtractVisibility,
	IsConst:           CVariableConfig.IsConst,
	IsStatic:          CVariableConfig.IsStatic,
	IsMutable:         CVariableConfig.IsMutable,
}

// extractCVarName extracts the first variable name from a C/C++ declaration node.
func extractCVarName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "init_declarator" {
			declarator := child.ChildByFieldName("declarator", lang)
			if declarator != nil && declarator.Type(lang) == "identifier" {
				return declarator.Text(source)
			}
			if declarator != nil && declarator.Type(lang) == "pointer_declarator" {
				for j := 0; j < int(declarator.NamedChildCount()); j++ {
					inner := declarator.NamedChild(j)
					if inner != nil && inner.Type(lang) == "identifier" {
						return inner.Text(source)
					}
				}
			}
		}
		if child.Type(lang) == "identifier" {
			return child.Text(source)
		}
	}
	return ""
}

// extractCVarNames extracts all bound names from a C/C++ declaration node.
// Supports structured bindings (auto [a, b] = ...;).
func extractCVarNames(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
	binding := findStructuredBindingDeclarator(node, lang)
	if binding != nil {
		var names []string
		for i := 0; i < int(binding.NamedChildCount()); i++ {
			child := binding.NamedChild(i)
			if child != nil && child.Type(lang) == "identifier" {
				names = append(names, child.Text(source))
			}
		}
		return names
	}

	single := extractCVarName(node, source, lang)
	if single != "" {
		return []string{single}
	}
	return nil
}

// findStructuredBindingDeclarator locates the structured_binding_declarator in a declaration.
// C++ structured bindings (auto [a, b] = pair;) parse as:
//   declaration → init_declarator → structured_binding_declarator → identifier+
func findStructuredBindingDeclarator(node *gotreesitter.Node, lang *gotreesitter.Language) *gotreesitter.Node {
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil || child.Type(lang) != "init_declarator" {
			continue
		}
		declarator := child.ChildByFieldName("declarator", lang)
		if declarator != nil && declarator.Type(lang) == "structured_binding_declarator" {
			return declarator
		}
		// auto& [x, y] → reference_declarator wraps the structured_binding_declarator
		if declarator != nil && declarator.Type(lang) == "reference_declarator" {
			for j := 0; j < int(declarator.NamedChildCount()); j++ {
				inner := declarator.NamedChild(j)
				if inner != nil && inner.Type(lang) == "structured_binding_declarator" {
					return inner
				}
			}
		}
	}
	return nil
}

// extractCVarType extracts the type annotation from a C/C++ declaration node.
func extractCVarType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	typeNode := node.ChildByFieldName("type", lang)
	if typeNode != nil {
		t := strings.TrimSpace(typeNode.Text(source))
		return &t
	}
	// Fallback: first primitive_type or type_identifier child
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "primitive_type" || ct == "type_identifier" || ct == "sized_type_specifier" {
			t := strings.TrimSpace(child.Text(source))
			return &t
		}
	}
	return nil
}