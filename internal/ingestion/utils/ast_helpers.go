package utils

import (
	"fmt"
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
	"github.com/odvcencio/gotreesitter"
)

// ── Rust impl target qualification ────────────────────────────────────────

// QualifyRustImplTargetByModScope qualifies a Rust inherent-impl target
// (impl Inner { ... }) by its enclosing mod_item scope, so a bare same-tail
// target nested under different modules resolves to a DISTINCT path
// (outer.Inner vs other.Inner) — the #1982 follow-up to #1975.
// Walks mod_item ancestors (outermost → innermost) and joins them with the
// normalized raw target via the shared SplitQualifiedName.
// A top-level impl Inner (no enclosing mod) returns the bare target unchanged.
// Keyed purely on tree-sitter node types (no language name), matching the
// inherent-impl branch in FindEnclosingClassInfo; the caller restricts this to
// UNSCOPED targets (type_identifier) so a SCOPED impl a::Inner keeps its full
// raw text (#1975).
func QualifyRustImplTargetByModScope(implNode *gotreesitter.Node, rawTargetText string, lang *gotreesitter.Language) string {
	modSegments := make([]string, 0)
	current := implNode.Parent()
	for current != nil {
		if current.Type(lang) == "mod_item" {
			nameNode := current.ChildByFieldName("name", lang)
			if nameNode == nil {
				// Fallback: find identifier child
				for i := 0; i < current.ChildCount(); i++ {
					c := current.Child(i)
					if c != nil && c.Type(lang) == "identifier" {
						nameNode = c
						break
					}
				}
			}
			if nameNode != nil {
				modSegments = append([]string{nameNode.Type(lang)}, modSegments...)
			}
		}
		current = current.Parent()
	}
	parts := append(modSegments, SplitQualifiedName(rawTargetText)...)
	// Filter empty strings
	filtered := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			filtered = append(filtered, p)
		}
	}
	return strings.Join(filtered, ".")
}

// ── Scope-label predicate ─────────────────────────────────────────────────

// IsQualifiableScopeLabel returns true when the node label is one that
// qualifies via the scope walk (qualifyScopeName) rather than the standard
// extractQualifiedName path. #1991: single-sources the nodeLabel == "Trait"
// checks in parsing-processor / parse-worker.
func IsQualifiableScopeLabel(nodeLabel string) bool {
	return nodeLabel == "Trait"
}

// ── Definition capture keys ───────────────────────────────────────────────

// DefinitionCaptureKeys is the ordered list of definition capture keys for
// tree-sitter query matches. Used to extract the definition node from a
// capture map.
var DefinitionCaptureKeys = []string{
	"definition.function",
	"definition.class",
	"definition.interface",
	"definition.method",
	"definition.struct",
	"definition.enum",
	"definition.namespace",
	"definition.module",
	"definition.trait",
	"definition.impl",
	"definition.type",
	"definition.const",
	"definition.static",
	"definition.variable",
	"definition.typedef",
	"definition.macro",
	"definition.union",
	"definition.property",
	"definition.record",
	"definition.delegate",
	"definition.annotation",
	"definition.constructor",
	"definition.template",
}

// GetDefinitionNodeFromCaptures extracts the definition node from a tree-sitter
// query capture map by checking DefinitionCaptureKeys in priority order.
func GetDefinitionNodeFromCaptures(captureMap map[string]*gotreesitter.Node) *gotreesitter.Node {
	for _, key := range DefinitionCaptureKeys {
		if node, ok := captureMap[key]; ok && node != nil {
			return node
		}
	}
	return nil
}

// ── Concrete typedef duplicate suppression ────────────────────────────────

// QueryMatchLike represents a tree-sitter query match with captures.
type QueryMatchLike struct {
	Captures []QueryCapture
}

// QueryCapture represents a single capture within a query match.
type QueryCapture struct {
	Name string
	Node *gotreesitter.Node
}

func nodeRangeKey(node *gotreesitter.Node) string {
	return fmt.Sprintf("%d:%d:%d:%d",
		node.StartPoint().Row,
		node.StartPoint().Column,
		node.EndPoint().Row,
		node.EndPoint().Column,
	)
}

func isConcreteTypedefCapture(captureMap map[string]*gotreesitter.Node, lang *gotreesitter.Language) bool {
	defNode := GetDefinitionNodeFromCaptures(captureMap)
	if defNode == nil || defNode.Type(lang) != "type_definition" {
		return false
	}
	_, hasStruct := captureMap["definition.struct"]
	_, hasEnum := captureMap["definition.enum"]
	return hasStruct || hasEnum
}

// BuildConcreteTypedefDefinitionRanges collects the source ranges of all
// concrete typedef definitions (type_definition nodes that also match
// definition.struct or definition.enum) across a set of query matches.
func BuildConcreteTypedefDefinitionRanges(matches []QueryMatchLike, lang *gotreesitter.Language) map[string]bool {
	ranges := make(map[string]bool)
	for _, match := range matches {
		captureMap := make(map[string]*gotreesitter.Node)
		for _, capture := range match.Captures {
			captureMap[capture.Name] = capture.Node
		}
		defNode := GetDefinitionNodeFromCaptures(captureMap)
		if defNode != nil && isConcreteTypedefCapture(captureMap, lang) {
			ranges[nodeRangeKey(defNode)] = true
		}
	}
	return ranges
}

// IsSuppressedConcreteTypedefDuplicate returns true when a typedef capture
// duplicates a concrete (struct/enum) definition at the same source range,
// so it should be suppressed to avoid emitting a duplicate node.
func IsSuppressedConcreteTypedefDuplicate(
	captureMap map[string]*gotreesitter.Node,
	concreteTypedefRanges map[string]bool,
	lang *gotreesitter.Language,
) bool {
	defNode := GetDefinitionNodeFromCaptures(captureMap)
	if defNode == nil || defNode.Type(lang) != "type_definition" {
		return false
	}
	_, hasTypedef := captureMap["definition.typedef"]
	return hasTypedef && concreteTypedefRanges[nodeRangeKey(defNode)]
}

// ── Function node types ───────────────────────────────────────────────────

// FunctionNodeTypes represents AST node types for function/method definitions
// across languages. Used by parent-walk in call-processor, parse-worker, and
// type-env to detect enclosing function scope boundaries.
//
// INVARIANT: This set MUST be a superset of every language's
// MethodExtractionConfig.methodNodeTypes. When adding a new node type to a
// MethodExtractor config, add it here too.
var FunctionNodeTypes = map[string]bool{
	// TypeScript/JavaScript
	"function_declaration": true,
	"arrow_function":      true,
	"function_expression":  true,
	"method_definition":    true,
	"generator_function_declaration": true,
	// Python
	"function_definition": true,
	// Common async variants
	"async_function_declaration": true,
	"async_arrow_function":       true,
	// Java
	"method_declaration":             true,
	"constructor_declaration":        true,
	"compact_constructor_declaration": true,
	"annotation_type_element_declaration": true,
	// C/C++
	// "function_definition" already included above
	// Go
	// "method_declaration" already included from Java
	// C#
	"local_function_statement": true,
	// Rust
	"function_item": true,
	"impl_item":     true,
	// PHP
	"anonymous_function": true,
	// Kotlin
	"lambda_literal":       true,
	"secondary_constructor": true,
	// Swift
	"init_declaration":   true,
	"deinit_declaration": true,
	// Ruby
	"method":          true,
	"singleton_method": true,
	// Dart
	"function_signature": true,
	"method_signature":   true,
}

// ── Class container types ─────────────────────────────────────────────────

// ClassContainerTypes represents AST node types that act as class-like
// containers (for HAS_METHOD edge extraction).
//
// INVARIANT: When a language config adds a new node type to typeDeclarationNodes,
// that type must also be added here AND to ContainerTypeToLabel below.
var ClassContainerTypes = map[string]bool{
	"class_declaration":          true,
	"abstract_class_declaration": true,
	"interface_declaration":      true,
	"struct_declaration":         true,
	"record_declaration":         true,
	"class_specifier":            true,
	"struct_specifier":           true,
	"impl_item":                  true,
	"trait_item":                 true,
	"struct_item":                true,
	"enum_item":                  true,
	"class_definition":           true,
	"trait_declaration":          true,
	// PHP
	"enum_declaration":     true,
	"protocol_declaration": true,
	// Dart
	"mixin_declaration":     true,
	"extension_declaration": true,
	// Ruby
	"class":          true,
	"module":         true,
	"singleton_class": true,
	// Kotlin
	"object_declaration": true,
	"companion_object":   true,
	// Go
	"struct_type":    true,
	"interface_type": true,
}

// ContainerTypeToLabel maps container AST node types to their graph label.
var ContainerTypeToLabel = map[string]string{
	"class_declaration":          "Class",
	"abstract_class_declaration": "Class",
	"interface_declaration":      "Interface",
	"struct_declaration":         "Struct",
	"struct_specifier":           "Struct",
	"class_specifier":            "Class",
	"class_definition":           "Class",
	"impl_item":                  "Impl",
	"trait_item":                 "Trait",
	"struct_item":                "Struct",
	"enum_item":                  "Enum",
	"trait_declaration":          "Trait",
	"enum_declaration":           "Enum",
	"record_declaration":         "Record",
	"protocol_declaration":       "Interface",
	"mixin_declaration":          "Mixin",
	"extension_declaration":      "Extension",
	"class":                      "Class",
	// Ruby module declarations map to Trait so they participate in the
	// class-like type registry used by lookupClassByName / inheritance
	// resolution. This lets include/extend/prepend mixin heritage
	// resolve to the providing module.
	"module":           "Trait",
	"singleton_class":  "Class",
	"object_declaration": "Class",
	"companion_object":   "Class",
	"struct_type":       "Struct",
	"interface_type":    "Interface",
}

// ── Tree walking ──────────────────────────────────────────────────────────

// WalkNamedTree performs a pre-order walk over a node and all its named
// descendants, invoking cb on each. Replaces the per-language visit clones
// that every language's capture-synthesis walker re-implemented (#1956).
func WalkNamedTree(node *gotreesitter.Node, cb func(*gotreesitter.Node)) {
	cb(node)
	for i := 0; i < node.NamedChildCount(); i++ {
		child := node.NamedChild(i)
		if child != nil {
			WalkNamedTree(child, cb)
		}
	}
}

// ── Ancestor search ───────────────────────────────────────────────────────

// FindAncestorBeforeBoundary returns the first matching ancestor unless a
// boundary ancestor is reached first.
func FindAncestorBeforeBoundary(
	node *gotreesitter.Node,
	targetTypes map[string]bool,
	boundaryTypes map[string]bool,
	lang *gotreesitter.Language,
) *gotreesitter.Node {
	current := node.Parent()
	for current != nil {
		if boundaryTypes[current.Type(lang)] {
			return nil
		}
		if targetTypes[current.Type(lang)] {
			return current
		}
		current = current.Parent()
	}
	return nil
}

// ── Label from captures ───────────────────────────────────────────────────

// LabelOverrideFunc is the provider hook for reclassifying a capture's label.
// Returns the overridden label, or ("", false) to skip this symbol,
// or (originalLabel, true) to keep the default.
type LabelOverrideFunc func(node *gotreesitter.Node, defaultLabel string) (label string, ok bool)

// LanguageProviderLite contains the subset of LanguageProvider needed by
// GetLabelFromCaptures. This avoids a circular import of the full provider.
type LanguageProviderLite struct {
	// LabelOverride is an optional hook for language-specific label
	// reclassification (e.g. C/C++ duplicate skipping, Kotlin Method promotion).
	LabelOverride LabelOverrideFunc
}

// GetLabelFromCaptures determines the graph node label from a tree-sitter
// capture map. Handles language-specific reclassification via the provider's
// labelOverride hook. Returns ("", false) if the capture should be skipped
// (import, call, C/C++ duplicate, missing name).
func GetLabelFromCaptures(
	captureMap map[string]*gotreesitter.Node,
	provider *LanguageProviderLite,
) (string, bool) {
	if _, ok := captureMap["import"]; ok {
		return "", false
	}
	if _, ok := captureMap["call"]; ok {
		return "", false
	}

	hasDefaultExportHocNameSeed := false
	if defFn, ok := captureMap["definition.function"]; ok && defFn != nil {
		_, hasHoc := captureMap["hoc"]
		_, hasCallee := captureMap["callee"]
		hasDefaultExportHocNameSeed = hasHoc || hasCallee
	}

	_, hasName := captureMap["name"]
	_, hasCtor := captureMap["definition.constructor"]
	if !hasName && !hasCtor && !hasDefaultExportHocNameSeed {
		return "", false
	}

	// Apply labelOverride for function captures
	if defFn, ok := captureMap["definition.function"]; ok && defFn != nil {
		if provider != nil && provider.LabelOverride != nil {
			if label, ok := provider.LabelOverride(defFn, "Function"); ok && label != "Function" {
				return label, true
			}
		}
		return "Function", true
	}

	if _, ok := captureMap["definition.class"]; ok {
		return "Class", true
	}
	if _, ok := captureMap["definition.interface"]; ok {
		return "Interface", true
	}
	if _, ok := captureMap["definition.method"]; ok {
		return "Method", true
	}
	if _, ok := captureMap["definition.struct"]; ok {
		return "Struct", true
	}
	if _, ok := captureMap["definition.enum"]; ok {
		return "Enum", true
	}
	if _, ok := captureMap["definition.namespace"]; ok {
		return "Namespace", true
	}

	if defMod, ok := captureMap["definition.module"]; ok && defMod != nil {
		if provider != nil && provider.LabelOverride != nil {
			if label, ok := provider.LabelOverride(defMod, "Module"); ok && label != "" && label != "Module" {
				return label, true
			}
		}
		return "Module", true
	}

	if _, ok := captureMap["definition.trait"]; ok {
		return "Trait", true
	}
	if _, ok := captureMap["definition.impl"]; ok {
		return "Impl", true
	}
	if _, ok := captureMap["definition.type"]; ok {
		return "TypeAlias", true
	}
	if _, ok := captureMap["definition.const"]; ok {
		return "Const", true
	}
	if _, ok := captureMap["definition.static"]; ok {
		return "Static", true
	}
	if _, ok := captureMap["definition.variable"]; ok {
		return "Variable", true
	}
	if _, ok := captureMap["definition.typedef"]; ok {
		return "Typedef", true
	}
	if _, ok := captureMap["definition.macro"]; ok {
		return "Macro", true
	}
	if _, ok := captureMap["definition.union"]; ok {
		return "Union", true
	}
	if _, ok := captureMap["definition.property"]; ok {
		return "Property", true
	}
	if _, ok := captureMap["definition.record"]; ok {
		return "Record", true
	}
	if _, ok := captureMap["definition.delegate"]; ok {
		return "Delegate", true
	}
	if _, ok := captureMap["definition.annotation"]; ok {
		return "Annotation", true
	}
	if _, ok := captureMap["definition.constructor"]; ok {
		return "Constructor", true
	}
	if _, ok := captureMap["definition.template"]; ok {
		return "Template", true
	}

	return "CodeElement", true
}

// Ensure shared types are referenced (prevents unused import error).
var _ shared.NodeLabel = ""