// Package golang — Go import decomposer: split a Go import declaration
// into CaptureMatch records (one per imported name or alias).
// Go import statements can import multiple names from one path:
//
//	import "pkg"           → single named import
//	import alias "pkg"     → alias import
//	import . "pkg"         → wildcard (dot) import
//	import _ "pkg"         → blank (side-effect) import — dropped in V1
//	import ( ... )         → grouped imports
//
// Ported from TS languages/go/import-decomposer.ts.
package golang

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/mengshi02/codetrip/internal/ingestion/utils"
	"github.com/odvcencio/gotreesitter"
)

// SplitGoImportStatementFromNode decomposes a Go import_declaration or import_spec
// AST node into one CaptureMatch per import_spec, with @import.statement,
// @import.kind, @import.source, @import.name, and optionally @import.alias captures.
// Mirrors TS splitGoImportStatement(node).
func SplitGoImportStatementFromNode(lang *gotreesitter.Language, node *gotreesitter.Node, source []byte) []CaptureMatch {
	if node == nil {
		return nil
	}

	if node.Type(lang) == "import_declaration" {
		var out []CaptureMatch
		for i := 0; i < int(node.NamedChildCount()); i++ {
			child := node.NamedChild(i)
			if child == nil {
				continue
			}
			if child.Type(lang) == "import_spec" {
				out = append(out, SplitGoImportStatementFromNode(lang, child, source)...)
			}
			if child.Type(lang) == "import_spec_list" {
				for j := 0; j < int(child.NamedChildCount()); j++ {
					spec := child.NamedChild(j)
					if spec != nil && spec.Type(lang) == "import_spec" {
						out = append(out, SplitGoImportStatementFromNode(lang, spec, source)...)
					}
				}
			}
		}
		return out
	}

	if node.Type(lang) != "import_spec" {
		return nil
	}

	pathNode := node.ChildByFieldName("path", lang)
	if pathNode == nil {
		return nil
	}

	rawPath := strings.Trim(pathNode.Text(source), "\"")
	rawPath = strings.Trim(rawPath, "`")

	nameNode := node.ChildByFieldName("name", lang)
	var alias string
	if nameNode != nil {
		alias = nameNode.Text(source)
	}

	leaf := rawPath
	if idx := strings.LastIndex(rawPath, "/"); idx >= 0 {
		leaf = rawPath[idx+1:]
	}

	kind := "namespace"
	if alias == "." {
		kind = "dot"
	} else if alias == "_" {
		kind = "blank"
	} else if alias != "" {
		kind = "alias"
	}

	// Blank imports (import _ "pkg") are dropped in V1
	if kind == "blank" {
		return nil
	}

	aliased := alias != "" && alias != "." && alias != "_"
	nameText := leaf
	if aliased {
		nameText = alias
	}

	nameAnchor := pathNode
	if nameNode != nil {
		nameAnchor = nameNode
	}

	cm := CaptureMatch{
		"@import.statement": utils.SyntheticCapture("@import.statement", node, node.Text(source)),
		"@import.kind":      utils.SyntheticCapture("@import.kind", node, kind),
		"@import.source":    utils.SyntheticCapture("@import.source", pathNode, rawPath),
		"@import.name":      utils.SyntheticCapture("@import.name", nameAnchor, nameText),
	}

	if aliased {
		cm["@import.alias"] = utils.SyntheticCapture("@import.alias", nameNode, alias)
	}

	return []CaptureMatch{cm}
}

// InterpretGoImportFromCaptures converts a Go import CaptureMatch into a ParsedImport.
// Mirrors TS interpretGoImport(captures).
func InterpretGoImportFromCaptures(captures CaptureMatch) *shared.ParsedImport {
	kindCap, hasKind := captures["@import.kind"]
	if !hasKind {
		return nil
	}
	sourceCap, hasSource := captures["@import.source"]
	if !hasSource {
		return nil
	}
	nameCap, hasName := captures["@import.name"]
	if !hasName {
		return nil
	}

	kind := kindCap.Text
	source := sourceCap.Text
	name := nameCap.Text

	switch kind {
	case "dot":
		return &shared.ParsedImport{
			Kind:      shared.ParsedImportWildcard,
			TargetRaw: &source,
		}
	case "alias":
		aliasCap, hasAlias := captures["@import.alias"]
		if !hasAlias {
			return nil
		}
		return &shared.ParsedImport{
			Kind:         shared.ParsedImportAlias,
			LocalName:    aliasCap.Text,
			ImportedName: name,
			TargetRaw:    &source,
		}
	case "namespace":
		return &shared.ParsedImport{
			Kind:         shared.ParsedImportNamespace,
			LocalName:    name,
			ImportedName: name,
			TargetRaw:    &source,
		}
	}
	return nil
}