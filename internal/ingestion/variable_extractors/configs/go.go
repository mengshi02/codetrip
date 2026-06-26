// Package configs provides per-language variable extraction configurations.
//
// go.go — Go variable extraction config.
//
// Go has package-scoped var and const declarations:
//   - `var x int = 5`
//   - `const MaxSize = 100`
//   - `var ( ... )` grouped declarations
//
// tree-sitter-go uses:
//   - var_declaration → var_spec → identifier, type
//   - const_declaration → const_spec → identifier, type
//
// Visibility: uppercase first letter = exported (public), lowercase = unexported (package).
//
// Ported from TS variable-extractors/configs/go.ts.
package configs

import (
	"strings"
	"unicode"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
)

// Ensure strings is used.
var _ = strings.TrimSpace

// GoVariableConfig is the variable extraction configuration for Go.
var GoVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangGo,
	ConstNodeTypes:    []string{"const_declaration"},
	StaticNodeTypes:   []string{},
	VariableNodeTypes: []string{"var_declaration", "short_var_declaration"},

	ExtractName:   extractGoVarName,
	ExtractNames:  collectGoDeclarationNames,
	ExtractType:   extractGoVarType,
	ExtractTypeForName: func(node *gotreesitter.Node, name string, source []byte, lang *gotreesitter.Language) *string {
		spec := findGoSpecForName(node, source, lang)
		if spec != nil {
			return extractGoSpecType(spec, source, lang)
		}
		return nil
	},
	ExtractVisibility: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.VariableVisibility {
		name := extractGoVarName(node, source, lang)
		if name == "" {
			return core.VisibilityPackage
		}
		return goVisibilityForName(name)
	},
	ExtractVisibilityForName: func(_ *gotreesitter.Node, name string, _ []byte, _ *gotreesitter.Language) core.VariableVisibility {
		return goVisibilityForName(name)
	},
	IsConst: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return node.Type(lang) == "const_declaration"
	},
	IsStatic: func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool {
		return false // Go does not have static declarations
	},
	IsMutable: func(node *gotreesitter.Node, _ []byte, lang *gotreesitter.Language) bool {
		return node.Type(lang) != "const_declaration"
	},
}

// goVisibilityForName determines Go visibility from the first character:
// uppercase = exported (public), lowercase = unexported (package).
func goVisibilityForName(name string) core.VariableVisibility {
	if len(name) == 0 {
		return core.VisibilityPackage
	}
	firstChar := rune(name[0])
	if unicode.IsUpper(firstChar) {
		return core.VisibilityPublic
	}
	return core.VisibilityPackage
}

// collectGoSpecNames extracts all identifier names from a var_spec or const_spec node.
func collectGoSpecNames(spec *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
	var names []string
	for i := 0; i < int(spec.NamedChildCount()); i++ {
		child := spec.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "identifier" {
			names = append(names, child.Text(source))
			continue
		}
		break
	}
	return names
}

// collectGoSpecs collects all var_spec and const_spec nodes from a declaration.
func collectGoSpecs(node *gotreesitter.Node, lang *gotreesitter.Language) []*gotreesitter.Node {
	var specs []*gotreesitter.Node
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "var_spec" || child.Type(lang) == "const_spec" {
			specs = append(specs, child)
			continue
		}
		if child.Type(lang) == "var_spec_list" {
			specs = append(specs, collectGoSpecs(child, lang)...)
		}
	}
	return specs
}

// collectGoDeclarationNames extracts all variable names from a Go declaration node.
func collectGoDeclarationNames(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) []string {
	if node.Type(lang) == "short_var_declaration" {
		left := node.ChildByFieldName("left", lang)
		if left == nil || left.Type(lang) != "expression_list" {
			return nil
		}
		var names []string
		for i := 0; i < int(left.NamedChildCount()); i++ {
			child := left.NamedChild(i)
			if child != nil && child.Type(lang) == "identifier" {
				names = append(names, child.Text(source))
			}
		}
		return names
	}

	var names []string
	for _, spec := range collectGoSpecs(node, lang) {
		names = append(names, collectGoSpecNames(spec, source, lang)...)
	}
	return names
}

// findGoSpecForName locates the var_spec/const_spec matching a given variable name.
func findGoSpecForName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *gotreesitter.Node {
	for _, spec := range collectGoSpecs(node, lang) {
		for _, n := range collectGoSpecNames(spec, source, lang) {
			if n != "" {
				return spec
			}
		}
	}
	return nil
}

// extractGoSpecType extracts the type annotation from a var_spec or const_spec node.
func extractGoSpecType(spec *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	typeNode := spec.ChildByFieldName("type", lang)
	if typeNode != nil {
		t := strings.TrimSpace(typeNode.Text(source))
		return &t
	}
	return nil
}

// extractGoVarName extracts the first variable name from a Go declaration node.
func extractGoVarName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	names := collectGoDeclarationNames(node, source, lang)
	if len(names) > 0 {
		return names[0]
	}

	// var_declaration/const_declaration → var_spec/const_spec → identifier
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "var_spec" || child.Type(lang) == "const_spec" {
			name := child.ChildByFieldName("name", lang)
			if name != nil {
				return name.Text(source)
			}
			// Fallback: first identifier child
			for j := 0; j < int(child.NamedChildCount()); j++ {
				gc := child.NamedChild(j)
				if gc != nil && gc.Type(lang) == "identifier" {
					return gc.Text(source)
				}
			}
		}
	}

	// short_var_declaration: x := 5 → expression_list → identifier
	if node.Type(lang) == "short_var_declaration" {
		left := node.ChildByFieldName("left", lang)
		if left != nil && left.Type(lang) == "expression_list" {
			for i := 0; i < int(left.NamedChildCount()); i++ {
				child := left.NamedChild(i)
				if child != nil && child.Type(lang) == "identifier" {
					return child.Text(source)
				}
			}
		}
	}

	return ""
}

// extractGoVarType extracts the type annotation from a Go declaration node.
func extractGoVarType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	for _, spec := range collectGoSpecs(node, lang) {
		typeName := extractGoSpecType(spec, source, lang)
		if typeName != nil {
			return typeName
		}
	}
	return nil
}