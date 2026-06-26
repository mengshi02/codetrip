package cpp

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
	"github.com/odvcencio/gotreesitter"
)

// CppIncludeCapture represents the structured result of decomposing a
// preproc_include or using_declaration node.
type CppIncludeCapture struct {
	Statement shared.Capture
	Kind      shared.Capture // "wildcard" or "named"
	Source    shared.Capture
	System    shared.Capture // optional, for system headers
	Name      shared.Capture // optional, for named imports
	UsingNS   shared.Capture // optional, for using-namespace
}

// splitCppInclude decomposes a preproc_include node into structured captures.
// C++ #include maps to a wildcard import (all symbols from the header are visible).
// Only literal include paths are emitted; computed includes (#include MACRO) are skipped.
// Ported from GitNexus cpp/import-decomposer.ts.
func splitCppInclude(tsLang *gotreesitter.Language, node *gotreesitter.Node, source []byte) *CppIncludeCapture {
	pathNode := node.ChildByFieldName("path", tsLang)
	if pathNode == nil {
		// Fallback: scan direct children for string_literal/system_lib_string
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			if child.Type(tsLang) == "string_literal" || child.Type(tsLang) == "system_lib_string" {
				return buildCppIncludeCapture(tsLang, node, child, source)
			}
		}
		return nil
	}
	return buildCppIncludeCapture(tsLang, node, pathNode, source)
}

func buildCppIncludeCapture(tsLang *gotreesitter.Language, node, pathNode *gotreesitter.Node, source []byte) *CppIncludeCapture {
	// Skip computed includes (#include MACRO) — the path is an identifier,
	// not a literal header path.
	pathType := pathNode.Type(tsLang)
	if pathType != "string_literal" && pathType != "system_lib_string" {
		return nil
	}

	var raw string
	if pathType == "string_literal" {
		// Find string_content child
		for i := 0; i < int(pathNode.NamedChildCount()); i++ {
			child := pathNode.NamedChild(i)
			if child != nil && child.Type(tsLang) == "string_content" {
				raw = child.Text(source)
				break
			}
		}
		if raw == "" {
			raw = strings.Trim(pathNode.Text(source), "\"")
		}
	} else {
		raw = pathNode.Text(source)
		raw = strings.Trim(raw, "<>")
	}

	isSystem := pathType == "system_lib_string"

	result := &CppIncludeCapture{
		Statement: utils.NodeToCapture("@import.statement", node, source),
		Kind:      utils.SyntheticCapture("@import.kind", node, "wildcard"),
		Source:    utils.SyntheticCapture("@import.source", node, raw),
	}

	if isSystem {
		result.System = utils.SyntheticCapture("@import.system", node, "true")
	}

	return result
}

// splitCppUsingDecl decomposes a using_declaration node into structured captures.
//   - using namespace X; → wildcard import
//   - using X::name;     → named import
//   - Class-scope using Base::member; is suppressed (not a namespace import).
//
// Ported from GitNexus cpp/import-decomposer.ts.
func splitCppUsingDecl(tsLang *gotreesitter.Language, node *gotreesitter.Node, source []byte) *CppIncludeCapture {
	if node.Type(tsLang) != "using_declaration" {
		return nil
	}

	// A class-scope `using Base::member;` changes the derived class's member
	// lookup set; it is not a namespace import. Suppress import decomposition.
	for parent := node.Parent(); parent != nil; parent = parent.Parent() {
		pt := parent.Type(tsLang)
		if pt == "class_specifier" || pt == "struct_specifier" {
			return nil
		}
	}

	// Check for "namespace" keyword among anonymous children
	hasNamespaceKeyword := false
	for i := 0; i < int(node.ChildCount()); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		// Anonymous (un-named) child with text "namespace"
		if child.NamedChildCount() == 0 && child.Text(source) == "namespace" {
			hasNamespaceKeyword = true
			break
		}
	}

	if hasNamespaceKeyword {
		// using namespace <name>;
		var namespaceName string
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child == nil {
				continue
			}
			ct := child.Type(tsLang)
			if ct == "identifier" || ct == "qualified_identifier" {
				namespaceName = child.Text(source)
				break
			}
		}
		if namespaceName == "" {
			return nil
		}

		return &CppIncludeCapture{
			Statement: utils.NodeToCapture("@import.statement", node, source),
			Kind:      utils.SyntheticCapture("@import.kind", node, "wildcard"),
			Source:    utils.SyntheticCapture("@import.source", node, namespaceName),
			UsingNS:   utils.SyntheticCapture("@import.using-namespace", node, "true"),
		}
	}

	// using <qualified_identifier>; (e.g. using std::vector)
	var qualId *gotreesitter.Node
	for i := 0; i < int(node.NamedChildCount()); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Type(tsLang) == "qualified_identifier" {
			qualId = child
			break
		}
	}
	if qualId == nil {
		return nil
	}

	// Extract the imported name (last identifier) and source (namespace part)
	nameNode := qualId.ChildByFieldName("name", tsLang)
	scopeNode := qualId.ChildByFieldName("scope", tsLang)

	var importedName string
	if nameNode != nil {
		importedName = nameNode.Text(source)
	} else {
		parts := strings.Split(qualId.Text(source), "::")
		importedName = parts[len(parts)-1]
	}

	var src string
	if scopeNode != nil {
		src = scopeNode.Text(source)
	} else {
		src = strings.TrimSuffix(qualId.Text(source), "::"+importedName)
	}

	return &CppIncludeCapture{
		Statement: utils.NodeToCapture("@import.statement", node, source),
		Kind:      utils.SyntheticCapture("@import.kind", node, "named"),
		Source:    utils.SyntheticCapture("@import.source", node, src),
		Name:      utils.SyntheticCapture("@import.name", node, importedName),
	}
}