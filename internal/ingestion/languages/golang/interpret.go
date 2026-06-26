// Package golang — Go import/type-binding interpretation hooks.
// These functions translate raw tree-sitter captures into ParsedImport
// and ParsedTypeBinding records that the scope-resolution pipeline consumes.
// Ported from TS languages/go/interpret.ts.
package golang

import (
	"strings"

	"github.com/mengshi02/codetrip/internal/ingestion/shared"
)

// InterpretGoImport converts a Go import capture into a ParsedImport.
// Handles named imports, alias imports, dot imports (wildcards), and
// blank imports (side-effect).
//
// Mirrors TS interpretGoImport(captures).
func InterpretGoImport(captures shared.CaptureMatch) shared.ParsedImport {
	kindCap, hasKind := captures["@import.kind"]
	if !hasKind {
		return shared.ParsedImport{}
	}
	sourceCap, hasSource := captures["@import.source"]
	if !hasSource {
		return shared.ParsedImport{}
	}
	nameCap, hasName := captures["@import.name"]
	if !hasName {
		return shared.ParsedImport{}
	}

	kind := kindCap.Text
	source := sourceCap.Text
	name := nameCap.Text

	switch kind {
	case "dot":
		return shared.ParsedImport{
			Kind:      shared.ParsedImportWildcard,
			TargetRaw: &source,
		}
	case "alias":
		aliasCap, hasAlias := captures["@import.alias"]
		alias := name
		if hasAlias {
			alias = aliasCap.Text
		}
		return shared.ParsedImport{
			Kind:         shared.ParsedImportAlias,
			LocalName:    alias,
			ImportedName: name,
			TargetRaw:    &source,
		}
	case "namespace":
		return shared.ParsedImport{
			Kind:         shared.ParsedImportNamespace,
			LocalName:    name,
			ImportedName: name,
			TargetRaw:    &source,
		}
	case "blank":
		return shared.ParsedImport{
			Kind:      shared.ParsedImportSideEffect,
			TargetRaw: &source,
		}
	}
	return shared.ParsedImport{}
}

// InterpretGoTypeBinding converts a Go type-binding capture into a
// ParsedTypeBinding. Handles receiver self bindings, return-type
// annotations, variable-type annotations, and assignment-inferred types.
//
// Mirrors TS interpretGoTypeBinding(captures).
func InterpretGoTypeBinding(captures shared.CaptureMatch) shared.ParsedTypeBinding {
	nameCap, hasName := captures["@type-binding.name"]
	if !hasName {
		return shared.ParsedTypeBinding{}
	}
	typeCap, hasType := captures["@type-binding.type"]
	if !hasType {
		return shared.ParsedTypeBinding{}
	}

	boundName := nameCap.Text
	rawTypeName := typeCap.Text

	source := determineTypeBindingSource(captures)

	return shared.ParsedTypeBinding{
		BoundName:   boundName,
		RawTypeName: NormalizeGoTypeName(rawTypeName),
		Source:      source,
	}
}

// determineTypeBindingSource determines the TypeRefSource from the capture keys present.
// Mirrors the TS interpret logic that inspects which @type-binding.* captures exist.
func determineTypeBindingSource(captures shared.CaptureMatch) shared.TypeRefSource {
	if _, has := captures["@type-binding.self"]; has {
		return shared.TypeRefSourceSelf
	}
	if _, has := captures["@type-binding.constructor"]; has {
		return shared.TypeRefSourceConstructorInferred
	}
	if _, has := captures["@type-binding.call-return"]; has {
		return shared.TypeRefSourceAssignmentInferred
	}
	if _, has := captures["@type-binding.assertion"]; has {
		return shared.TypeRefSourceAssignmentInferred
	}
	if _, has := captures["@type-binding.field"]; has {
		return shared.TypeRefSourceAnnotation
	}
	if _, has := captures["@type-binding.return"]; has {
		return shared.TypeRefSourceReturnAnnotation
	}
	if _, has := captures["@type-binding.parameter"]; has {
		return shared.TypeRefSourceParameterAnnotation
	}
	return shared.TypeRefSourceAnnotation
}

// NormalizeGoTypeName strips pointer markers, slice/map/chan wrappers,
// package qualifiers, and generic parameters from a Go type name string,
// producing the simple type name used for binding lookups.
//
// Examples:
//   "*Foo" → "Foo"
//   "pkg.Foo" → "Foo"
//   "[]Foo" → "Foo"
//   "map[string]Foo" → "Foo"
//   "chan Foo" → "Foo"
//   "Foo[Bar]" → "Foo"
//
// Mirrors TS normalizeGoTypeName(raw).
func NormalizeGoTypeName(raw string) string {
	s := strings.TrimSpace(raw)

	// Iteratively strip outer wrappers
	for {
		prev := s

		// Strip leading pointer markers
		s = strings.TrimLeft(s, "*")
		s = strings.TrimSpace(s)

		// Strip package qualifier (take last segment after dot)
		if idx := strings.LastIndex(s, "."); idx >= 0 {
			s = s[idx+1:]
		}

		// Strip generic type arguments: Foo[Bar] → Foo
		if gtIdx := strings.Index(s, "["); gtIdx > 0 {
			// Make sure the brackets are balanced
			depth := 0
			valid := true
			for i := gtIdx; i < len(s); i++ {
				if s[i] == '[' {
					depth++
				} else if s[i] == ']' {
					depth--
				}
				if depth == 0 {
					s = s[:gtIdx]
					valid = true
					break
				}
			}
			if !valid {
				// Unbalanced brackets — strip from [ onwards
				s = s[:gtIdx]
			}
		}

		// Strip [] prefix (slice)
		if strings.HasPrefix(s, "[]") {
			s = s[2:]
			s = strings.TrimSpace(s)
		}

		// Strip map[...] prefix
		if strings.HasPrefix(s, "map[") {
			// Find the closing bracket for the key type, then skip to the value type
			depth := 0
			for i := 4; i < len(s); i++ {
				if s[i] == '[' {
					depth++
				} else if s[i] == ']' {
					depth--
					if depth < 0 {
						s = s[i+1:]
						s = strings.TrimSpace(s)
						break
					}
					if depth == 0 {
						// This ] closes map[key], but there might be more brackets
						// Continue scanning for the value type
						s = s[i+1:]
						s = strings.TrimSpace(s)
						break
					}
				}
			}
		}

		// Strip chan prefix
		if strings.HasPrefix(s, "chan ") || strings.HasPrefix(s, "chan<-") || strings.HasPrefix(s, "<-chan") {
			if strings.HasPrefix(s, "<-chan") {
				s = s[6:]
			} else if strings.HasPrefix(s, "chan<-") {
				s = s[6:]
			} else {
				s = s[5:]
			}
			s = strings.TrimSpace(s)
		}

		// Strip func(...) prefix (function types)
		if strings.HasPrefix(s, "func") {
			return "" // Function types have no simple name
		}

		// Strip interface{}/struct{} (no useful name)
		if s == "interface{}" || s == "struct{}" {
			return ""
		}

		// If nothing changed this iteration, we're done
		if s == prev {
			break
		}
	}

	return s
}