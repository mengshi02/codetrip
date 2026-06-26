package core

import (
	"strings"
	"unicode"

	"github.com/odvcencio/gotreesitter"
)

// ExportChecker decides whether a node / name is exported in a given language.
// Each language provides a checker; the generic pipeline calls it during
// class-like and call-site extraction to determine the 'exported' flag.
type ExportChecker func(node *gotreesitter.Node, name string, lang *gotreesitter.Language) bool

// TSExportChecker: a node is exported if any ancestor is an export_statement
// or export_default_declaration. Walks the ancestor chain to the root.
func TSExportChecker(node *gotreesitter.Node, name string, lang *gotreesitter.Language) bool {
	for cur := node.Parent(); cur != nil; cur = cur.Parent() {
		kind := cur.Type(lang)
		if kind == "export_statement" || kind == "export_default_declaration" {
			return true
		}
	}
	return false
}

// PythonExportChecker: follows Python's _name convention.
// Names starting with _ are private; __name__ is module-special but not private.
// Only truly private names (single underscore, not dunder) are non-exported.
func PythonExportChecker(node *gotreesitter.Node, name string, lang *gotreesitter.Language) bool {
	if strings.HasPrefix(name, "__") && strings.HasSuffix(name, "__") {
		// Dunder names like __init__ are NOT private — they are module-special.
		return true
	}
	return !strings.HasPrefix(name, "_")
}

// JavaExportChecker: in Java, all class/struct/enum/interface members are
// exported (public) unless explicitly marked private or protected.
// Scans the node's direct children for modifier nodes.
func JavaExportChecker(node *gotreesitter.Node, name string, lang *gotreesitter.Language) bool {
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		kind := child.Type(lang)
		if kind == "private" || kind == "protected" {
			return false
		}
	}
	return true
}

// CSHARP_DECL_TYPES: node kinds that are implicitly public when no visibility modifier exists.
var CSHARP_DECL_TYPES = []string{
	"class_declaration",
	"struct_declaration",
	"interface_declaration",
	"enum_declaration",
	"delegate_declaration",
	"method_declaration",
	"property_declaration",
	"field_declaration",
	"constructor_declaration",
	"event_declaration",
	"namespace_declaration",
}

// CSharpExportChecker: a C# declaration is exported if:
//  1. It has an explicit public/protected/internal/protected internal modifier, OR
//  2. It lacks any visibility modifier AND is one of the CSHARP_DECL_TYPES.
func CSharpExportChecker(node *gotreesitter.Node, name string, lang *gotreesitter.Language) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}

	// Scan parent's children for a visibility_modifier that contains this node's sibling.
	for i := 0; i < parent.ChildCount(); i++ {
		sibling := parent.Child(i)
		if sibling == nil {
			continue
		}
		if sibling.Type(lang) == "visibility_modifier" {
			// Check sub-children for public/protected keywords
			for j := 0; j < sibling.ChildCount(); j++ {
				modChild := sibling.Child(j)
				if modChild == nil {
					continue
				}
				modKind := modChild.Type(lang)
				if modKind == "public" || modKind == "protected" ||
					modKind == "internal" || modKind == "protected internal" {
					return true
				}
			}
			// Has visibility_modifier but none of the public-like ones → not exported
			return false
		}
	}

	// No visibility_modifier found — check if node kind is a declaration type
	nodeKind := node.Type(lang)
	for _, decl := range CSHARP_DECL_TYPES {
		if nodeKind == decl {
			return true
		}
	}
	return false
}

// GoExportChecker: Go exports are determined by naming convention.
// Names starting with an uppercase letter (Unicode category Lu) are exported.
func GoExportChecker(node *gotreesitter.Node, name string, lang *gotreesitter.Language) bool {
	if len(name) == 0 {
		return false
	}
	return unicode.IsUpper(rune(name[0]))
}

// RUST_DECL_TYPES: node kinds for Rust declaration items.
// These default to private when no pub modifier is present.
var RUST_DECL_TYPES = []string{
	"function_item",
	"struct_item",
	"enum_item",
	"trait_item",
	"mod_item",
	"type_item",
	"const_item",
	"static_item",
	"impl_item",
	"macro_definition",
	"macro_invocation",
	"union_item",
	"extern_crate_item",
	"use_declaration",
}

// RustExportChecker: a Rust item is exported (pub) if:
//  1. It has a visibility_modifier sibling containing "pub"
// The default in Rust is private, so only explicit pub makes things exported.
func RustExportChecker(node *gotreesitter.Node, name string, lang *gotreesitter.Language) bool {
	parent := node.Parent()
	if parent == nil {
		return false
	}

	// Scan siblings for a visibility_modifier child containing "pub"
	for i := 0; i < parent.ChildCount(); i++ {
		sibling := parent.Child(i)
		if sibling == nil {
			continue
		}
		if sibling.Type(lang) == "visibility_modifier" {
			// Check if any sub-child is "pub"
			for j := 0; j < sibling.ChildCount(); j++ {
				modChild := sibling.Child(j)
				if modChild != nil && modChild.Type(lang) == "pub" {
					return true
				}
			}
			// Has visibility_modifier but not pub → not exported
			return false
		}
	}

	// No visibility_modifier → not exported (Rust default is private)
	return false
}

// CCppExportChecker: C/C++ symbols are "exported" (externally visible) unless:
//  1. Marked static (file-scope only), OR
//  2. Inside an anonymous namespace (C++ idiom for file-scope linkage).
func CCppExportChecker(node *gotreesitter.Node, name string, lang *gotreesitter.Language) bool {
	// Walk ancestors — if any ancestor is an anonymous namespace, not exported
	for cur := node.Parent(); cur != nil; cur = cur.Parent() {
		if cur.Type(lang) == "anonymous_namespace" {
			return false
		}
	}

	// Scan the node's direct children for "static" modifier
	for i := 0; i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Type(lang) == "static" {
			return false
		}
	}

	// Check siblings in parent for storage_class_specifier containing "static"
	parent := node.Parent()
	if parent != nil {
		for i := 0; i < parent.ChildCount(); i++ {
			sibling := parent.Child(i)
			if sibling == nil {
				continue
			}
			if sibling.Type(lang) == "storage_class_specifier" {
				for j := 0; j < sibling.ChildCount(); j++ {
					scChild := sibling.Child(j)
					if scChild != nil && scChild.Type(lang) == "static" {
						return false
					}
				}
			}
		}
	}

	// No static or anonymous namespace → exported
	return true
}