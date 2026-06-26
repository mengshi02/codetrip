// python.go — Python variable extraction config.
//
// Handles module-level assignments and annotated assignments:
//   - MAX_SIZE = 100          → const by UPPER_CASE convention
//   - name: str = "default"   → annotated assignment with type
//   - _private_var = 42       → protected by convention
//   - __private_var = 42      → private by convention
//
// tree-sitter-python uses expression_statement containing assignment or type nodes.
//
// Ported from TS variable-extractors/configs/python.ts.
package configs

import (
	"regexp"
	"strings"

	"github.com/odvcencio/gotreesitter"

	"github.com/mengshi02/codetrip/internal/ingestion/core"
	typeextractors "github.com/mengshi02/codetrip/internal/ingestion/type_extractors"
)

// upperCaseRe matches UPPER_CASE identifiers (Python constant convention).
var upperCaseRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

// extractPythonName extracts the variable name from a Python expression_statement.
func extractPythonName(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) string {
	inner := node.NamedChild(0)
	if inner == nil {
		return ""
	}

	switch inner.Type(lang) {
	case "type":
		// Annotated assignment without value: name: str
		// AST: expression_statement > type > identifier
		name := inner.ChildByFieldName("name", lang)
		if name == nil {
			name = inner.NamedChild(0)
		}
		if name != nil && name.Type(lang) == "identifier" {
			return name.Text(source)
		}

	case "assignment":
		// Plain assignment: x = 5
		left := inner.ChildByFieldName("left", lang)
		if left != nil && left.Type(lang) == "identifier" {
			return left.Text(source)
		}
	}
	return ""
}

// extractPythonType extracts the type annotation from a Python expression_statement.
func extractPythonType(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) *string {
	inner := node.NamedChild(0)
	if inner == nil {
		return nil
	}

	switch inner.Type(lang) {
	case "type":
		// Standalone annotated type without assignment: name: str
		typeNode := inner.ChildByFieldName("type", lang)
		if typeNode == nil {
			typeNode = inner.NamedChild(1)
		}
		if typeNode != nil {
			if t := typeextractors.ExtractSimpleTypeNameFromNode(typeNode, source, lang, 0); t != nil {
				return t
			}
			trimmed := strings.TrimSpace(typeNode.Text(source))
			return &trimmed
		}

	case "assignment":
		// Annotated assignment: name: str = "hello"
		// AST: expression_statement > assignment > [identifier, type > identifier, ...]
		for i := 0; i < int(inner.ChildCount()); i++ {
			child := inner.Child(i)
			if child != nil && child.Type(lang) == "type" {
				typeId := child.NamedChild(0)
				if typeId != nil {
					if t := typeextractors.ExtractSimpleTypeNameFromNode(typeId, source, lang, 0); t != nil {
						return t
					}
					trimmed := strings.TrimSpace(typeId.Text(source))
					return &trimmed
				}
			}
		}
	}
	return nil
}

// extractPythonVisibility extracts visibility based on Python naming conventions.
func extractPythonVisibility(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) core.VariableVisibility {
	name := extractPythonName(node, source, lang)
	if name == "" {
		return core.VisibilityPublic
	}
	return pythonVisibilityForName(name)
}

// pythonVisibilityForName determines visibility from Python naming conventions.
func pythonVisibilityForName(name string) core.VariableVisibility {
	// Dunder names (__name__, __all__) are public Python conventions
	if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
		return core.VisibilityPublic
	}
	// Double underscore prefix (name mangled) = private
	if strings.HasPrefix(name, "__") {
		return core.VisibilityPrivate
	}
	// Single underscore prefix = protected by convention
	if strings.HasPrefix(name, "_") {
		return core.VisibilityProtected
	}
	return core.VisibilityPublic
}

// isPythonConstByConvention checks if the name follows UPPER_CASE constant convention.
func isPythonConstByConvention(name string) bool {
	if name == "" {
		return false
	}
	return name == strings.ToUpper(name) && upperCaseRe.MatchString(name)
}

// PythonVariableConfig is the variable extraction configuration for Python.
var PythonVariableConfig = core.VariableExtractionConfig{
	Language:          core.LangPython,
	ConstNodeTypes:    []string{},
	StaticNodeTypes:   []string{},
	VariableNodeTypes: []string{"expression_statement"},

	ExtractName:       extractPythonName,
	ExtractType:       extractPythonType,
	ExtractVisibility: extractPythonVisibility,
	ExtractVisibilityForName: func(_ *gotreesitter.Node, name string, _ []byte, _ *gotreesitter.Language) core.VariableVisibility {
		return pythonVisibilityForName(name)
	},

	IsConst: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		name := extractPythonName(node, source, lang)
		return isPythonConstByConvention(name)
	},

	IsStatic: func(_ *gotreesitter.Node, _ []byte, _ *gotreesitter.Language) bool {
		return false
	},

	IsMutable: func(node *gotreesitter.Node, source []byte, lang *gotreesitter.Language) bool {
		name := extractPythonName(node, source, lang)
		if name == "" {
			return true
		}
		return !isPythonConstByConvention(name)
	},
}
