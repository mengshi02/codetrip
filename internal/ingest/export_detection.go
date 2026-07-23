package ingest

// Export detection — determines whether a symbol is exported/public in its language.

import (
	"strings"
	"unicode"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// IsNodeExported checks if a tree-sitter node is exported/public in its language.
func IsNodeExported(node *sitter.Node, name string, language string, source []byte) bool {
	switch language {
	case "javascript", "typescript", "tsx":
		return tsExportChecker(node, name, source)
	case "python":
		return !strings.HasPrefix(name, "_")
	case "java":
		return javaExportChecker(node, name, source)
	case "csharp":
		return csharpExportChecker(node, name, source)
	case "go":
		return goExportChecker(name)
	case "rust":
		return rustExportChecker(node, name, source)
	case "kotlin":
		return kotlinExportChecker(node, name, source)
	case "c", "cpp":
		return cCppExportChecker(node, name, source)
	case "php":
		return phpExportChecker(node, name, source)
	case "swift":
		return swiftExportChecker(node, name, source)
	default:
		return false
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Per-language export checkers
// ─────────────────────────────────────────────────────────────────────────────

// tsExportChecker: JS/TS — walk ancestors looking for export_statement or export_specifier.
func tsExportChecker(node *sitter.Node, _ string, _ []byte) bool {
	current := node
	for current != nil {
		kind := current.Kind()
		if kind == "export_statement" || kind == "export_specifier" {
			return true
		}
		if kind == "lexical_declaration" {
			parent := current.Parent()
			if parent != nil && parent.Kind() == "export_statement" {
				return true
			}
		}
		current = current.Parent()
	}
	return false
}

// javaExportChecker: Java — check for 'public' modifier in sibling modifiers nodes.
func javaExportChecker(node *sitter.Node, _ string, source []byte) bool {
	current := node
	for current != nil {
		// Definitions are passed directly by the parser.  In that case the
		// modifiers node is a child of current (not a sibling of its name).
		// Check the declaration itself before walking to its parents.
		for i := uint(0); i < current.ChildCount(); i++ {
			child := current.Child(i)
			if child != nil && child.Kind() == "modifiers" && strings.Contains(child.Utf8Text(source), "public") {
				return true
			}
			if child != nil && !child.IsNamed() && child.Utf8Text(source) == "public" &&
				(current.Kind() == "method_declaration" || current.Kind() == "constructor_declaration") {
				return true
			}
		}
		if parent := current.Parent(); parent != nil {
			for i := uint(0); i < parent.ChildCount(); i++ {
				child := parent.Child(i)
				if child != nil && child.Kind() == "modifiers" {
					// Check text content for 'public'
					text := child.Utf8Text(source)
					if strings.Contains(text, "public") {
						return true
					}
				}
			}
			// Method/constructor with 'public' at start
			if parent.Kind() == "method_declaration" || parent.Kind() == "constructor_declaration" {
				for i := uint(0); i < parent.ChildCount(); i++ {
					child := parent.Child(i)
					if child != nil && !child.IsNamed() && child.Utf8Text(source) == "public" {
						return true
					}
				}
			}
		}
		current = current.Parent()
	}
	return false
}

// csharpDeclTypes — C# declaration node types for sibling modifier scanning.
var csharpDeclTypes = map[string]bool{
	"method_declaration": true, "local_function_statement": true, "constructor_declaration": true,
	"class_declaration": true, "interface_declaration": true, "struct_declaration": true,
	"enum_declaration": true, "record_declaration": true, "record_struct_declaration": true,
	"record_class_declaration": true, "delegate_declaration": true,
	"property_declaration": true, "field_declaration": true, "event_declaration": true,
	"namespace_declaration": true, "file_scoped_namespace_declaration": true,
}

// csharpExportChecker: C# — modifier nodes are siblings of the name node.
func csharpExportChecker(node *sitter.Node, _ string, source []byte) bool {
	current := node
	for current != nil {
		if csharpDeclTypes[current.Kind()] {
			for i := uint(0); i < current.ChildCount(); i++ {
				child := current.Child(i)
				if child != nil && child.Kind() == "modifier" {
					text := child.Utf8Text(source)
					if text == "public" {
						return true
					}
				}
			}
			return false
		}
		current = current.Parent()
	}
	return false
}

// goExportChecker: Go — uppercase first letter = exported.
func goExportChecker(name string) bool {
	if len(name) == 0 {
		return false
	}
	first := rune(name[0])
	return unicode.IsUpper(first) && unicode.IsLetter(first)
}

// rustDeclTypes — Rust declaration node types for sibling visibility_modifier scanning.
var rustDeclTypes = map[string]bool{
	"function_item": true, "struct_item": true, "enum_item": true, "trait_item": true,
	"impl_item": true, "union_item": true, "type_item": true, "const_item": true,
	"static_item": true, "mod_item": true, "use_declaration": true,
	"associated_type": true, "function_signature_item": true,
}

// rustExportChecker: Rust — visibility_modifier is a sibling of the name node.
func rustExportChecker(node *sitter.Node, _ string, source []byte) bool {
	current := node
	for current != nil {
		if rustDeclTypes[current.Kind()] {
			for i := uint(0); i < current.ChildCount(); i++ {
				child := current.Child(i)
				if child != nil && child.Kind() == "visibility_modifier" {
					text := child.Utf8Text(source)
					if strings.HasPrefix(text, "pub") {
						return true
					}
				}
			}
			return false
		}
		current = current.Parent()
	}
	return false
}

// kotlinExportChecker: Kotlin — default visibility is public.
// visibility_modifier is inside modifiers, a sibling of the name node.
func kotlinExportChecker(node *sitter.Node, _ string, source []byte) bool {
	current := node
	for current != nil {
		if parent := current.Parent(); parent != nil {
			visMod := FindSiblingChild(parent, "modifiers", "visibility_modifier")
			if visMod != nil {
				text := visMod.Utf8Text(source)
				if text == "private" || text == "internal" || text == "protected" {
					return false
				}
				if text == "public" {
					return true
				}
			}
		}
		current = current.Parent()
	}
	// No visibility modifier = public (Kotlin default)
	return true
}

// cCppExportChecker: C/C++ — functions without 'static' have external linkage.
func cCppExportChecker(node *sitter.Node, _ string, source []byte) bool {
	cur := node
	for cur != nil {
		if cur.Kind() == "function_definition" || cur.Kind() == "declaration" {
			for i := uint(0); i < cur.ChildCount(); i++ {
				child := cur.Child(i)
				if child != nil && child.Kind() == "storage_class_specifier" {
					text := child.Utf8Text(source)
					if text == "static" {
						return false
					}
				}
			}
		}
		// C++ anonymous namespace: namespace_definition with no name child = internal linkage
		if cur.Kind() == "namespace_definition" {
			nameNode := cur.ChildByFieldName("name")
			if nameNode == nil {
				return false
			}
		}
		cur = cur.Parent()
	}
	return true // Top-level C/C++ functions default to external linkage
}

// phpExportChecker: PHP — check for visibility modifier or top-level scope.
func phpExportChecker(node *sitter.Node, _ string, source []byte) bool {
	current := node
	for current != nil {
		if current.Kind() == "class_declaration" ||
			current.Kind() == "interface_declaration" ||
			current.Kind() == "trait_declaration" ||
			current.Kind() == "enum_declaration" {
			return true
		}
		if current.Kind() == "visibility_modifier" {
			text := current.Utf8Text(source)
			if text == "public" {
				return true
			}
		}
		current = current.Parent()
	}
	// Top-level functions are globally accessible
	return true
}

// swiftExportChecker: Swift — check for 'public' or 'open' access modifiers.
func swiftExportChecker(node *sitter.Node, _ string, source []byte) bool {
	current := node
	for current != nil {
		kind := current.Kind()
		if kind == "modifiers" || kind == "visibility_modifier" {
			text := current.Utf8Text(source)
			if strings.Contains(text, "public") || strings.Contains(text, "open") {
				return true
			}
		}
		current = current.Parent()
	}
	return false
}
