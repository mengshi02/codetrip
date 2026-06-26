package c

import (
	"strings"

	"github.com/odvcencio/gotreesitter"
)

// CIncludeCapture represents the result of decomposing a C #include node.
type CIncludeCapture struct {
	Kind     string // "wildcard" for C includes
	Source   string // The include path
	IsSystem bool   // Whether this is a system header (<stdio.h>)
}

// SplitCInclude decomposes a preproc_include node into structured import captures.
// C #include maps to a wildcard import (all symbols from the header are visible).
// Only literal include paths are emitted as import sources.
// Computed includes like `#include HEADER_MACRO` are skipped.
// Ported from GitNexus c/import-decomposer.ts.
func SplitCInclude(tsLang *gotreesitter.Language, node *gotreesitter.Node, source []byte) *CIncludeCapture {
	pathNode := node.ChildByFieldName("path", tsLang)
	if pathNode == nil {
		// Fallback: scan children for string_literal or system_lib_string
		for i := 0; i < int(node.ChildCount()); i++ {
			child := node.Child(i)
			if child == nil {
				continue
			}
			ct := child.Type(tsLang)
			if ct == "string_literal" || ct == "system_lib_string" {
				return buildIncludeCapture(tsLang, node, child, source)
			}
		}
		return nil
	}
	return buildIncludeCapture(tsLang, node, pathNode, source)
}

func buildIncludeCapture(tsLang *gotreesitter.Language, node, pathNode *gotreesitter.Node, source []byte) *CIncludeCapture {
	// Skip computed includes (`#include MACRO`) — path is an identifier, not a literal header path.
	pathType := pathNode.Type(tsLang)
	if pathType != "string_literal" && pathType != "system_lib_string" {
		return nil
	}

	var raw string
	isSystem := false

	if pathType == "string_literal" {
		// string_literal has children: `"`, string_content, `"`
		// Use named children to find the string_content node
		for i := 0; i < int(pathNode.NamedChildCount()); i++ {
			child := pathNode.NamedChild(i)
			if child != nil && child.Type(tsLang) == "string_content" {
				raw = child.Text(source)
				break
			}
		}
		if raw == "" {
			// Fallback: strip quotes from the full text
			text := pathNode.Text(source)
			raw = strings.Trim(text, "\"")
		}
	} else {
		// system_lib_string: <stdio.h> → strip angle brackets
		raw = pathNode.Text(source)
		raw = strings.Trim(raw, "<>")
		isSystem = true
	}

	return &CIncludeCapture{
		Kind:     "wildcard",
		Source:   raw,
		IsSystem: isSystem,
	}
}