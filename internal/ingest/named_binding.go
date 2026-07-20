package ingest

// Named binding extraction — maps local import names to their exported names.
//
// Core types:
//   - NamedBinding: {local, exported} — e.g. import { User as U } → {local:"U", exported:"User"}
//   - NamedImportBinding: {sourcePath, exportedName} — tracks where a local name resolves
//   - NamedImportMap: Map<FilePath, Map<LocalName, NamedImportBinding>>
//
// Functions:
//   - WalkBindingChain: follow re-export chains through NamedImportMap (max depth 5)
//   - ExtractNamedBindings: language-dispatched named binding extraction from import AST nodes

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// NamedBinding represents a single imported name mapping: local alias → exported name.
type NamedBinding struct {
	Local    string // The name used in the importing file
	Exported string // The original name in the source module
}

// NamedImportBinding tracks the source file and original name for an imported symbol.
type NamedImportBinding struct {
	SourcePath   string
	ExportedName string
}

// NamedImportMap maps importing file paths to their name bindings.
// Map<FilePath, Map<LocalName, NamedImportBinding>>
type NamedImportMap map[string]map[string]NamedImportBinding

// NewNamedImportMap creates an empty NamedImportMap.
func NewNamedImportMap() NamedImportMap {
	return make(NamedImportMap)
}

// ─────────────────────────────────────────────────────────────────────────────
// WalkBindingChain — follow re-export chains through NamedImportMap.
// ─────────────────────────────────────────────────────────────────────────────

// WalkBindingChain walks a named-binding re-export chain through NamedImportMap.
// Returns the definitions found at the end of the chain, or nil if the chain breaks
// (missing binding, circular reference, or depth exceeded). Max depth 5.
func WalkBindingChain(
	name string,
	currentFilePath string,
	symbolTable *SymbolTable,
	namedImportMap NamedImportMap,
	allDefs []*SymbolDefinition,
) []*SymbolDefinition {
	lookupFile := currentFilePath
	lookupName := name
	visited := make(map[string]bool)

	for depth := 0; depth < 5; depth++ {
		bindings, ok := namedImportMap[lookupFile]
		if !ok {
			return nil
		}

		binding, ok := bindings[lookupName]
		if !ok {
			return nil
		}

		key := binding.SourcePath + ":" + binding.ExportedName
		if visited[key] {
			return nil // circular
		}
		visited[key] = true

		targetName := binding.ExportedName
		var resolvedDefs []*SymbolDefinition

		if targetName != lookupName || depth > 0 {
			// Filter allDefs by source path
			filtered := symbolTable.LookupFuzzy(targetName)
			for _, def := range filtered {
				if def.FilePath == binding.SourcePath {
					resolvedDefs = append(resolvedDefs, def)
				}
			}
		} else {
			// Use the pre-computed allDefs, filtered by source path
			for _, def := range allDefs {
				if def.FilePath == binding.SourcePath {
					resolvedDefs = append(resolvedDefs, def)
				}
			}
		}

		if len(resolvedDefs) > 0 {
			return resolvedDefs
		}

		// No definition in source file → follow re-export chain
		lookupFile = binding.SourcePath
		lookupName = targetName
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// ExtractNamedBindings — language-dispatched named binding extraction.
// ─────────────────────────────────────────────────────────────────────────────

// ExtractNamedBindings extracts named bindings from an import AST node.
// Returns nil if the import is not a named import (e.g., import * or default).
func ExtractNamedBindings(importNode *sitter.Node, language string, source []byte) []NamedBinding {
	switch language {
	case "typescript", "tsx", "javascript":
		return extractTsNamedBindings(importNode, source)
	case "python":
		return extractPythonNamedBindings(importNode, source)
	case "kotlin":
		return extractKotlinNamedBindings(importNode, source)
	case "rust":
		return extractRustNamedBindings(importNode, source)
	case "php":
		return extractPhpNamedBindings(importNode, source)
	case "csharp":
		return extractCsharpNamedBindings(importNode, source)
	case "java":
		return extractJavaNamedBindings(importNode, source)
	default:
		return nil
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TypeScript/JavaScript named binding extraction.
// ─────────────────────────────────────────────────────────────────────────────

func extractTsNamedBindings(importNode *sitter.Node, source []byte) []NamedBinding {
	// import_statement > import_clause > named_imports > import_specifier*
	importClause := findChildByKindNamed(importNode, "import_clause")
	if importClause != nil {
		namedImports := findChildByKindNamed(importClause, "named_imports")
		if namedImports == nil {
			return nil // default import, namespace import, or side-effect
		}

		var bindings []NamedBinding
		for i := uint(0); i < namedImports.NamedChildCount(); i++ {
			specifier := namedImports.NamedChild(i)
			if specifier == nil || specifier.Kind() != "import_specifier" {
				continue
			}

			var identifiers []string
			for j := uint(0); j < specifier.NamedChildCount(); j++ {
				child := specifier.NamedChild(j)
				if child != nil && child.Kind() == "identifier" {
					identifiers = append(identifiers, child.Utf8Text(source))
				}
			}

			if len(identifiers) == 1 {
				bindings = append(bindings, NamedBinding{Local: identifiers[0], Exported: identifiers[0]})
			} else if len(identifiers) == 2 {
				// import { Foo as Bar } → exported='Foo', local='Bar'
				bindings = append(bindings, NamedBinding{Local: identifiers[1], Exported: identifiers[0]})
			}
		}
		if len(bindings) > 0 {
			return bindings
		}
		return nil
	}

	// Re-export: export { X } from './y' → export_statement > export_clause > export_specifier
	exportClause := findChildByKindNamed(importNode, "export_clause")
	if exportClause != nil {
		var bindings []NamedBinding
		for i := uint(0); i < exportClause.NamedChildCount(); i++ {
			specifier := exportClause.NamedChild(i)
			if specifier == nil || specifier.Kind() != "export_specifier" {
				continue
			}

			var identifiers []string
			for j := uint(0); j < specifier.NamedChildCount(); j++ {
				child := specifier.NamedChild(j)
				if child != nil && child.Kind() == "identifier" {
					identifiers = append(identifiers, child.Utf8Text(source))
				}
			}

			if len(identifiers) == 1 {
				bindings = append(bindings, NamedBinding{Local: identifiers[0], Exported: identifiers[0]})
			} else if len(identifiers) == 2 {
				// export { Repo as Repository } → exported='Repo', local='Repository'
				bindings = append(bindings, NamedBinding{Local: identifiers[1], Exported: identifiers[0]})
			}
		}
		if len(bindings) > 0 {
			return bindings
		}
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Python named binding extraction.
// ─────────────────────────────────────────────────────────────────────────────

func extractPythonNamedBindings(importNode *sitter.Node, source []byte) []NamedBinding {
	// Only from import_from_statement, not plain import_statement
	if importNode.Kind() != "import_from_statement" {
		return nil
	}

	var bindings []NamedBinding
	// Get the module_name field to skip it
	moduleName := importNode.ChildByFieldName("module_name")
	moduleStart := uint(0)
	if moduleName != nil {
		moduleStart = moduleName.StartByte()
	}

	for i := uint(0); i < importNode.NamedChildCount(); i++ {
		child := importNode.NamedChild(i)
		if child == nil {
			continue
		}

		if child.Kind() == "dotted_name" {
			// Skip the module_name (first dotted_name is the source module)
			if moduleName != nil && child.StartByte() == moduleStart {
				continue
			}
			// This is an imported name: from x import User
			name := child.Utf8Text(source)
			if name != "" {
				bindings = append(bindings, NamedBinding{Local: name, Exported: name})
			}
		}

		if child.Kind() == "aliased_import" {
			// from x import Repo as R
			dottedName := findChildByKindNamed(child, "dotted_name")
			aliasIdent := findChildByKindNamed(child, "identifier")
			if dottedName != nil && aliasIdent != nil {
				bindings = append(bindings, NamedBinding{
					Local:    aliasIdent.Utf8Text(source),
					Exported: dottedName.Utf8Text(source),
				})
			}
		}
	}

	if len(bindings) > 0 {
		return bindings
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Kotlin named binding extraction.
// ─────────────────────────────────────────────────────────────────────────────

func extractKotlinNamedBindings(importNode *sitter.Node, source []byte) []NamedBinding {
	// import_header > identifier + import_alias > simple_identifier
	if importNode.Kind() != "import_header" {
		return nil
	}

	fullIdent := findChildByKindNamed(importNode, "identifier")
	if fullIdent == nil {
		return nil
	}

	fullText := fullIdent.Utf8Text(source)
	exportedName := fullText
	if idx := strings.LastIndex(fullText, "."); idx >= 0 {
		exportedName = fullText[idx+1:]
	}

	importAlias := findChildByKindNamed(importNode, "import_alias")
	if importAlias != nil {
		// Aliased: import com.example.User as U
		aliasIdent := findChildByKindNamed(importAlias, "simple_identifier")
		if aliasIdent == nil {
			return nil
		}
		return []NamedBinding{{Local: aliasIdent.Utf8Text(source), Exported: exportedName}}
	}

	// Non-aliased: import com.example.User → local="User", exported="User"
	// Skip wildcard imports (ending in *)
	if strings.HasSuffix(fullText, ".*") || strings.HasSuffix(fullText, "*") {
		return nil
	}
	// Skip lowercase last segments — those are member/function imports
	if len(exportedName) > 0 && exportedName[0] >= 'a' && exportedName[0] <= 'z' {
		return nil
	}
	return []NamedBinding{{Local: exportedName, Exported: exportedName}}
}

// ─────────────────────────────────────────────────────────────────────────────
// Rust named binding extraction.
// ─────────────────────────────────────────────────────────────────────────────

func extractRustNamedBindings(importNode *sitter.Node, source []byte) []NamedBinding {
	if importNode.Kind() != "use_declaration" {
		return nil
	}

	var bindings []NamedBinding
	collectRustBindings(importNode, source, &bindings)
	if len(bindings) > 0 {
		return bindings
	}
	return nil
}

func collectRustBindings(node *sitter.Node, source []byte, bindings *[]NamedBinding) {
	if node.Kind() == "use_as_clause" {
		// First identifier = exported name, second identifier = local alias
		var idents []string
		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child == nil {
				continue
			}
			if child.Kind() == "identifier" {
				idents = append(idents, child.Utf8Text(source))
			}
			if child.Kind() == "scoped_identifier" {
				nameNode := child.ChildByFieldName("name")
				if nameNode != nil {
					idents = append(idents, nameNode.Utf8Text(source))
				}
			}
		}
		if len(idents) == 2 {
			*bindings = append(*bindings, NamedBinding{Local: idents[1], Exported: idents[0]})
		}
		return
	}

	// Terminal identifier in a use_list: use crate::models::{User, Repo}
	if node.Kind() == "identifier" {
		parent := node.Parent()
		if parent != nil && parent.Kind() == "use_list" {
			*bindings = append(*bindings, NamedBinding{Local: node.Utf8Text(source), Exported: node.Utf8Text(source)})
			return
		}
	}

	// Skip scoped_identifier that serves as path prefix in scoped_use_list
	if node.Kind() == "scoped_identifier" {
		parent := node.Parent()
		if parent != nil && parent.Kind() == "scoped_use_list" {
			return // path prefix — the use_list sibling handles the actual symbols
		}

		// Terminal scoped_identifier: use crate::models::User;
		// Only extract if this is a leaf (no deeper use_list/use_as_clause/scoped_use_list)
		hasDeeper := false
		for i := uint(0); i < node.NamedChildCount(); i++ {
			child := node.NamedChild(i)
			if child != nil && (child.Kind() == "use_list" || child.Kind() == "use_as_clause" || child.Kind() == "scoped_use_list") {
				hasDeeper = true
				break
			}
		}
		if !hasDeeper {
			nameNode := node.ChildByFieldName("name")
			if nameNode != nil {
				*bindings = append(*bindings, NamedBinding{Local: nameNode.Utf8Text(source), Exported: nameNode.Utf8Text(source)})
			}
			return
		}
	}

	// Recurse into children
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil {
			collectRustBindings(child, source, bindings)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// PHP named binding extraction.
// ─────────────────────────────────────────────────────────────────────────────

func extractPhpNamedBindings(importNode *sitter.Node, source []byte) []NamedBinding {
	// namespace_use_declaration > namespace_use_clause* (flat)
	// namespace_use_declaration > namespace_use_group > namespace_use_clause* (grouped)
	if importNode.Kind() != "namespace_use_declaration" {
		return nil
	}

	var bindings []NamedBinding

	// Collect all clauses — from direct children AND from namespace_use_group
	var clauses []*sitter.Node
	for i := uint(0); i < importNode.NamedChildCount(); i++ {
		child := importNode.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Kind() == "namespace_use_clause" {
			clauses = append(clauses, child)
		} else if child.Kind() == "namespace_use_group" {
			for j := uint(0); j < child.NamedChildCount(); j++ {
				groupChild := child.NamedChild(j)
				if groupChild != nil && groupChild.Kind() == "namespace_use_clause" {
					clauses = append(clauses, groupChild)
				}
			}
		}
	}

	for _, clause := range clauses {
		var qualifiedName *sitter.Node
		var names []*sitter.Node
		for j := uint(0); j < clause.NamedChildCount(); j++ {
			child := clause.NamedChild(j)
			if child == nil {
				continue
			}
			if child.Kind() == "qualified_name" {
				qualifiedName = child
			} else if child.Kind() == "name" {
				names = append(names, child)
			}
		}

		if qualifiedName != nil && len(names) > 0 {
			// Flat aliased import: use App\Models\Repo as R;
			fullText := qualifiedName.Utf8Text(source)
			exportedName := fullText
			if idx := strings.LastIndex(fullText, "\\"); idx >= 0 {
				exportedName = fullText[idx+1:]
			}
			bindings = append(bindings, NamedBinding{Local: names[0].Utf8Text(source), Exported: exportedName})
		} else if qualifiedName != nil && len(names) == 0 {
			// Flat non-aliased import: use App\Models\User;
			fullText := qualifiedName.Utf8Text(source)
			lastSegment := fullText
			if idx := strings.LastIndex(fullText, "\\"); idx >= 0 {
				lastSegment = fullText[idx+1:]
			}
			bindings = append(bindings, NamedBinding{Local: lastSegment, Exported: lastSegment})
		} else if qualifiedName == nil && len(names) >= 2 {
			// Grouped aliased import: {Repo as R} — first name = exported, second = alias
			bindings = append(bindings, NamedBinding{Local: names[1].Utf8Text(source), Exported: names[0].Utf8Text(source)})
		} else if qualifiedName == nil && len(names) == 1 {
			// Grouped non-aliased import: {User} in use App\Models\{User, Repo as R}
			bindings = append(bindings, NamedBinding{Local: names[0].Utf8Text(source), Exported: names[0].Utf8Text(source)})
		}
	}

	if len(bindings) > 0 {
		return bindings
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// C# named binding extraction.
// ─────────────────────────────────────────────────────────────────────────────

func extractCsharpNamedBindings(importNode *sitter.Node, source []byte) []NamedBinding {
	// using_directive with identifier (alias) + qualified_name (target)
	if importNode.Kind() != "using_directive" {
		return nil
	}

	var aliasIdent *sitter.Node
	var qualifiedName *sitter.Node
	for i := uint(0); i < importNode.NamedChildCount(); i++ {
		child := importNode.NamedChild(i)
		if child == nil {
			continue
		}
		if child.Kind() == "identifier" && aliasIdent == nil {
			aliasIdent = child
		} else if child.Kind() == "qualified_name" {
			qualifiedName = child
		}
	}

	if aliasIdent == nil || qualifiedName == nil {
		return nil
	}

	fullText := qualifiedName.Utf8Text(source)
	exportedName := fullText
	if idx := strings.LastIndex(fullText, "."); idx >= 0 {
		exportedName = fullText[idx+1:]
	}

	return []NamedBinding{{Local: aliasIdent.Utf8Text(source), Exported: exportedName}}
}

// ─────────────────────────────────────────────────────────────────────────────
// Java named binding extraction.
// ─────────────────────────────────────────────────────────────────────────────

func extractJavaNamedBindings(importNode *sitter.Node, source []byte) []NamedBinding {
	// import_declaration > scoped_identifier "com.example.models.User"
	// Wildcard imports (.*) don't produce named bindings
	if importNode.Kind() != "import_declaration" {
		return nil
	}

	// Check for asterisk (wildcard import) — skip those
	for i := uint(0); i < importNode.ChildCount(); i++ {
		child := importNode.Child(i)
		if child != nil && child.Kind() == "asterisk" {
			return nil
		}
	}

	scopedId := findChildByKindNamed(importNode, "scoped_identifier")
	if scopedId == nil {
		return nil
	}

	fullText := scopedId.Utf8Text(source)
	lastDot := strings.LastIndex(fullText, ".")
	if lastDot == -1 {
		return nil
	}

	className := fullText[lastDot+1:]
	// Skip lowercase names — those are package imports, not class imports
	if len(className) > 0 && className[0] >= 'a' && className[0] <= 'z' {
		return nil
	}

	return []NamedBinding{{Local: className, Exported: className}}
}

// ─────────────────────────────────────────────────────────────────────────────
// Helper: find named child by kind
// ─────────────────────────────────────────────────────────────────────────────

// findChildByKindNamed finds the first NAMED child of the given kind.
func findChildByKindNamed(node *sitter.Node, kind string) *sitter.Node {
	for i := uint(0); i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil && child.Kind() == kind {
			return child
		}
	}
	return nil
}
