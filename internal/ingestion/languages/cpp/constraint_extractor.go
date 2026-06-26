package cpp

// C++ Constraint Extractor — extract C++20 concepts and requires-clauses.
//
// C++20 introduced concepts and requires-clauses as a type-safe replacement
// for SFINAE. This file extracts constraint information from template
// declarations for use in overload resolution.
// Ported from TS languages/cpp/constraint-extractor.ts.

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/odvcencio/gotreesitter"
)

// CppConstraint represents an extracted C++20 concept constraint.
type CppConstraint struct {
	Name         string   // concept name (e.g. "Integral", "Sortable")
	Parameters   []string // concept parameters
	RequiresExpr string   // raw requires-expression text, if any
}

// ExtractCppConstraints extracts concept constraints from a C++ definition.
// TODO: full implementation — parse requires-clauses and concept references.
func ExtractCppConstraints(def shared.SymbolDefinition) []CppConstraint {
	return nil
}

// ExtractCppRequiresClause extracts the requires-clause from a template declaration.
// Returns nil if no requires-clause is present.
// TODO: full implementation
func ExtractCppRequiresClause(def shared.SymbolDefinition) string {
	return ""
}

// ExtractCppTemplateConstraints extracts concept constraints from a template_declaration
// AST node and its associated function_declarator. Returns nil if no constraints found.
// This is called during captures emission to enrich @declaration.template-constraints.
func ExtractCppTemplateConstraints(lang *gotreesitter.Language, templateDecl *gotreesitter.Node, funcDeclarator *gotreesitter.Node, source []byte) []CppConstraint {
	var constraints []CppConstraint

	// Walk the template_parameter_list for concept-constrained parameters
	paramList := templateDecl.ChildByFieldName("parameters", lang)
	if paramList == nil {
		// Try finding the template_parameter_list as a direct child
		for i := 0; i < int(templateDecl.ChildCount()); i++ {
			child := templateDecl.Child(i)
			if child != nil && child.Type(lang) == "template_parameter_list" {
				paramList = child
				break
			}
		}
	}
	if paramList != nil {
		for i := 0; i < int(paramList.NamedChildCount()); i++ {
			param := paramList.NamedChild(i)
			if param == nil {
				continue
			}
			pt := param.Type(lang)
			// type_parameter_declaration may have a concept constraint
			// e.g. template <Integral T> → "Integral" is the concept name
			if pt == "type_parameter_declaration" || pt == "optional_type_parameter_declaration" || pt == "variadic_type_parameter_declaration" {
				constraint := extractConceptFromTypeParam(lang, param, source)
				if constraint != nil {
					constraints = append(constraints, *constraint)
				}
			}
		}
	}

	// Check for a requires-clause on the template declaration or function declarator
	requiresText := extractRequiresClauseText(lang, templateDecl, funcDeclarator, source)
	if requiresText != "" {
		constraints = append(constraints, CppConstraint{
			Name:         "requires",
			RequiresExpr: requiresText,
		})
	}

	return constraints
}

// extractConceptFromTypeParam extracts a concept constraint from a type parameter declaration.
// e.g. template <Integral T> → CppConstraint{Name: "Integral", Parameters: ["T"]}
func extractConceptFromTypeParam(lang *gotreesitter.Language, param *gotreesitter.Node, source []byte) *CppConstraint {
	// Walk children looking for type_constraint or concept-style pattern
	for i := 0; i < int(param.ChildCount()); i++ {
		child := param.Child(i)
		if child == nil {
			continue
		}
		ct := child.Type(lang)
		if ct == "type_constraint" {
			// type_constraint has a name (concept) and possibly parameters
			nameNode := child.ChildByFieldName("name", lang)
			if nameNode != nil {
				conceptName := nameNode.Text(source)
				var params []string
				for j := 0; j < int(child.NamedChildCount()); j++ {
					nc := child.NamedChild(j)
					if nc != nil && nc.Type(lang) == "type_identifier" {
						params = append(params, nc.Text(source))
					}
				}
				return &CppConstraint{
					Name:       conceptName,
					Parameters: params,
				}
			}
		}
	}
	return nil
}

// extractRequiresClauseText extracts the raw text of a requires-clause from the
// template declaration or function declarator.
func extractRequiresClauseText(lang *gotreesitter.Language, templateDecl *gotreesitter.Node, funcDeclarator *gotreesitter.Node, source []byte) string {
	// Check template_declaration children for requires_clause
	text := findRequiresClauseText(lang, templateDecl, source)
	if text != "" {
		return text
	}
	// Check function_declarator children for requires_clause
	if funcDeclarator != nil {
		text = findRequiresClauseText(lang, funcDeclarator, source)
		if text != "" {
			return text
		}
	}
	return ""
}

// findRequiresClauseText searches a node's children for a requires_clause.
func findRequiresClauseText(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) string {
	if node == nil {
		return ""
	}
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.Type(lang) == "requires_clause" {
			return strings.TrimSpace(child.Text(source))
		}
	}
	return ""
}